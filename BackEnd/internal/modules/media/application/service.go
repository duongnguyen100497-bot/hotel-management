package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	auditapp "github.com/trandinh0506/hotel-management/backend/internal/modules/audit/application"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/media/domain"
	mediapg "github.com/trandinh0506/hotel-management/backend/internal/modules/media/persistence/postgres"
	propertyapp "github.com/trandinh0506/hotel-management/backend/internal/modules/property/application"
	rbacapp "github.com/trandinh0506/hotel-management/backend/internal/modules/rbac/application"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/auth"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/clock"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/db"
	platformhttp "github.com/trandinh0506/hotel-management/backend/internal/platform/http"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/id"
	platformminio "github.com/trandinh0506/hotel-management/backend/internal/platform/minio"
)

const (
	// EventMediaObjectDeleteRequested is the transactional outbox event type used to delete MinIO objects asynchronously.
	EventMediaObjectDeleteRequested = "media.object_delete_requested"

	DefaultUploadMaxBytes = 5 * 1024 * 1024 // 5 MiB
	DefaultPresignPutTTL  = 10 * time.Minute
	DefaultPendingMaxAge  = 1 * time.Hour
)

// Exported application-level type aliases for cross-module callers.
type (
	MediaAsset    = domain.MediaAsset
	RoomMediaLink = domain.RoomMediaLink
	MediaItem     = propertyapp.MediaItem
)

// RoomMediaReader exposes read-only room & room-type image resolution for the property module.
type RoomMediaReader interface {
	ResolveMediaURL(ctx context.Context, mediaID uuid.UUID) (url string, ready bool, err error)
	ListMediaByRoomTypeIDs(ctx context.Context, roomTypeIDs []uuid.UUID) (map[uuid.UUID][]MediaItem, error)
	ListMediaByRoomIDs(ctx context.Context, roomIDs []uuid.UUID) (map[uuid.UUID][]MediaItem, error)
}

// Repository defines the persistence contract for the media module.
type Repository interface {
	CreatePendingMediaAsset(ctx context.Context, id uuid.UUID, objectKey string, uploadedBy *uuid.UUID, now time.Time) (domain.MediaAsset, error)
	GetMediaAssetByID(ctx context.Context, id uuid.UUID) (domain.MediaAsset, error)
	GetMediaAssetByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.MediaAsset, error)
	MarkMediaAssetReady(ctx context.Context, id uuid.UUID, meta domain.ValidatedImageMetadata, now time.Time) (domain.MediaAsset, error)
	MarkMediaAssetDeletionPending(ctx context.Context, id uuid.UUID, now time.Time) (domain.MediaAsset, error)
	MarkMediaAssetDeleted(ctx context.Context, id uuid.UUID, now time.Time) error
	DeleteAllLinksForAsset(ctx context.Context, mediaAssetID uuid.UUID) error

	ClearRoomCover(ctx context.Context, roomID uuid.UUID) error
	ClearRoomTypeCover(ctx context.Context, roomTypeID uuid.UUID) error
	UpsertRoomMediaLink(ctx context.Context, linkID, roomID, mediaAssetID uuid.UUID, sortOrder int, isCover bool, now time.Time) (domain.RoomMediaLink, error)
	UpsertRoomTypeMediaLink(ctx context.Context, linkID, roomTypeID, mediaAssetID uuid.UUID, sortOrder int, isCover bool, now time.Time) (domain.RoomMediaLink, error)
	GetRoomMediaLink(ctx context.Context, roomID, mediaAssetID uuid.UUID) (domain.RoomMediaLink, error)
	GetRoomTypeMediaLink(ctx context.Context, roomTypeID, mediaAssetID uuid.UUID) (domain.RoomMediaLink, error)

	ListReadyMediaByRoomTypeIDs(ctx context.Context, roomTypeIDs []uuid.UUID) ([]mediapg.LinkedMediaRow, error)
	ListReadyMediaByRoomIDs(ctx context.Context, roomIDs []uuid.UUID) ([]mediapg.LinkedMediaRow, error)
	ListExpiredPendingMediaAssets(ctx context.Context, cutoff time.Time, limit int) ([]mediapg.ExpiredPendingAsset, error)
}

