package application

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	auditapp "github.com/trandinh0506/hotel-management/backend/internal/modules/audit/application"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/property/domain"
	propertypg "github.com/trandinh0506/hotel-management/backend/internal/modules/property/persistence/postgres"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/auth"
	platformhttp "github.com/trandinh0506/hotel-management/backend/internal/platform/http"
)

var _ RoomCatalogReader = (*Service)(nil)

// PublicCatalogFilter defines search parameters for GET /api/v1/catalog/room-types.
type PublicCatalogFilter struct {
	Capacity *int
	MinPrice *decimal.Decimal
	MaxPrice *decimal.Decimal
	CheckIn  string
	CheckOut string
	Sort     string
	Cursor   string
	Limit    int
}

// StaffRoomTypeFilter defines filter parameters for GET /api/v1/room-types.
type StaffRoomTypeFilter struct {
	IncludeArchived bool
	IsBookable      *bool
	Cursor          string
	Limit           int
}

// StaffRoomFilter defines filter parameters for GET /api/v1/rooms.
type StaffRoomFilter struct {
	RoomTypeID        *uuid.UUID
	OperationalStatus string
	Floor             *int
	IncludeArchived   bool
	Cursor            string
	Limit             int
}

// CreateRoomTypeParams defines inputs for POST /api/v1/room-types.
type CreateRoomTypeParams struct {
	Code            string
	Name            string
	Description     string
	BaseNightlyRate decimal.Decimal
	Currency        string
	Capacity        int
	AreaSqm         *decimal.Decimal
	BedType         string
	Amenities       []string
	IsBookable      *bool
}

// UpdateRoomTypeParams defines inputs for PATCH /api/v1/room-types/{id}.
type UpdateRoomTypeParams struct {
	ID              uuid.UUID
	Name            *string
	Description     *string
	BaseNightlyRate *decimal.Decimal
	Currency        *string
	Capacity        *int
	AreaSqm         *decimal.Decimal
	ClearAreaSqm    bool
	BedType         *string
	Amenities       *[]string
	IsBookable      *bool
	Archived        *bool
	ExpectedVersion int
}

// CreateRoomParams defines inputs for POST /api/v1/rooms.
type CreateRoomParams struct {
	Code        string
	Name        string
	RoomTypeID  uuid.UUID
	Floor       int
	Capacity    *int
	Description string
}

// UpdateRoomParams defines inputs for PATCH /api/v1/rooms/{id}.
// Note: OperationalStatus is intentionally omitted because PATCH /rooms/{id}
// is strictly forbidden from mutating operational_status directly.
type UpdateRoomParams struct {
	ID              uuid.UUID
	Code            *string
	Name            *string
	RoomTypeID      *uuid.UUID
	Floor           *int
	Capacity        *int
	Description     *string
	ExpectedVersion int
}

