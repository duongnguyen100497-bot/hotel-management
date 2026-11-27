package postgres

import (
	"context"
	"encoding/hex"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/media/domain"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/media/persistence/postgres/sqlcgen"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/db"
)

// LinkedMediaRow holds a room_media row joined with its ready media_assets metadata.
type LinkedMediaRow struct {
	OwnerID      uuid.UUID
	MediaAssetID uuid.UUID
	ObjectKey    string
	SortOrder    int
	IsCover      bool
	MimeType     string
	Width        *int
	Height       *int
}

// ExpiredPendingAsset holds identification for a stale pending upload.
type ExpiredPendingAsset struct {
	ID        uuid.UUID
	ObjectKey string
	CreatedAt time.Time
}

// Repository provides PostgreSQL persistence for media_assets and room_media.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository constructs a PostgreSQL media repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) queries(ctx context.Context) *sqlcgen.Queries {
	return sqlcgen.New(db.Querier(ctx, r.pool))
}

// CreatePendingMediaAsset inserts a new pending media_assets row.
func (r *Repository) CreatePendingMediaAsset(ctx context.Context, id uuid.UUID, objectKey string, uploadedBy *uuid.UUID, now time.Time) (domain.MediaAsset, error) {
	row, err := r.queries(ctx).CreatePendingMediaAsset(ctx, sqlcgen.CreatePendingMediaAssetParams{
		ID:         id,
		ObjectKey:  objectKey,
		UploadedBy: uploadedBy,
		CreatedAt:  now,
	})
	if err != nil {
		return domain.MediaAsset{}, db.MapPgError(err)
	}
	return toDomainMediaAsset(row), nil
}

// GetMediaAssetByID fetches a media asset by ID.
func (r *Repository) GetMediaAssetByID(ctx context.Context, id uuid.UUID) (domain.MediaAsset, error) {
	row, err := r.queries(ctx).GetMediaAssetByID(ctx, id)
	if err != nil {
		return domain.MediaAsset{}, db.MapPgError(err)
	}
	return toDomainMediaAsset(row), nil
}

// GetMediaAssetByIDForUpdate locks and fetches a media asset row (`SELECT ... FOR UPDATE`).
func (r *Repository) GetMediaAssetByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.MediaAsset, error) {
	row, err := r.queries(ctx).GetMediaAssetByIDForUpdate(ctx, id)
	if err != nil {
		return domain.MediaAsset{}, db.MapPgError(err)
	}
	return toDomainMediaAsset(row), nil
}

// MarkMediaAssetReady updates a pending media asset to `ready` with server-verified bitmap metadata.
func (r *Repository) MarkMediaAssetReady(ctx context.Context, id uuid.UUID, meta domain.ValidatedImageMetadata, now time.Time) (domain.MediaAsset, error) {
	row, err := r.queries(ctx).MarkMediaAssetReady(ctx, sqlcgen.MarkMediaAssetReadyParams{
		ID:        id,
		Sha256:    meta.SHA256Raw,
		MimeType:  pgtype.Text{String: meta.MimeType, Valid: meta.MimeType != ""},
		ByteSize:  pgtype.Int8{Int64: meta.ByteSize, Valid: meta.ByteSize > 0},
		Width:     pgtype.Int4{Int32: int32(meta.Width), Valid: meta.Width > 0},
		Height:    pgtype.Int4{Int32: int32(meta.Height), Valid: meta.Height > 0},
		UpdatedAt: now,
	})
	if err != nil {
		return domain.MediaAsset{}, db.MapPgError(err)
	}
	return toDomainMediaAsset(row), nil
}

// MarkMediaAssetDeletionPending transitions a media asset to `deletion_pending`.
func (r *Repository) MarkMediaAssetDeletionPending(ctx context.Context, id uuid.UUID, now time.Time) (domain.MediaAsset, error) {
	row, err := r.queries(ctx).MarkMediaAssetDeletionPending(ctx, sqlcgen.MarkMediaAssetDeletionPendingParams{
		ID:        id,
		UpdatedAt: now,
	})
	if err != nil {
		return domain.MediaAsset{}, db.MapPgError(err)
	}
	return toDomainMediaAsset(row), nil
}

// MarkMediaAssetDeleted transitions a media asset to `deleted`.
func (r *Repository) MarkMediaAssetDeleted(ctx context.Context, id uuid.UUID, now time.Time) error {
	if err := r.queries(ctx).MarkMediaAssetDeleted(ctx, sqlcgen.MarkMediaAssetDeletedParams{
		ID:        id,
		UpdatedAt: now,
	}); err != nil {
		return db.MapPgError(err)
	}
	return nil
}

