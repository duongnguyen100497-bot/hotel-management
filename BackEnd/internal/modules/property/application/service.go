package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	auditapp "github.com/trandinh0506/hotel-management/backend/internal/modules/audit/application"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/property/domain"
	propertypg "github.com/trandinh0506/hotel-management/backend/internal/modules/property/persistence/postgres"
	rbacapp "github.com/trandinh0506/hotel-management/backend/internal/modules/rbac/application"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/auth"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/clock"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/db"
	platformhttp "github.com/trandinh0506/hotel-management/backend/internal/platform/http"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/id"
)

// Exported application-level aliases for cross-module consumers.
type (
	HotelSettings       = domain.HotelSettings
	PublicHotelSettings = domain.PublicHotelSettings
	MediaItem           = domain.MediaItem
	RoomType            = domain.RoomType
	Room                = domain.Room
	ServiceCatalogItem  = domain.ServiceCatalogItem
)

// SettingsReader provides authoritative read access to single-hotel operational settings
// (hold TTLs, free cancellation hours, currency, timezone) for pricing, booking, and reporting modules.
type SettingsReader interface {
	GetSettings(ctx context.Context) (HotelSettings, error)
}

// RoomCatalogReader provides authoritative read access to room types and physical rooms
// for booking, stayops, media, and reporting modules.
type RoomCatalogReader interface {
	GetRoomTypeByID(ctx context.Context, id uuid.UUID) (RoomType, error)
	GetPublicRoomTypeByCode(ctx context.Context, code string) (RoomType, error)
	GetRoomByID(ctx context.Context, id uuid.UUID) (Room, error)
	ListBookableRoomsByType(ctx context.Context, roomTypeID uuid.UUID) ([]Room, error)
}

// MediaResolver is an optional consumer-side port implemented by the media module (T2.5)
// so property can resolve MinIO URLs without creating a circular module dependency.
type MediaResolver interface {
	ResolveMediaURL(ctx context.Context, mediaID uuid.UUID) (url string, ready bool, err error)
	ListMediaByRoomTypeIDs(ctx context.Context, roomTypeIDs []uuid.UUID) (map[uuid.UUID][]MediaItem, error)
	ListMediaByRoomIDs(ctx context.Context, roomIDs []uuid.UUID) (map[uuid.UUID][]MediaItem, error)
}