// SearchPublicCatalog searches bookable, non-archived room types with filters, allowlisted sort, and cursor pagination.
func (s *Service) SearchPublicCatalog(ctx context.Context, filter PublicCatalogFilter) (platformhttp.PageResponse[domain.RoomType], error) {
	if filter.Capacity != nil && *filter.Capacity <= 0 {
		return platformhttp.PageResponse[domain.RoomType]{}, apperror.Validation("invalid capacity filter", apperror.FieldError{
			Field:   "capacity",
			Message: "must be greater than 0",
		})
	}
	if filter.MinPrice != nil && filter.MinPrice.IsNegative() {
		return platformhttp.PageResponse[domain.RoomType]{}, apperror.Validation("invalid minPrice", apperror.FieldError{
			Field:   "minPrice",
			Message: "must be non-negative",
		})
	}
	if filter.MaxPrice != nil && !filter.MaxPrice.IsPositive() {
		return platformhttp.PageResponse[domain.RoomType]{}, apperror.Validation("invalid maxPrice", apperror.FieldError{
			Field:   "maxPrice",
			Message: "must be greater than 0",
		})
	}
	if filter.MinPrice != nil && filter.MaxPrice != nil && filter.MinPrice.GreaterThan(*filter.MaxPrice) {
		return platformhttp.PageResponse[domain.RoomType]{}, apperror.Validation("invalid price range", apperror.FieldError{
			Field:   "maxPrice",
			Message: "must be greater than or equal to minPrice",
		})
	}
	var (
		hasDateFilter bool
		checkInDate   time.Time
		checkOutDate  time.Time
	)
	if filter.CheckIn != "" || filter.CheckOut != "" {
		var err error
		checkInDate, checkOutDate, err = parseOptionalDateRange(filter.CheckIn, filter.CheckOut)
		if err != nil {
			return platformhttp.PageResponse[domain.RoomType]{}, err
		}
		hasDateFilter = true
	}

	sortMode := strings.TrimSpace(filter.Sort)
	if sortMode == "" {
		sortMode = "price_asc"
	}
	switch sortMode {
	case "price_asc", "price_desc", "capacity_asc", "capacity_desc", "created_asc", "created_desc":
	default:
		return platformhttp.PageResponse[domain.RoomType]{}, apperror.Validation("unsupported sort parameter", apperror.FieldError{
			Field:   "sort",
			Message: "allowed values: price_asc, price_desc, capacity_asc, capacity_desc, created_asc, created_desc",
		})
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	items, nextCursor, err := s.repo.SearchPublicRoomTypes(ctx, propertypg.PublicCatalogQuery{
		Capacity: filter.Capacity,
		MinPrice: filter.MinPrice,
		MaxPrice: filter.MaxPrice,
		Sort:     sortMode,
		Cursor:   filter.Cursor,
		Limit:    limit,
	})
	if err != nil {
		return platformhttp.PageResponse[domain.RoomType]{}, err
	}
	if hasDateFilter && s.availability != nil && len(items) > 0 {
		ids := make([]uuid.UUID, 0, len(items))
		for _, item := range items {
			ids = append(ids, item.ID)
		}
		availMap, err := s.availability.CountAvailableRoomsByType(ctx, ids, checkInDate, checkOutDate)
		if err != nil {
			return platformhttp.PageResponse[domain.RoomType]{}, err
		}
		filtered := make([]domain.RoomType, 0, len(items))
		for _, item := range items {
			cnt := availMap[item.ID]
			if cnt > 0 {
				c := cnt
				item.AvailableRoomCount = &c
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	s.enrichRoomTypesMedia(ctx, items)

	var curPtr *string
	if filter.Cursor != "" {
		c := filter.Cursor
		curPtr = &c
	}
	return platformhttp.PageResponse[domain.RoomType]{
		Data: items,
		Page: platformhttp.PageInfo{
			Limit:      limit,
			Cursor:     curPtr,
			NextCursor: nextCursor,
		},
	}, nil
}

// GetPublicRoomTypeByCode fetches a single bookable, non-archived room type by its code.
func (s *Service) GetPublicRoomTypeByCode(ctx context.Context, code string) (domain.RoomType, error) {
	if strings.TrimSpace(code) == "" {
		return domain.RoomType{}, apperror.Validation("room type code is required", apperror.FieldError{
			Field:   "code",
			Message: "code is required",
		})
	}
	rt, err := s.repo.GetPublicRoomTypeByCode(ctx, code)
	if err != nil {
		return domain.RoomType{}, err
	}
	slice := []domain.RoomType{rt}
	s.enrichRoomTypesMedia(ctx, slice)
	return slice[0], nil
}

// GetRoomTypeByID returns a room type by ID (including non-bookable for internal callers).
func (s *Service) GetRoomTypeByID(ctx context.Context, id uuid.UUID) (domain.RoomType, error) {
	rt, err := s.repo.GetRoomTypeByID(ctx, id)
	if err != nil {
		return domain.RoomType{}, err
	}
	slice := []domain.RoomType{rt}
	s.enrichRoomTypesMedia(ctx, slice)
	return slice[0], nil
}

// GetRoomByID returns a physical room by ID.
func (s *Service) GetRoomByID(ctx context.Context, id uuid.UUID) (domain.Room, error) {
	rm, err := s.repo.GetRoomByID(ctx, id)
	if err != nil {
		return domain.Room{}, err
	}
	slice := []domain.Room{rm}
	s.enrichRoomsMedia(ctx, slice)
	return slice[0], nil
}

// ListBookableRoomsByType returns active non-maintenance physical rooms for a room type.
func (s *Service) ListBookableRoomsByType(ctx context.Context, roomTypeID uuid.UUID) ([]domain.Room, error) {
	return s.repo.ListBookableRoomsByType(ctx, roomTypeID)
}

// ListRoomTypes lists room types for staff/admin callers (`property.read`).
func (s *Service) ListRoomTypes(ctx context.Context, actor auth.Principal, filter StaffRoomTypeFilter) (platformhttp.PageResponse[domain.RoomType], error) {
	if s.authz != nil {
		if err := s.authz.Authorize(ctx, actor, "property.read"); err != nil {
			return platformhttp.PageResponse[domain.RoomType]{}, err
		}
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	items, nextCursor, err := s.repo.ListStaffRoomTypes(ctx, propertypg.StaffRoomTypeQuery{
		IncludeArchived: filter.IncludeArchived,
		IsBookable:      filter.IsBookable,
		Cursor:          filter.Cursor,
		Limit:           limit,
	})
	if err != nil {
		return platformhttp.PageResponse[domain.RoomType]{}, err
	}
	s.enrichRoomTypesMedia(ctx, items)

	var curPtr *string
	if filter.Cursor != "" {
		c := filter.Cursor
		curPtr = &c
	}
	return platformhttp.PageResponse[domain.RoomType]{
		Data: items,
		Page: platformhttp.PageInfo{
			Limit:      limit,
			Cursor:     curPtr,
			NextCursor: nextCursor,
		},
	}, nil
}

// CreateRoomType creates a new room type (`room_type.manage`).
func (s *Service) CreateRoomType(ctx context.Context, actor auth.Principal, params CreateRoomTypeParams) (domain.RoomType, error) {
	if s.authz != nil {
		if err := s.authz.Authorize(ctx, actor, "room_type.manage"); err != nil {
			return domain.RoomType{}, err
		}
	}

	isBookable := true
	if params.IsBookable != nil {
		isBookable = *params.IsBookable
	}
	currency := strings.TrimSpace(params.Currency)
	if currency == "" {
		if cfg, err := s.repo.GetHotelSettings(ctx); err == nil && cfg.Currency != "" {
			currency = cfg.Currency
		} else {
			currency = "VND"
		}
	}

	now := s.clk.Now()
	candidate := domain.RoomType{
		ID:              s.ids.NewUUID(),
		Code:            params.Code,
		Name:            params.Name,
		Description:     params.Description,
		BaseNightlyRate: params.BaseNightlyRate,
		Currency:        currency,
		Capacity:        params.Capacity,
		AreaSqm:         params.AreaSqm,
		BedType:         params.BedType,
		Amenities:       params.Amenities,
		IsBookable:      isBookable,
		CreatedAt:       now,
	}
	if err := candidate.NormalizeAndValidateRoomType(false, 0); err != nil {
		return domain.RoomType{}, err
	}

	var created domain.RoomType
	err := s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		rt, err := s.repo.CreateRoomType(txCtx, candidate)
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
				Action:        "property.room_type_created",
				ResourceType:  "room_type",
				ResourceID:    &rt.ID,
				RequestID:     platformhttp.RequestIDUUIDFromContext(txCtx),
				CorrelationID: platformhttp.CorrelationIDUUIDFromContext(txCtx),
				AfterData:     roomTypeAuditMap(rt),
			}); err != nil {
				return err
			}
		}
		created = rt
		return nil
	})
	return created, err
}