// UploadTicket is returned by RequestUpload with a short-lived presigned PUT URL.
// Invariant: object_key and MinIO credentials are never exposed to the client.
type UploadTicket struct {
	MediaID     uuid.UUID `json:"mediaId"`
	UploadURL   string    `json:"uploadUrl"`
	ContentType string    `json:"contentType"`
	MaxBytes    int64     `json:"maxBytes"`
	ExpiresAt   time.Time `json:"expiresAt"`
}

// RequestUploadParams defines inputs for POST /api/v1/media/uploads.
type RequestUploadParams struct {
	FileName    string
	ContentType string
}

// CompleteUploadParams defines inputs for POST /api/v1/media/{id}/complete.
type CompleteUploadParams struct {
	MediaID    uuid.UUID
	RoomID     *uuid.UUID
	RoomTypeID *uuid.UUID
	SortOrder  int
	IsCover    bool
}

// LinkMediaParams defines inputs for POST /api/v1/rooms/{id}/media and POST /api/v1/room-types/{id}/media.
type LinkMediaParams struct {
	RoomID     *uuid.UUID
	RoomTypeID *uuid.UUID
	MediaID    uuid.UUID
	SortOrder  int
	IsCover    bool
}

// UpdateMediaLinkParams defines inputs for PATCH /api/v1/rooms/{id}/media/{mediaId} and PATCH /api/v1/room-types/{id}/media/{mediaId}.
type UpdateMediaLinkParams struct {
	RoomID     *uuid.UUID
	RoomTypeID *uuid.UUID
	MediaID    uuid.UUID
	SortOrder  *int
	IsCover    *bool
}

type objectDeleteOutboxPayload struct {
	MediaID   uuid.UUID `json:"media_id"`
	ObjectKey string    `json:"object_key"`
}

// Service implements media upload, magic-byte inspection, room/room-type linking, and outbox-driven MinIO cleanup.
type Service struct {
	txRunner       db.TxRunner
	repo           Repository
	storage        platformminio.Storage
	rooms          propertyapp.RoomCatalogReader
	authz          rbacapp.Authorizer
	audit          auditapp.Recorder
	outbox         auditapp.OutboxEnqueuer
	clk            clock.Clock
	ids            id.Generator
	logger         *slog.Logger
	uploadMaxBytes int64
	presignPutTTL  time.Duration
	pendingMaxAge  time.Duration
}

var (
	_ RoomMediaReader           = (*Service)(nil)
	_ propertyapp.MediaResolver = (*Service)(nil)
)

// NewService constructs a media application service.
func NewService(
	txRunner db.TxRunner,
	repo Repository,
	storage platformminio.Storage,
	rooms propertyapp.RoomCatalogReader,
	authz rbacapp.Authorizer,
	audit auditapp.Recorder,
	outbox auditapp.OutboxEnqueuer,
	clk clock.Clock,
	ids id.Generator,
) *Service {
	if clk == nil {
		clk = clock.NewSystem()
	}
	if ids == nil {
		ids = id.NewUUIDGenerator()
	}
	return &Service{
		txRunner:       txRunner,
		repo:           repo,
		storage:        storage,
		rooms:          rooms,
		authz:          authz,
		audit:          audit,
		outbox:         outbox,
		clk:            clk,
		ids:            ids,
		uploadMaxBytes: DefaultUploadMaxBytes,
		presignPutTTL:  DefaultPresignPutTTL,
		pendingMaxAge:  DefaultPendingMaxAge,
	}
}

// WithUploadMaxBytes configures the maximum permitted image upload size in bytes.
func (s *Service) WithUploadMaxBytes(maxBytes int64) *Service {
	if maxBytes > 0 {
		s.uploadMaxBytes = maxBytes
	}
	return s
}

// WithPresignPutTTL configures the lifetime of presigned PUT URLs.
func (s *Service) WithPresignPutTTL(ttl time.Duration) *Service {
	if ttl > 0 {
		s.presignPutTTL = ttl
	}
	return s
}

// WithPendingMaxAge configures the maximum age for uncompleted pending uploads before reconciliation cleanup.
func (s *Service) WithPendingMaxAge(maxAge time.Duration) *Service {
	if maxAge > 0 {
		s.pendingMaxAge = maxAge
	}
	return s
}

// WithLogger attaches a structured logger for background cleanup operations.
func (s *Service) WithLogger(logger *slog.Logger) *Service {
	s.logger = logger
	return s
}