// Repository defines the persistence contract for the property module.
type Repository interface {
	GetHotelSettings(ctx context.Context) (domain.HotelSettings, error)
	GetHotelSettingsForUpdate(ctx context.Context) (domain.HotelSettings, error)
	UpdateHotelSettings(ctx context.Context, settings domain.HotelSettings, expectedVersion int, now time.Time) (domain.HotelSettings, error)

	CreateRoomType(ctx context.Context, rt domain.RoomType) (domain.RoomType, error)
	GetRoomTypeByID(ctx context.Context, id uuid.UUID) (domain.RoomType, error)
	GetRoomTypeByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.RoomType, error)
	GetPublicRoomTypeByCode(ctx context.Context, code string) (domain.RoomType, error)
	UpdateRoomType(ctx context.Context, rt domain.RoomType, expectedVersion int, now time.Time) (domain.RoomType, error)
	SearchPublicRoomTypes(ctx context.Context, q propertypg.PublicCatalogQuery) ([]domain.RoomType, *string, error)
	ListStaffRoomTypes(ctx context.Context, q propertypg.StaffRoomTypeQuery) ([]domain.RoomType, *string, error)

	CreateRoom(ctx context.Context, rm domain.Room) (domain.Room, error)
	GetRoomByID(ctx context.Context, id uuid.UUID) (domain.Room, error)
	GetRoomByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.Room, error)
	UpdateRoomMetadata(ctx context.Context, rm domain.Room, expectedVersion int, now time.Time) (domain.Room, error)
	UpdateRoomOperationalStatus(ctx context.Context, roomID uuid.UUID, targetStatus string, expectedVersion int, now time.Time) (domain.Room, error)
	ArchiveRoom(ctx context.Context, id uuid.UUID, archivedAt time.Time) (domain.Room, error)
	ListBookableRoomsByType(ctx context.Context, roomTypeID uuid.UUID) ([]domain.Room, error)
	ListStaffRooms(ctx context.Context, q propertypg.StaffRoomQuery) ([]domain.Room, *string, error)

	InsertRoomStateHistory(ctx context.Context, entry domain.RoomStateHistoryEntry) (domain.RoomStateHistoryEntry, error)
	ListRoomStateHistory(ctx context.Context, roomID uuid.UUID, cursor string, limit int) ([]domain.RoomStateHistoryEntry, *string, error)
	CountOperationalRoomsAt(ctx context.Context, snapshotAt time.Time) (totalActive int, maintenance int, operational int, err error)

	CreateServiceCatalogItem(ctx context.Context, item domain.ServiceCatalogItem) (domain.ServiceCatalogItem, error)
	GetServiceCatalogItemByID(ctx context.Context, id uuid.UUID) (domain.ServiceCatalogItem, error)
	GetServiceCatalogItemByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.ServiceCatalogItem, error)
	UpdateServiceCatalogItem(ctx context.Context, item domain.ServiceCatalogItem, expectedVersion int, now time.Time) (domain.ServiceCatalogItem, error)
	DeactivateServiceCatalogItem(ctx context.Context, id uuid.UUID, now time.Time) (domain.ServiceCatalogItem, error)
	ListServiceCatalogItems(ctx context.Context, q propertypg.ServiceCatalogListQuery) ([]domain.ServiceCatalogItem, *string, error)
}

// UpdateSettingsParams defines inputs for PUT /api/v1/hotel/settings.
type UpdateSettingsParams struct {
	Name                     string
	Address                  string
	Phone                    string
	Email                    string
	Timezone                 string
	Currency                 string
	CheckInTime              string
	CheckOutTime             string
	IndividualHoldTTLMinutes int
	GroupHoldTTLMinutes      int
	FreeCancelHours          int
	LogoMediaID              *uuid.UUID
	ExpectedVersion          int
}

// Service implements hotel settings and property catalog/operational workflows.
type Service struct {
	txRunner db.TxRunner
	repo     Repository
	authz    rbacapp.Authorizer
	audit    auditapp.Recorder
	media    MediaResolver
	clk      clock.Clock
	ids      id.Generator
}

// Compile-time interface assertion.
var _ SettingsReader = (*Service)(nil)

// NewService constructs a property application service.
func NewService(
	txRunner db.TxRunner,
	repo Repository,
	authz rbacapp.Authorizer,
	audit auditapp.Recorder,
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
		txRunner: txRunner,
		repo:     repo,
		authz:    authz,
		audit:    audit,
		clk:      clk,
		ids:      ids,
	}
}

// SetMediaResolver wires the media URL resolver once the media module is initialized.
func (s *Service) SetMediaResolver(resolver MediaResolver) {
	s.media = resolver
}

// GetSettings returns the full hotel settings, resolving LogoURL if configured.
func (s *Service) GetSettings(ctx context.Context) (domain.HotelSettings, error) {
	settings, err := s.repo.GetHotelSettings(ctx)
	if err != nil {
		return domain.HotelSettings{}, err
	}
	if settings.LogoMediaID != nil && s.media != nil {
		if url, ready, err := s.media.ResolveMediaURL(ctx, *settings.LogoMediaID); err == nil && ready {
			settings.LogoURL = url
		}
	}
	return settings, nil
}

// GetPublicSettings returns the public hotel settings view.
func (s *Service) GetPublicSettings(ctx context.Context) (domain.PublicHotelSettings, error) {
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return domain.PublicHotelSettings{}, err
	}
	return settings.ToPublic(), nil
}