// UpdateRoomType updates an existing room type with optimistic locking (`room_type.manage`).
// Note: updating BaseNightlyRate never mutates historical booking/invoice snapshots.
func (s *Service) UpdateRoomType(ctx context.Context, actor auth.Principal, params UpdateRoomTypeParams) (domain.RoomType, error) {
	if s.authz != nil {
		if err := s.authz.Authorize(ctx, actor, "room_type.manage"); err != nil {
			return domain.RoomType{}, err
		}
	}
	if params.ExpectedVersion < 1 {
		return domain.RoomType{}, apperror.Validation("invalid version", apperror.FieldError{
			Field:   "version",
			Message: "version must be a positive integer",
		})
	}

	now := s.clk.Now()
	var updated domain.RoomType

	err := s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		before, err := s.repo.GetRoomTypeByIDForUpdate(txCtx, params.ID)
		if err != nil {
			return err
		}
		if before.Version != params.ExpectedVersion {
			return apperror.VersionConflict("room type version conflict")
		}

		candidate := before
		if params.Name != nil {
			candidate.Name = *params.Name
		}
		if params.Description != nil {
			candidate.Description = *params.Description
		}
		if params.BaseNightlyRate != nil {
			candidate.BaseNightlyRate = *params.BaseNightlyRate
		}
		if params.Currency != nil {
			candidate.Currency = *params.Currency
		}
		if params.Capacity != nil {
			candidate.Capacity = *params.Capacity
		}
		if params.ClearAreaSqm {
			candidate.AreaSqm = nil
		} else if params.AreaSqm != nil {
			candidate.AreaSqm = params.AreaSqm
		}
		if params.BedType != nil {
			candidate.BedType = *params.BedType
		}
		if params.Amenities != nil {
			candidate.Amenities = *params.Amenities
		}
		if params.IsBookable != nil {
			candidate.IsBookable = *params.IsBookable
		}
		if params.Archived != nil {
			if *params.Archived {
				if candidate.ArchivedAt == nil {
					ts := now
					candidate.ArchivedAt = &ts
				}
				candidate.IsBookable = false
			} else {
				candidate.ArchivedAt = nil
			}
		}

		if err := candidate.NormalizeAndValidateRoomType(true, params.ExpectedVersion); err != nil {
			return err
		}

		after, err := s.repo.UpdateRoomType(txCtx, candidate, params.ExpectedVersion, now)
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
				Action:        "property.room_type_updated",
				ResourceType:  "room_type",
				ResourceID:    &after.ID,
				RequestID:     platformhttp.RequestIDUUIDFromContext(txCtx),
				CorrelationID: platformhttp.CorrelationIDUUIDFromContext(txCtx),
				BeforeData:    roomTypeAuditMap(before),
				AfterData:     roomTypeAuditMap(after),
			}); err != nil {
				return err
			}
		}
		updated = after
		return nil
	})
	if err != nil {
		return domain.RoomType{}, err
	}
	slice := []domain.RoomType{updated}
	s.enrichRoomTypesMedia(ctx, slice)
	return slice[0], nil
}