// RequestUpload validates the declared upload, generates a random server-controlled object_key (`uploads/<uuid>`),
// creates a `pending` media_assets row, and returns a short-lived presigned PUT URL (`media.manage`).
func (s *Service) RequestUpload(ctx context.Context, actor auth.Principal, params RequestUploadParams) (UploadTicket, error) {
	if s.authz != nil {
		if err := s.authz.Authorize(ctx, actor, "media.manage"); err != nil {
			return UploadTicket{}, err
		}
	}
	if s.storage == nil {
		return UploadTicket{}, apperror.ServiceUnavailable("object storage is unavailable")
	}

	normMime, err := domain.ValidateDeclaredUpload(params.FileName, params.ContentType)
	if err != nil {
		return UploadTicket{}, err
	}

	mediaID := s.ids.NewUUID()
	// Server-generated UUID object key prevents path traversal and key guessing.
	objectKey := fmt.Sprintf("uploads/%s", s.ids.NewUUID().String())
	now := s.clk.Now()

	uploadURL, err := s.storage.GeneratePresignedPut(ctx, objectKey, s.presignPutTTL)
	if err != nil {
		return UploadTicket{}, apperror.ServiceUnavailable("failed to generate presigned upload url")
	}

	var actorID *uuid.UUID
	if actor.UserID != uuid.Nil {
		actorID = &actor.UserID
	}

	err = s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		asset, err := s.repo.CreatePendingMediaAsset(txCtx, mediaID, objectKey, actorID, now)
		if err != nil {
			return err
		}
		if s.audit != nil {
			if err := s.audit.Record(txCtx, auditapp.AuditEntry{
				ActorUserID:   actorID,
				Action:        "media.upload_requested",
				ResourceType:  "media_asset",
				ResourceID:    &asset.ID,
				RequestID:     platformhttp.RequestIDUUIDFromContext(txCtx),
				CorrelationID: platformhttp.CorrelationIDUUIDFromContext(txCtx),
				AfterData: map[string]any{
					"status":                asset.Status,
					"declared_content_type": normMime,
				},
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return UploadTicket{}, err
	}

	return UploadTicket{
		MediaID:     mediaID,
		UploadURL:   uploadURL,
		ContentType: normMime,
		MaxBytes:    s.uploadMaxBytes,
		ExpiresAt:   now.Add(s.presignPutTTL),
	}, nil
}

// CompleteUpload reads the actual uploaded object from MinIO, enforces UPLOAD_MAX_BYTES,
// performs magic-byte detection (`http.DetectContentType` + `domain.InspectAndDecodeImage`) and full raster
// bitmap decoding (JPEG, PNG, WebP), extracts width/height/sha256, marks the asset `ready`,
// and optionally links it to a room or room_type in a single transaction (`media.manage`).
func (s *Service) CompleteUpload(ctx context.Context, actor auth.Principal, params CompleteUploadParams) (domain.MediaAsset, error) {
	if s.authz != nil {
		if err := s.authz.Authorize(ctx, actor, "media.manage"); err != nil {
			return domain.MediaAsset{}, err
		}
	}
	if s.storage == nil {
		return domain.MediaAsset{}, apperror.ServiceUnavailable("object storage is unavailable")
	}
	if params.RoomID != nil && params.RoomTypeID != nil {
		return domain.MediaAsset{}, apperror.Validation("cannot link media to both roomId and roomTypeId simultaneously", apperror.FieldError{
			Field:   "roomId",
			Message: "specify at most one of roomId or roomTypeId",
		})
	}
	if params.SortOrder < 0 || params.SortOrder > 32767 {
		return domain.MediaAsset{}, apperror.Validation("invalid sortOrder", apperror.FieldError{
			Field:   "sortOrder",
			Message: "must be between 0 and 32767",
		})
	}

	if err := s.verifyTargetOwnerExists(ctx, params.RoomID, params.RoomTypeID); err != nil {
		return domain.MediaAsset{}, err
	}

	asset, err := s.repo.GetMediaAssetByID(ctx, params.MediaID)
	if err != nil {
		return domain.MediaAsset{}, err
	}
	if asset.Status == domain.StatusDeleted || asset.Status == domain.StatusDeletionPending {
		return domain.MediaAsset{}, apperror.Conflict("media asset has been deleted")
	}
	if asset.Status != domain.StatusPending && asset.Status != domain.StatusReady {
		return domain.MediaAsset{}, apperror.Conflict("media asset cannot be completed from current state")
	}

	// Read up to uploadMaxBytes + 1 bytes from MinIO so oversized files are caught without OOM.
	rawBytes, err := s.storage.GetObjectBytes(ctx, asset.ObjectKey, s.uploadMaxBytes+1)
	if err != nil {
		if errors.Is(err, platformminio.ErrObjectNotFound) {
			return domain.MediaAsset{}, apperror.Validation("uploaded object not found in storage", apperror.FieldError{
				Field:   "mediaId",
				Message: "file has not been uploaded to the presigned URL yet",
			})
		}
		return domain.MediaAsset{}, apperror.ServiceUnavailable("failed to read uploaded object from storage")
	}

	// Verify both http.DetectContentType and domain magic-byte + full bitmap decode.
	sniffLen := len(rawBytes)
	if sniffLen > 512 {
		sniffLen = 512
	}
	if sniffLen > 0 {
		stdDetected := strings.ToLower(strings.TrimSpace(http.DetectContentType(rawBytes[:sniffLen])))
		if idx := strings.IndexByte(stdDetected, ';'); idx >= 0 {
			stdDetected = strings.TrimSpace(stdDetected[:idx])
		}
		if !domain.IsAllowedImageMime(stdDetected) {
			return domain.MediaAsset{}, apperror.Validation("disallowed or spoofed file content detected via magic bytes", apperror.FieldError{
				Field:   "contentType",
				Message: fmt.Sprintf("http.DetectContentType identified %q; only image/jpeg, image/png, and image/webp are allowed", stdDetected),
			})
		}
	}

	meta, err := domain.InspectAndDecodeImage(rawBytes, s.uploadMaxBytes)
	if err != nil {
		return domain.MediaAsset{}, err
	}

	now := s.clk.Now()
	var actorID *uuid.UUID
	if actor.UserID != uuid.Nil {
		actorID = &actor.UserID
	}

	var completed domain.MediaAsset
	err = s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		locked, err := s.repo.GetMediaAssetByIDForUpdate(txCtx, params.MediaID)
		if err != nil {
			return err
		}
		if locked.Status == domain.StatusDeleted || locked.Status == domain.StatusDeletionPending {
			return apperror.Conflict("media asset has been deleted")
		}

		updated, err := s.repo.MarkMediaAssetReady(txCtx, locked.ID, meta, now)
		if err != nil {
			return err
		}

		if params.RoomID != nil {
			if params.IsCover {
				if err := s.repo.ClearRoomCover(txCtx, *params.RoomID); err != nil {
					return err
				}
			}
			if _, err := s.repo.UpsertRoomMediaLink(txCtx, s.ids.NewUUID(), *params.RoomID, updated.ID, params.SortOrder, params.IsCover, now); err != nil {
				return err
			}
		} else if params.RoomTypeID != nil {
			if params.IsCover {
				if err := s.repo.ClearRoomTypeCover(txCtx, *params.RoomTypeID); err != nil {
					return err
				}
			}
			if _, err := s.repo.UpsertRoomTypeMediaLink(txCtx, s.ids.NewUUID(), *params.RoomTypeID, updated.ID, params.SortOrder, params.IsCover, now); err != nil {
				return err
			}
		}

		if s.audit != nil {
			afterData := map[string]any{
				"status":    updated.Status,
				"mime_type": updated.MimeType,
				"byte_size": updated.ByteSize,
				"width":     updated.Width,
				"height":    updated.Height,
				"sha256":    updated.SHA256Hex,
			}
			if params.RoomID != nil {
				afterData["room_id"] = params.RoomID.String()
				afterData["is_cover"] = params.IsCover
			}
			if params.RoomTypeID != nil {
				afterData["room_type_id"] = params.RoomTypeID.String()
				afterData["is_cover"] = params.IsCover
			}
			if err := s.audit.Record(txCtx, auditapp.AuditEntry{
				ActorUserID:   actorID,
				Action:        "media.upload_completed",
				ResourceType:  "media_asset",
				ResourceID:    &updated.ID,
				RequestID:     platformhttp.RequestIDUUIDFromContext(txCtx),
				CorrelationID: platformhttp.CorrelationIDUUIDFromContext(txCtx),
				BeforeData:    map[string]any{"status": locked.Status},
				AfterData:     afterData,
			}); err != nil {
				return err
			}
		}

		completed = updated
		return nil
	})
	if err != nil {
		return domain.MediaAsset{}, err
	}

	if url, err := s.storage.ObjectURL(ctx, completed.ObjectKey); err == nil {
		completed.URL = url
	}
	return completed, nil
}