// UpdateSettings updates the hotel settings using optimistic locking and records before/after audit data.
// Note: updating individual_hold_ttl_minutes or group_hold_ttl_minutes affects ONLY holds created after commit.
func (s *Service) UpdateSettings(ctx context.Context, actor auth.Principal, params UpdateSettingsParams) (domain.HotelSettings, error) {
	if s.authz != nil {
		if err := s.authz.Authorize(ctx, actor, "hotel_settings.update"); err != nil {
			return domain.HotelSettings{}, err
		}
	}

	candidate := domain.HotelSettings{
		Name:                     params.Name,
		Address:                  params.Address,
		Phone:                    params.Phone,
		Email:                    params.Email,
		Timezone:                 params.Timezone,
		Currency:                 params.Currency,
		CheckInTime:              params.CheckInTime,
		CheckOutTime:             params.CheckOutTime,
		IndividualHoldTTLMinutes: params.IndividualHoldTTLMinutes,
		GroupHoldTTLMinutes:      params.GroupHoldTTLMinutes,
		FreeCancelHours:          params.FreeCancelHours,
		LogoMediaID:              params.LogoMediaID,
	}
	if err := candidate.NormalizeAndValidate(params.ExpectedVersion); err != nil {
		return domain.HotelSettings{}, err
	}

	if candidate.LogoMediaID != nil && s.media != nil {
		_, ready, err := s.media.ResolveMediaURL(ctx, *candidate.LogoMediaID)
		if err != nil {
			return domain.HotelSettings{}, err
		}
		if !ready {
			return domain.HotelSettings{}, apperror.Validation("invalid logoMediaId", apperror.FieldError{
				Field:   "logoMediaId",
				Message: "referenced media asset must exist and be ready",
			})
		}
	}

	now := s.clk.Now()
	var updated domain.HotelSettings

	err := s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		before, err := s.repo.GetHotelSettingsForUpdate(txCtx)
		if err != nil {
			return err
		}
		if before.Version != params.ExpectedVersion {
			return apperror.VersionConflict("hotel settings version conflict")
		}

		after, err := s.repo.UpdateHotelSettings(txCtx, candidate, params.ExpectedVersion, now)
		if err != nil {
			return err
		}

		if s.audit != nil {
			var actorID *uuid.UUID
			if actor.UserID != uuid.Nil {
				actorID = &actor.UserID
			}
			if err := s.audit.Record(txCtx, auditapp.AuditEntry{
				ActorUserID:   actorID,
				Action:        "property.hotel_settings_updated",
				ResourceType:  "hotel_settings",
				RequestID:     platformhttp.RequestIDUUIDFromContext(txCtx),
				CorrelationID: platformhttp.CorrelationIDUUIDFromContext(txCtx),
				BeforeData:    settingsAuditMap(before),
				AfterData:     settingsAuditMap(after),
			}); err != nil {
				return err
			}
		}

		updated = after
		return nil
	})
	if err != nil {
		return domain.HotelSettings{}, err
	}

	if updated.LogoMediaID != nil && s.media != nil {
		if url, ready, err := s.media.ResolveMediaURL(ctx, *updated.LogoMediaID); err == nil && ready {
			updated.LogoURL = url
		}
	}
	return updated, nil
}

func settingsAuditMap(s domain.HotelSettings) map[string]any {
	m := map[string]any{
		"name":                        s.Name,
		"address":                     s.Address,
		"phone":                       s.Phone,
		"email":                       s.Email,
		"timezone":                    s.Timezone,
		"currency":                    s.Currency,
		"check_in_time":               s.CheckInTime,
		"check_out_time":              s.CheckOutTime,
		"individual_hold_ttl_minutes": s.IndividualHoldTTLMinutes,
		"group_hold_ttl_minutes":      s.GroupHoldTTLMinutes,
		"free_cancel_hours":           s.FreeCancelHours,
		"version":                     s.Version,
	}
	if s.LogoMediaID != nil {
		m["logo_media_id"] = s.LogoMediaID.String()
	} else {
		m["logo_media_id"] = nil
	}
	return m
}