// ListRooms lists physical rooms for staff/admin callers (`property.read`).
func (s *Service) ListRooms(ctx context.Context, actor auth.Principal, filter StaffRoomFilter) (platformhttp.PageResponse[domain.Room], error) {
	if s.authz != nil {
		if err := s.authz.Authorize(ctx, actor, "property.read"); err != nil {
			return platformhttp.PageResponse[domain.Room]{}, err
		}
	}
	status := strings.TrimSpace(filter.OperationalStatus)
	if status != "" && !domain.IsValidRoomStatus(status) {
		return platformhttp.PageResponse[domain.Room]{}, apperror.Validation("invalid operationalStatus filter", apperror.FieldError{
			Field:   "operationalStatus",
			Message: "allowed values: available, occupied, cleaning, maintenance",
		})
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	items, nextCursor, err := s.repo.ListStaffRooms(ctx, propertypg.StaffRoomQuery{
		RoomTypeID:        filter.RoomTypeID,
		OperationalStatus: status,
		Floor:             filter.Floor,
		IncludeArchived:   filter.IncludeArchived,
		Cursor:            filter.Cursor,
		Limit:             limit,
	})
	if err != nil {
		return platformhttp.PageResponse[domain.Room]{}, err
	}
	s.enrichRoomsMedia(ctx, items)

	var curPtr *string
	if filter.Cursor != "" {
		c := filter.Cursor
		curPtr = &c
	}
	return platformhttp.PageResponse[domain.Room]{
		Data: items,
		Page: platformhttp.PageInfo{
			Limit:      limit,
			Cursor:     curPtr,
			NextCursor: nextCursor,
		},
	}, nil
}

// CreateRoom creates a physical room (`room.create`) in initial `available` operational status.
func (s *Service) CreateRoom(ctx context.Context, actor auth.Principal, params CreateRoomParams) (domain.Room, error) {
	if s.authz != nil {
		if err := s.authz.Authorize(ctx, actor, "room.create"); err != nil {
			return domain.Room{}, err
		}
	}

	now := s.clk.Now()
	var created domain.Room

	err := s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		rt, err := s.repo.GetRoomTypeByID(txCtx, params.RoomTypeID)
		if err != nil {
			if appErr, ok := apperror.As(err); ok && appErr.Code == apperror.CodeNotFound {
				return apperror.Validation("referenced roomTypeId does not exist", apperror.FieldError{
					Field:   "roomTypeId",
					Message: "room type not found",
				})
			}
			return err
		}
		if rt.ArchivedAt != nil {
			return apperror.Validation("cannot assign room to an archived room type", apperror.FieldError{
				Field:   "roomTypeId",
				Message: "room type is archived",
			})
		}

		capVal := rt.Capacity
		if params.Capacity != nil {
			capVal = *params.Capacity
		}

		candidate := domain.Room{
			ID:                s.ids.NewUUID(),
			Code:              params.Code,
			Name:              params.Name,
			RoomTypeID:        rt.ID,
			Floor:             params.Floor,
			Capacity:          capVal,
			OperationalStatus: domain.RoomStatusAvailable,
			Description:       params.Description,
			CreatedAt:         now,
		}
		if err := candidate.NormalizeAndValidateRoom(false, 0); err != nil {
			return err
		}

		rm, err := s.repo.CreateRoom(txCtx, candidate)
		if err != nil {
			return err
		}
		rm.RoomTypeCode = rt.Code
		rm.RoomTypeName = rt.Name

		if s.audit != nil {
			var actorID *uuid.UUID
			if actor.UserID != uuid.Nil {
				actorID = &actor.UserID
			}
			if err := s.audit.Record(txCtx, auditapp.AuditEntry{
				ActorUserID:   actorID,
				Action:        "property.room_created",
				ResourceType:  "room",
				ResourceID:    &rm.ID,
				RequestID:     platformhttp.RequestIDUUIDFromContext(txCtx),
				CorrelationID: platformhttp.CorrelationIDUUIDFromContext(txCtx),
				AfterData:     roomAuditMap(rm),
			}); err != nil {
				return err
			}
		}

		created = rm
		return nil
	})
	return created, err
}