// LinkMedia attaches an already `ready` media asset to a physical room or room type (`media.manage`).
func (s *Service) LinkMedia(ctx context.Context, actor auth.Principal, params LinkMediaParams) (domain.RoomMediaLink, error) {
	if s.authz != nil {
		if err := s.authz.Authorize(ctx, actor, "media.manage"); err != nil {
			return domain.RoomMediaLink{}, err
		}
	}
	if (params.RoomID == nil) == (params.RoomTypeID == nil) {
		return domain.RoomMediaLink{}, apperror.Validation("exactly one of roomId or roomTypeId is required", apperror.FieldError{
			Field:   "roomId",
			Message: "specify either roomId or roomTypeId",
		})
	}
	if params.SortOrder < 0 || params.SortOrder > 32767 {
		return domain.RoomMediaLink{}, apperror.Validation("invalid sortOrder", apperror.FieldError{
			Field:   "sortOrder",
			Message: "must be between 0 and 32767",
		})
	}

	if err := s.verifyTargetOwnerExists(ctx, params.RoomID, params.RoomTypeID); err != nil {
		return domain.RoomMediaLink{}, err
	}

	now := s.clk.Now()
	var actorID *uuid.UUID
	if actor.UserID != uuid.Nil {
		actorID = &actor.UserID
	}

	var (
		link      domain.RoomMediaLink
		objectKey string
	)
	err := s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		asset, err := s.repo.GetMediaAssetByID(txCtx, params.MediaID)
		if err != nil {
			return err
		}
		if asset.Status != domain.StatusReady {
			return apperror.Validation("media asset must be in ready state before linking", apperror.FieldError{
				Field:   "mediaId",
				Message: "media asset is not ready",
			})
		}
		objectKey = asset.ObjectKey

		if params.RoomID != nil {
			if params.IsCover {
				if err := s.repo.ClearRoomCover(txCtx, *params.RoomID); err != nil {
					return err
				}
			}
			l, err := s.repo.UpsertRoomMediaLink(txCtx, s.ids.NewUUID(), *params.RoomID, asset.ID, params.SortOrder, params.IsCover, now)
			if err != nil {
				return err
			}
			link = l
		} else {
			if params.IsCover {
				if err := s.repo.ClearRoomTypeCover(txCtx, *params.RoomTypeID); err != nil {
					return err
				}
			}
			l, err := s.repo.UpsertRoomTypeMediaLink(txCtx, s.ids.NewUUID(), *params.RoomTypeID, asset.ID, params.SortOrder, params.IsCover, now)
			if err != nil {
				return err
			}
			link = l
		}

		if s.audit != nil {
			if err := s.audit.Record(txCtx, auditapp.AuditEntry{
				ActorUserID:   actorID,
				Action:        "media.linked",
				ResourceType:  "room_media",
				ResourceID:    &link.ID,
				RequestID:     platformhttp.RequestIDUUIDFromContext(txCtx),
				CorrelationID: platformhttp.CorrelationIDUUIDFromContext(txCtx),
				AfterData:     roomMediaAuditMap(link),
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return domain.RoomMediaLink{}, err
	}

	if s.storage != nil && objectKey != "" {
		if url, err := s.storage.ObjectURL(ctx, objectKey); err == nil {
			link.URL = url
		}
	}
	return link, nil
}

// UpdateMediaLink updates sortOrder and/or isCover on an existing room or room-type media association (`media.manage`).
func (s *Service) UpdateMediaLink(ctx context.Context, actor auth.Principal, params UpdateMediaLinkParams) (domain.RoomMediaLink, error) {
	if s.authz != nil {
		if err := s.authz.Authorize(ctx, actor, "media.manage"); err != nil {
			return domain.RoomMediaLink{}, err
		}
	}
	if (params.RoomID == nil) == (params.RoomTypeID == nil) {
		return domain.RoomMediaLink{}, apperror.Validation("exactly one of roomId or roomTypeId is required", apperror.FieldError{
			Field:   "roomId",
			Message: "specify either roomId or roomTypeId",
		})
	}
	if params.SortOrder != nil && (*params.SortOrder < 0 || *params.SortOrder > 32767) {
		return domain.RoomMediaLink{}, apperror.Validation("invalid sortOrder", apperror.FieldError{
			Field:   "sortOrder",
			Message: "must be between 0 and 32767",
		})
	}

	if err := s.verifyTargetOwnerExists(ctx, params.RoomID, params.RoomTypeID); err != nil {
		return domain.RoomMediaLink{}, err
	}

	now := s.clk.Now()
	var actorID *uuid.UUID
	if actor.UserID != uuid.Nil {
		actorID = &actor.UserID
	}

	var (
		updated   domain.RoomMediaLink
		objectKey string
	)
	err := s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		asset, err := s.repo.GetMediaAssetByID(txCtx, params.MediaID)
		if err != nil {
			return err
		}
		objectKey = asset.ObjectKey

		if params.RoomID != nil {
			before, err := s.repo.GetRoomMediaLink(txCtx, *params.RoomID, params.MediaID)
			if err != nil {
				return err
			}
			nextSort := before.SortOrder
			if params.SortOrder != nil {
				nextSort = *params.SortOrder
			}
			nextCover := before.IsCover
			if params.IsCover != nil {
				nextCover = *params.IsCover
			}
			if nextCover {
				if err := s.repo.ClearRoomCover(txCtx, *params.RoomID); err != nil {
					return err
				}
			}
			after, err := s.repo.UpsertRoomMediaLink(txCtx, before.ID, *params.RoomID, params.MediaID, nextSort, nextCover, now)
			if err != nil {
				return err
			}
			if s.audit != nil {
				if err := s.audit.Record(txCtx, auditapp.AuditEntry{
					ActorUserID:   actorID,
					Action:        "media.link_updated",
					ResourceType:  "room_media",
					ResourceID:    &after.ID,
					RequestID:     platformhttp.RequestIDUUIDFromContext(txCtx),
					CorrelationID: platformhttp.CorrelationIDUUIDFromContext(txCtx),
					BeforeData:    roomMediaAuditMap(before),
					AfterData:     roomMediaAuditMap(after),
				}); err != nil {
					return err
				}
			}
			updated = after
		} else {
			before, err := s.repo.GetRoomTypeMediaLink(txCtx, *params.RoomTypeID, params.MediaID)
			if err != nil {
				return err
			}
			nextSort := before.SortOrder
			if params.SortOrder != nil {
				nextSort = *params.SortOrder
			}
			nextCover := before.IsCover
			if params.IsCover != nil {
				nextCover = *params.IsCover
			}
			if nextCover {
				if err := s.repo.ClearRoomTypeCover(txCtx, *params.RoomTypeID); err != nil {
					return err
				}
			}
			after, err := s.repo.UpsertRoomTypeMediaLink(txCtx, before.ID, *params.RoomTypeID, params.MediaID, nextSort, nextCover, now)
			if err != nil {
				return err
			}
			if s.audit != nil {
				if err := s.audit.Record(txCtx, auditapp.AuditEntry{
					ActorUserID:   actorID,
					Action:        "media.link_updated",
					ResourceType:  "room_media",
					ResourceID:    &after.ID,
					RequestID:     platformhttp.RequestIDUUIDFromContext(txCtx),
					CorrelationID: platformhttp.CorrelationIDUUIDFromContext(txCtx),
					BeforeData:    roomMediaAuditMap(before),
					AfterData:     roomMediaAuditMap(after),
				}); err != nil {
					return err
				}
			}
			updated = after
		}
		return nil
	})
	if err != nil {
		return domain.RoomMediaLink{}, err
	}

	if s.storage != nil && objectKey != "" {
		if url, err := s.storage.ObjectURL(ctx, objectKey); err == nil {
			updated.URL = url
		}
	}
	return updated, nil
}