// DeleteAllLinksForAsset removes all room_media links and clears hotel_settings.logo_media_id for the asset.
func (r *Repository) DeleteAllLinksForAsset(ctx context.Context, mediaAssetID uuid.UUID) error {
	q := r.queries(ctx)
	if err := q.DeleteAllRoomMediaForAsset(ctx, mediaAssetID); err != nil {
		return db.MapPgError(err)
	}
	if err := q.ClearHotelLogoForMediaAsset(ctx, &mediaAssetID); err != nil {
		return db.MapPgError(err)
	}
	return nil
}

// ClearRoomCover unsets is_cover on any existing cover link for roomID.
func (r *Repository) ClearRoomCover(ctx context.Context, roomID uuid.UUID) error {
	if err := r.queries(ctx).ClearRoomCover(ctx, &roomID); err != nil {
		return db.MapPgError(err)
	}
	return nil
}

// ClearRoomTypeCover unsets is_cover on any existing cover link for roomTypeID.
func (r *Repository) ClearRoomTypeCover(ctx context.Context, roomTypeID uuid.UUID) error {
	if err := r.queries(ctx).ClearRoomTypeCover(ctx, &roomTypeID); err != nil {
		return db.MapPgError(err)
	}
	return nil
}

// UpsertRoomMediaLink creates or updates a (room_id, media_asset_id) link.
func (r *Repository) UpsertRoomMediaLink(ctx context.Context, linkID, roomID, mediaAssetID uuid.UUID, sortOrder int, isCover bool, now time.Time) (domain.RoomMediaLink, error) {
	row, err := r.queries(ctx).UpsertRoomMediaLink(ctx, sqlcgen.UpsertRoomMediaLinkParams{
		ID:           linkID,
		RoomID:       &roomID,
		MediaAssetID: mediaAssetID,
		SortOrder:    int16(sortOrder),
		IsCover:      isCover,
		CreatedAt:    now,
	})
	if err != nil {
		return domain.RoomMediaLink{}, db.MapPgError(err)
	}
	return toDomainRoomMediaLink(row), nil
}

// UpsertRoomTypeMediaLink creates or updates a (room_type_id, media_asset_id) link.
func (r *Repository) UpsertRoomTypeMediaLink(ctx context.Context, linkID, roomTypeID, mediaAssetID uuid.UUID, sortOrder int, isCover bool, now time.Time) (domain.RoomMediaLink, error) {
	row, err := r.queries(ctx).UpsertRoomTypeMediaLink(ctx, sqlcgen.UpsertRoomTypeMediaLinkParams{
		ID:           linkID,
		RoomTypeID:   &roomTypeID,
		MediaAssetID: mediaAssetID,
		SortOrder:    int16(sortOrder),
		IsCover:      isCover,
		CreatedAt:    now,
	})
	if err != nil {
		return domain.RoomMediaLink{}, db.MapPgError(err)
	}
	return toDomainRoomMediaLink(row), nil
}

// GetRoomMediaLink fetches an existing (room_id, media_asset_id) link.
func (r *Repository) GetRoomMediaLink(ctx context.Context, roomID, mediaAssetID uuid.UUID) (domain.RoomMediaLink, error) {
	row, err := r.queries(ctx).GetRoomMediaLink(ctx, sqlcgen.GetRoomMediaLinkParams{
		RoomID:       &roomID,
		MediaAssetID: mediaAssetID,
	})
	if err != nil {
		return domain.RoomMediaLink{}, db.MapPgError(err)
	}
	return toDomainRoomMediaLink(row), nil
}

// GetRoomTypeMediaLink fetches an existing (room_type_id, media_asset_id) link.
func (r *Repository) GetRoomTypeMediaLink(ctx context.Context, roomTypeID, mediaAssetID uuid.UUID) (domain.RoomMediaLink, error) {
	row, err := r.queries(ctx).GetRoomTypeMediaLink(ctx, sqlcgen.GetRoomTypeMediaLinkParams{
		RoomTypeID:   &roomTypeID,
		MediaAssetID: mediaAssetID,
	})
	if err != nil {
		return domain.RoomMediaLink{}, db.MapPgError(err)
	}
	return toDomainRoomMediaLink(row), nil
}