// UpdateRoom updates room metadata (`room.update`) with optimistic locking.
// Invariant: operational_status is NEVER mutated here.
func (s *Service) UpdateRoom(ctx context.Context, actor auth.Principal, params UpdateRoomParams) (domain.Room, error) {
	if s.authz != nil {
		if err := s.authz.Authorize(ctx, actor, "room.update"); err != nil {
			return domain.Room{}, err
		}
	}
	if params.ExpectedVersion < 1 {
		return domain.Room{}, apperror.Validation("invalid version", apperror.FieldError{
			Field:   "version",
			Message: "version must be a positive integer",
		})
	}

	now := s.clk.Now()
	var updated domain.Room

	err := s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		before, err := s.repo.GetRoomByIDForUpdate(txCtx, params.ID)
		if err != nil {
			return err
		}
		if before.ArchivedAt != nil {
			return apperror.Conflict("cannot update an archived room")
		}
		if before.Version != params.ExpectedVersion {
			return apperror.VersionConflict("room version conflict")
		}

		candidate := before
		if params.Code != nil {
			candidate.Code = *params.Code
		}
		if params.Name != nil {
			candidate.Name = *params.Name
		}
		if params.RoomTypeID != nil && *params.RoomTypeID != before.RoomTypeID {
			rt, err := s.repo.GetRoomTypeByID(txCtx, *params.RoomTypeID)
			if err != nil {
				if appErr, ok := apperror.As(err); ok && appErr.Code == apperror.CodeNotFound {
					return apperror.Validation("referenced roomTypeId does not exist", apperror.FieldError{
						Field:   "roomTypeId",
						Message: "room type not found",
					})
				}
				return err
			}
			if rt.ArchivedAt != nil {
				return apperror.Validation("cannot assign room to an archived room type", apperror.FieldError{
					Field:   "roomTypeId",
					Message: "room type is archived",
				})
			}
			candidate.RoomTypeID = rt.ID
		}
		if params.Floor != nil {
			candidate.Floor = *params.Floor
		}
		if params.Capacity != nil {
			candidate.Capacity = *params.Capacity
		}
		if params.Description != nil {
			candidate.Description = *params.Description
		}

		if err := candidate.NormalizeAndValidateRoom(true, params.ExpectedVersion); err != nil {
			return err
		}

		after, err := s.repo.UpdateRoomMetadata(txCtx, candidate, params.ExpectedVersion, now)
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
				Action:        "property.room_updated",
				ResourceType:  "room",
				ResourceID:    &after.ID,
				RequestID:     platformhttp.RequestIDUUIDFromContext(txCtx),
				CorrelationID: platformhttp.CorrelationIDUUIDFromContext(txCtx),
				BeforeData:    roomAuditMap(before),
				AfterData:     roomAuditMap(after),
			}); err != nil {
				return err
			}
		}

		updated = after
		return nil
	})
	if err != nil {
		return domain.Room{}, err
	}
	slice := []domain.Room{updated}
	s.enrichRoomsMedia(ctx, slice)
	return slice[0], nil
}