// DeleteMedia removes all room/room_type/logo links, marks the asset `deletion_pending`,
// and enqueues a `media.object_delete_requested` outbox event in the same transaction (`media.manage`).
func (s *Service) DeleteMedia(ctx context.Context, actor auth.Principal, mediaID uuid.UUID) (domain.MediaAsset, error) {
	if s.authz != nil {
		if err := s.authz.Authorize(ctx, actor, "media.manage"); err != nil {
			return domain.MediaAsset{}, err
		}
	}

	now := s.clk.Now()
	var actorID *uuid.UUID
	if actor.UserID != uuid.Nil {
		actorID = &actor.UserID
	}

	var deleted domain.MediaAsset
	err := s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		before, err := s.repo.GetMediaAssetByIDForUpdate(txCtx, mediaID)
		if err != nil {
			return err
		}
		if before.Status == domain.StatusDeleted || before.Status == domain.StatusDeletionPending {
			deleted = before
			return nil
		}

		if err := s.repo.DeleteAllLinksForAsset(txCtx, before.ID); err != nil {
			return err
		}

		after, err := s.repo.MarkMediaAssetDeletionPending(txCtx, before.ID, now)
		if err != nil {
			return err
		}

		if s.outbox != nil {
			payloadBytes, err := json.Marshal(objectDeleteOutboxPayload{
				MediaID:   after.ID,
				ObjectKey: after.ObjectKey,
			})
			if err != nil {
				return fmt.Errorf("marshal outbox payload: %w", err)
			}
			if err := s.outbox.Enqueue(txCtx, auditapp.OutboxEvent{
				AggregateType: "media_asset",
				AggregateID:   after.ID,
				EventType:     EventMediaObjectDeleteRequested,
				Payload:       payloadBytes,
			}); err != nil {
				return err
			}
		}

		if s.audit != nil {
			if err := s.audit.Record(txCtx, auditapp.AuditEntry{
				ActorUserID:   actorID,
				Action:        "media.delete_requested",
				ResourceType:  "media_asset",
				ResourceID:    &after.ID,
				RequestID:     platformhttp.RequestIDUUIDFromContext(txCtx),
				CorrelationID: platformhttp.CorrelationIDUUIDFromContext(txCtx),
				BeforeData:    map[string]any{"status": before.Status},
				AfterData:     map[string]any{"status": after.Status},
			}); err != nil {
				return err
			}
		}

		deleted = after
		return nil
	})
	return deleted, err
}