// ListReadyMediaByRoomTypeIDs returns all ready linked media rows grouped/ordered for the given roomTypeIDs.
func (r *Repository) ListReadyMediaByRoomTypeIDs(ctx context.Context, roomTypeIDs []uuid.UUID) ([]LinkedMediaRow, error) {
	if len(roomTypeIDs) == 0 {
		return nil, nil
	}
	rows, err := r.queries(ctx).ListReadyMediaByRoomTypeIDs(ctx, roomTypeIDs)
	if err != nil {
		return nil, db.MapPgError(err)
	}
	out := make([]LinkedMediaRow, 0, len(rows))
	for _, row := range rows {
		if row.RoomTypeID == nil {
			continue
		}
		var wPtr, hPtr *int
		if row.Width.Valid {
			w := int(row.Width.Int32)
			wPtr = &w
		}
		if row.Height.Valid {
			h := int(row.Height.Int32)
			hPtr = &h
		}
		out = append(out, LinkedMediaRow{
			OwnerID:      *row.RoomTypeID,
			MediaAssetID: row.MediaAssetID,
			ObjectKey:    row.ObjectKey,
			SortOrder:    int(row.SortOrder),
			IsCover:      row.IsCover,
			MimeType:     row.MimeType.String,
			Width:        wPtr,
			Height:       hPtr,
		})
	}
	return out, nil
}

// ListReadyMediaByRoomIDs returns all ready linked media rows grouped/ordered for the given roomIDs.
func (r *Repository) ListReadyMediaByRoomIDs(ctx context.Context, roomIDs []uuid.UUID) ([]LinkedMediaRow, error) {
	if len(roomIDs) == 0 {
		return nil, nil
	}
	rows, err := r.queries(ctx).ListReadyMediaByRoomIDs(ctx, roomIDs)
	if err != nil {
		return nil, db.MapPgError(err)
	}
	out := make([]LinkedMediaRow, 0, len(rows))
	for _, row := range rows {
		if row.RoomID == nil {
			continue
		}
		var wPtr, hPtr *int
		if row.Width.Valid {
			w := int(row.Width.Int32)
			wPtr = &w
		}
		if row.Height.Valid {
			h := int(row.Height.Int32)
			hPtr = &h
		}
		out = append(out, LinkedMediaRow{
			OwnerID:      *row.RoomID,
			MediaAssetID: row.MediaAssetID,
			ObjectKey:    row.ObjectKey,
			SortOrder:    int(row.SortOrder),
			IsCover:      row.IsCover,
			MimeType:     row.MimeType.String,
			Width:        wPtr,
			Height:       hPtr,
		})
	}
	return out, nil
}

// ListExpiredPendingMediaAssets lists up to limit pending media assets created before cutoff.
func (r *Repository) ListExpiredPendingMediaAssets(ctx context.Context, cutoff time.Time, limit int) ([]ExpiredPendingAsset, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.queries(ctx).ListExpiredPendingMediaAssets(ctx, sqlcgen.ListExpiredPendingMediaAssetsParams{
		CreatedAt: cutoff,
		Limit:     int32(limit),
	})
	if err != nil {
		return nil, db.MapPgError(err)
	}
	out := make([]ExpiredPendingAsset, 0, len(rows))
	for _, row := range rows {
		out = append(out, ExpiredPendingAsset{
			ID:        row.ID,
			ObjectKey: row.ObjectKey,
			CreatedAt: row.CreatedAt,
		})
	}
	return out, nil
}

func toDomainMediaAsset(row sqlcgen.MediaAsset) domain.MediaAsset {
	asset := domain.MediaAsset{
		ID:         row.ID,
		ObjectKey:  row.ObjectKey,
		Status:     row.Status,
		UploadedBy: row.UploadedBy,
		CreatedAt:  row.CreatedAt,
		UpdatedAt:  row.UpdatedAt,
	}
	if len(row.Sha256) > 0 {
		asset.SHA256Raw = row.Sha256
		asset.SHA256Hex = hex.EncodeToString(row.Sha256)
	}
	if row.MimeType.Valid {
		asset.MimeType = row.MimeType.String
	}
	if row.ByteSize.Valid {
		asset.ByteSize = row.ByteSize.Int64
	}
	if row.Width.Valid {
		asset.Width = int(row.Width.Int32)
	}
	if row.Height.Valid {
		asset.Height = int(row.Height.Int32)
	}
	return asset
}

func toDomainRoomMediaLink(row sqlcgen.RoomMedium) domain.RoomMediaLink {
	return domain.RoomMediaLink{
		ID:           row.ID,
		RoomID:       row.RoomID,
		RoomTypeID:   row.RoomTypeID,
		MediaAssetID: row.MediaAssetID,
		SortOrder:    int(row.SortOrder),
		IsCover:      row.IsCover,
		CreatedAt:    row.CreatedAt,
	}
}