// ArchiveRoom soft-archives a physical room (`room.archive`), refusing if the room is currently `occupied`.
func (s *Service) ArchiveRoom(ctx context.Context, actor auth.Principal, roomID uuid.UUID, expectedVersion *int) (domain.Room, error) {
	if s.authz != nil {
		if err := s.authz.Authorize(ctx, actor, "room.archive"); err != nil {
			return domain.Room{}, err
		}
	}

	now := s.clk.Now()
	var archived domain.Room

	err := s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		before, err := s.repo.GetRoomByIDForUpdate(txCtx, roomID)
		if err != nil {
			return err
		}
		if before.ArchivedAt != nil {
			return apperror.Conflict("room is already archived")
		}
		if expectedVersion != nil && before.Version != *expectedVersion {
			return apperror.VersionConflict("room version conflict")
		}
		if before.OperationalStatus == domain.RoomStatusOccupied {
			return apperror.Conflict("cannot archive a room while it is occupied")
		}

		after, err := s.repo.ArchiveRoom(txCtx, roomID, now)
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
				Action:        "property.room_archived",
				ResourceType:  "room",
				ResourceID:    &after.ID,
				RequestID:     platformhttp.RequestIDUUIDFromContext(txCtx),
				CorrelationID: platformhttp.CorrelationIDUUIDFromContext(txCtx),
				BeforeData:    roomAuditMap(before),
				AfterData:     roomAuditMap(after),
			}); err != nil {
				return err
			}
		}

		archived = after
		return nil
	})
	return archived, err
}