// HandleObjectDeleteRequested is the transactional outbox handler for `media.object_delete_requested`.
// It removes the object from MinIO (idempotent if already removed) and marks `media_assets.status = 'deleted'`.
func (s *Service) HandleObjectDeleteRequested(ctx context.Context, evt auditapp.OutboxEvent) error {
	var payload objectDeleteOutboxPayload
	if err := json.Unmarshal(evt.Payload, &payload); err != nil {
		return fmt.Errorf("decode media delete payload: %w", err)
	}
	if payload.MediaID == uuid.Nil {
		payload.MediaID = evt.AggregateID
	}

	if s.storage != nil && payload.ObjectKey != "" {
		if err := s.storage.RemoveObject(ctx, payload.ObjectKey); err != nil {
			return err
		}
	}
	return s.repo.MarkMediaAssetDeleted(ctx, payload.MediaID, s.clk.Now())
}

// ReconcileStalePendingUploads cleans up `pending` media assets older than s.pendingMaxAge (default 1 hour):
// removes any object uploaded to MinIO and marks the DB row `deleted`.
func (s *Service) ReconcileStalePendingUploads(ctx context.Context, limit int) (int, error) {
	cutoff := s.clk.Now().Add(-s.pendingMaxAge)
	staleList, err := s.repo.ListExpiredPendingMediaAssets(ctx, cutoff, limit)
	if err != nil {
		return 0, err
	}

	cleaned := 0
	for _, item := range staleList {
		if ctx.Err() != nil {
			break
		}
		if s.storage != nil && item.ObjectKey != "" {
			if err := s.storage.RemoveObject(ctx, item.ObjectKey); err != nil {
				if s.logger != nil {
					s.logger.Warn("failed to remove stale pending minio object",
						slog.String("media_id", item.ID.String()),
						slog.Any("error", err),
					)
				}
				continue
			}
		}
		if err := s.repo.MarkMediaAssetDeleted(ctx, item.ID, s.clk.Now()); err != nil {
			if s.logger != nil {
				s.logger.Warn("failed to mark stale pending media asset deleted",
					slog.String("media_id", item.ID.String()),
					slog.Any("error", err),
				)
			}
			continue
		}
		cleaned++
	}
	return cleaned, nil
}