func (s *Service) enrichRoomTypesMedia(ctx context.Context, items []domain.RoomType) {
	if s.media == nil || len(items) == 0 {
		return
	}
	ids := make([]uuid.UUID, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	mediaMap, err := s.media.ListMediaByRoomTypeIDs(ctx, ids)
	if err != nil || len(mediaMap) == 0 {
		return
	}
	for i := range items {
		if list, ok := mediaMap[items[i].ID]; ok && len(list) > 0 {
			items[i].Media = list
			items[i].CoverURL = selectCoverURL(list)
		}
	}
}

func (s *Service) enrichRoomsMedia(ctx context.Context, items []domain.Room) {
	if s.media == nil || len(items) == 0 {
		return
	}
	ids := make([]uuid.UUID, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	mediaMap, err := s.media.ListMediaByRoomIDs(ctx, ids)
	if err != nil || len(mediaMap) == 0 {
		return
	}
	for i := range items {
		if list, ok := mediaMap[items[i].ID]; ok && len(list) > 0 {
			items[i].Media = list
			items[i].CoverURL = selectCoverURL(list)
		}
	}
}

func selectCoverURL(items []domain.MediaItem) string {
	for _, m := range items {
		if m.IsCover && m.URL != "" {
			return m.URL
		}
	}
	if len(items) > 0 {
		return items[0].URL
	}
	return ""
}

func parseOptionalDateRange(checkInRaw, checkOutRaw string) (time.Time, time.Time, error) {
	if checkInRaw == "" || checkOutRaw == "" {
		return time.Time{}, time.Time{}, apperror.Validation("both checkIn and checkOut are required when filtering by dates", apperror.FieldError{
			Field:   "checkIn",
			Message: "provide both checkIn and checkOut in YYYY-MM-DD format",
		})
	}
	inDate, err := time.Parse("2006-01-02", strings.TrimSpace(checkInRaw))
	if err != nil {
		return time.Time{}, time.Time{}, apperror.Validation("invalid checkIn date", apperror.FieldError{
			Field:   "checkIn",
			Message: "must be in YYYY-MM-DD format",
		})
	}
	outDate, err := time.Parse("2006-01-02", strings.TrimSpace(checkOutRaw))
	if err != nil {
		return time.Time{}, time.Time{}, apperror.Validation("invalid checkOut date", apperror.FieldError{
			Field:   "checkOut",
			Message: "must be in YYYY-MM-DD format",
		})
	}
	if !outDate.After(inDate) {
		return time.Time{}, time.Time{}, apperror.Validation("invalid stay date range", apperror.FieldError{
			Field:   "checkOut",
			Message: "must be after checkIn",
		})
	}
	return time.Date(inDate.Year(), inDate.Month(), inDate.Day(), 0, 0, 0, 0, time.UTC),
		time.Date(outDate.Year(), outDate.Month(), outDate.Day(), 0, 0, 0, 0, time.UTC),
		nil
}

func roomTypeAuditMap(rt domain.RoomType) map[string]any {
	m := map[string]any{
		"code":              rt.Code,
		"name":              rt.Name,
		"description":       rt.Description,
		"base_nightly_rate": rt.BaseNightlyRate.String(),
		"currency":          rt.Currency,
		"capacity":          rt.Capacity,
		"bed_type":          rt.BedType,
		"amenities":         rt.Amenities,
		"is_bookable":       rt.IsBookable,
		"version":           rt.Version,
	}
	if rt.AreaSqm != nil {
		m["area_sqm"] = rt.AreaSqm.String()
	}
	if rt.ArchivedAt != nil {
		m["archived_at"] = rt.ArchivedAt.UTC().Format(time.RFC3339)
	}
	return m
}

func roomAuditMap(rm domain.Room) map[string]any {
	m := map[string]any{
		"code":               rm.Code,
		"name":               rm.Name,
		"room_type_id":       rm.RoomTypeID.String(),
		"floor":              rm.Floor,
		"capacity":           rm.Capacity,
		"operational_status": rm.OperationalStatus,
		"description":        rm.Description,
		"version":            rm.Version,
	}
	if rm.ArchivedAt != nil {
		m["archived_at"] = rm.ArchivedAt.UTC().Format(time.RFC3339)
	}
	return m
}