// RunCleanupWorker periodically runs ReconcileStalePendingUploads until ctx is cancelled.
func (s *Service) RunCleanupWorker(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := s.ReconcileStalePendingUploads(ctx, 50); err != nil && ctx.Err() == nil && s.logger != nil {
				s.logger.Error("stale media cleanup failed", slog.Any("error", err))
			}
		}
	}
}

// ResolveMediaURL implements propertyapp.MediaResolver for hotel logo and individual media lookups.
func (s *Service) ResolveMediaURL(ctx context.Context, mediaID uuid.UUID) (string, bool, error) {
	asset, err := s.repo.GetMediaAssetByID(ctx, mediaID)
	if err != nil {
		if appErr, ok := apperror.As(err); ok && appErr.Code == apperror.CodeNotFound {
			return "", false, nil
		}
		return "", false, err
	}
	if asset.Status != domain.StatusReady {
		return "", false, nil
	}
	if s.storage == nil {
		return "", true, nil
	}
	url, err := s.storage.ObjectURL(ctx, asset.ObjectKey)
	if err != nil {
		return "", true, err
	}
	return url, true, nil
}

// ListMediaByRoomTypeIDs implements propertyapp.MediaResolver, returning ready media items keyed by room_type_id.
// Note: if an image was linked to a physical room belonging to a room_type, room_type queries also include direct room_type media,
// and we also fall back to physical room media of that room_type when a room_type has no direct room_type media!
func (s *Service) ListMediaByRoomTypeIDs(ctx context.Context, roomTypeIDs []uuid.UUID) (map[uuid.UUID][]MediaItem, error) {
	rows, err := s.repo.ListReadyMediaByRoomTypeIDs(ctx, roomTypeIDs)
	if err != nil {
		return nil, err
	}
	result := s.groupLinkedMediaRows(ctx, rows)

	// If a room type has no direct room_type_id media links, also aggregate ready media linked to its physical rooms
	// so attaching an image via POST /api/v1/rooms/{id}/media surfaces on GET /api/v1/catalog/room-types as well.
	if s.rooms != nil {
		for _, rtID := range roomTypeIDs {
			if len(result[rtID]) > 0 {
				continue
			}
			rooms, err := s.rooms.ListBookableRoomsByType(ctx, rtID)
			if err != nil || len(rooms) == 0 {
				continue
			}
			roomIDs := make([]uuid.UUID, 0, len(rooms))
			for _, rm := range rooms {
				roomIDs = append(roomIDs, rm.ID)
			}
			roomMediaMap, err := s.ListMediaByRoomIDs(ctx, roomIDs)
			if err != nil {
				continue
			}
			var combined []MediaItem
			for _, rmID := range roomIDs {
				combined = append(combined, roomMediaMap[rmID]...)
			}
			if len(combined) > 0 {
				result[rtID] = combined
			}
		}
	}
	return result, nil
}

// ListMediaByRoomIDs implements propertyapp.MediaResolver, returning ready media items keyed by room_id.
func (s *Service) ListMediaByRoomIDs(ctx context.Context, roomIDs []uuid.UUID) (map[uuid.UUID][]MediaItem, error) {
	rows, err := s.repo.ListReadyMediaByRoomIDs(ctx, roomIDs)
	if err != nil {
		return nil, err
	}
	return s.groupLinkedMediaRows(ctx, rows), nil
}

func (s *Service) groupLinkedMediaRows(ctx context.Context, rows []mediapg.LinkedMediaRow) map[uuid.UUID][]MediaItem {
	out := make(map[uuid.UUID][]MediaItem)
	for _, row := range rows {
		var url string
		if s.storage != nil && row.ObjectKey != "" {
			if u, err := s.storage.ObjectURL(ctx, row.ObjectKey); err == nil {
				url = u
			}
		}
		out[row.OwnerID] = append(out[row.OwnerID], MediaItem{
			MediaID:   row.MediaAssetID,
			URL:       url,
			IsCover:   row.IsCover,
			SortOrder: row.SortOrder,
			MimeType:  row.MimeType,
			Width:     row.Width,
			Height:    row.Height,
		})
	}
	return out
}

func (s *Service) verifyTargetOwnerExists(ctx context.Context, roomID, roomTypeID *uuid.UUID) error {
	if s.rooms == nil {
		return nil
	}
	if roomID != nil {
		if _, err := s.rooms.GetRoomByID(ctx, *roomID); err != nil {
			return err
		}
	}
	if roomTypeID != nil {
		if _, err := s.rooms.GetRoomTypeByID(ctx, *roomTypeID); err != nil {
			return err
		}
	}
	return nil
}

func roomMediaAuditMap(link domain.RoomMediaLink) map[string]any {
	m := map[string]any{
		"media_id":   link.MediaAssetID.String(),
		"sort_order": link.SortOrder,
		"is_cover":   link.IsCover,
	}
	if link.RoomID != nil {
		m["room_id"] = link.RoomID.String()
	}
	if link.RoomTypeID != nil {
		m["room_type_id"] = link.RoomTypeID.String()
	}
	return m
}
