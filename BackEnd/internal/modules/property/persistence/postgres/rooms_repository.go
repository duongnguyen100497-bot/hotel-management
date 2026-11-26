package postgres

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/property/domain"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/property/persistence/postgres/sqlcgen"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/db"
)

// PublicCatalogQuery defines validated filter, sort, and cursor options for public room-type search.
type PublicCatalogQuery struct {
	Capacity *int
	MinPrice *decimal.Decimal
	MaxPrice *decimal.Decimal
	Sort     string
	Cursor   string
	Limit    int
}

// StaffRoomTypeQuery defines filter and cursor options for staff room-type listing.
type StaffRoomTypeQuery struct {
	IncludeArchived bool
	IsBookable      *bool
	Cursor          string
	Limit           int
}

// StaffRoomQuery defines filter and cursor options for staff physical room listing.
type StaffRoomQuery struct {
	RoomTypeID        *uuid.UUID
	OperationalStatus string
	Floor             *int
	IncludeArchived   bool
	Cursor            string
	Limit             int
}

type catalogCursor struct {
	Sort  string    `json:"s"`
	Price string    `json:"p,omitempty"`
	Cap   int       `json:"c,omitempty"`
	TS    time.Time `json:"t,omitempty"`
	ID    uuid.UUID `json:"id"`
}

type timeIDCursor struct {
	TS time.Time `json:"t"`
	ID uuid.UUID `json:"id"`
}

type roomListCursor struct {
	Floor int       `json:"f"`
	Code  string    `json:"c"`
	ID    uuid.UUID `json:"id"`
}

func (r *Repository) CreateRoomType(ctx context.Context, rt domain.RoomType) (domain.RoomType, error) {
	amenitiesJSON, err := encodeAmenities(rt.Amenities)
	if err != nil {
		return domain.RoomType{}, apperror.Internal(err)
	}
	row, err := r.queries(ctx).CreateRoomType(ctx, sqlcgen.CreateRoomTypeParams{
		ID:              rt.ID,
		Code:            rt.Code,
		Name:            rt.Name,
		Description:     toPgText(rt.Description),
		BaseNightlyRate: rt.BaseNightlyRate,
		Currency:        rt.Currency,
		Capacity:        int16(rt.Capacity),
		AreaSqm:         toNullDecimal(rt.AreaSqm),
		BedType:         toPgText(rt.BedType),
		Amenities:       amenitiesJSON,
		IsBookable:      rt.IsBookable,
		CreatedAt:       rt.CreatedAt,
	})
	if err != nil {
		if db.IsUniqueViolation(err) {
			return domain.RoomType{}, apperror.Conflict("room type code already exists").WithCause(err)
		}
		return domain.RoomType{}, db.MapPgError(err)
	}
	return mapRoomTypeRow(
		row.ID, row.Code, row.Name, row.Description, row.BaseNightlyRate, row.Currency,
		row.Capacity, row.AreaSqm, row.BedType, row.Amenities, row.IsBookable,
		row.ArchivedAt, row.CreatedAt, row.UpdatedAt, row.Version, 0,
	)
}

func (r *Repository) GetRoomTypeByID(ctx context.Context, id uuid.UUID) (domain.RoomType, error) {
	row, err := r.queries(ctx).GetRoomTypeByID(ctx, id)
	if err != nil {
		return domain.RoomType{}, db.MapPgError(err)
	}
	return mapRoomTypeRow(
		row.ID, row.Code, row.Name, row.Description, row.BaseNightlyRate, row.Currency,
		row.Capacity, row.AreaSqm, row.BedType, row.Amenities, row.IsBookable,
		row.ArchivedAt, row.CreatedAt, row.UpdatedAt, row.Version, 0,
	)
}

func (r *Repository) GetRoomTypeByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.RoomType, error) {
	row, err := r.queries(ctx).GetRoomTypeByIDForUpdate(ctx, id)
	if err != nil {
		return domain.RoomType{}, db.MapPgError(err)
	}
	return mapRoomTypeRow(
		row.ID, row.Code, row.Name, row.Description, row.BaseNightlyRate, row.Currency,
		row.Capacity, row.AreaSqm, row.BedType, row.Amenities, row.IsBookable,
		row.ArchivedAt, row.CreatedAt, row.UpdatedAt, row.Version, 0,
	)
}

func (r *Repository) GetPublicRoomTypeByCode(ctx context.Context, code string) (domain.RoomType, error) {
	norm := strings.ToUpper(strings.TrimSpace(code))
	row, err := r.queries(ctx).GetPublicRoomTypeByCode(ctx, norm)
	if err != nil {
		return domain.RoomType{}, db.MapPgError(err)
	}
	return mapRoomTypeRow(
		row.ID, row.Code, row.Name, row.Description, row.BaseNightlyRate, row.Currency,
		row.Capacity, row.AreaSqm, row.BedType, row.Amenities, row.IsBookable,
		row.ArchivedAt, row.CreatedAt, row.UpdatedAt, row.Version, int(row.OperationalRoomCount),
	)
}

func (r *Repository) UpdateRoomType(ctx context.Context, rt domain.RoomType, expectedVersion int, now time.Time) (domain.RoomType, error) {
	amenitiesJSON, err := encodeAmenities(rt.Amenities)
	if err != nil {
		return domain.RoomType{}, apperror.Internal(err)
	}
	row, err := r.queries(ctx).UpdateRoomType(ctx, sqlcgen.UpdateRoomTypeParams{
		ID:              rt.ID,
		Name:            rt.Name,
		Description:     toPgText(rt.Description),
		BaseNightlyRate: rt.BaseNightlyRate,
		Currency:        rt.Currency,
		Capacity:        int16(rt.Capacity),
		AreaSqm:         toNullDecimal(rt.AreaSqm),
		BedType:         toPgText(rt.BedType),
		Amenities:       amenitiesJSON,
		IsBookable:      rt.IsBookable,
		ArchivedAt:      rt.ArchivedAt,
		UpdatedAt:       now,
		ExpectedVersion: int32(expectedVersion),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.RoomType{}, apperror.VersionConflict("room type version conflict").WithCause(err)
		}
		return domain.RoomType{}, db.MapPgError(err)
	}
	return mapRoomTypeRow(
		row.ID, row.Code, row.Name, row.Description, row.BaseNightlyRate, row.Currency,
		row.Capacity, row.AreaSqm, row.BedType, row.Amenities, row.IsBookable,
		row.ArchivedAt, row.CreatedAt, row.UpdatedAt, row.Version, 0,
	)
}

func (r *Repository) SearchPublicRoomTypes(ctx context.Context, q PublicCatalogQuery) ([]domain.RoomType, *string, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = 20
	}
	sortMode := q.Sort
	if sortMode == "" {
		sortMode = "price_asc"
	}

	conds := []string{"rt.is_bookable = true", "rt.archived_at IS NULL"}
	args := make([]any, 0, 8)
	nextArg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if q.Capacity != nil {
		conds = append(conds, fmt.Sprintf("rt.capacity >= %s", nextArg(int16(*q.Capacity))))
	}
	if q.MinPrice != nil {
		conds = append(conds, fmt.Sprintf("rt.base_nightly_rate >= %s", nextArg(*q.MinPrice)))
	}
	if q.MaxPrice != nil {
		conds = append(conds, fmt.Sprintf("rt.base_nightly_rate <= %s", nextArg(*q.MaxPrice)))
	}

	if q.Cursor != "" {
		cur, err := decodeCursor[catalogCursor](q.Cursor)
		if err != nil || cur.ID == uuid.Nil || cur.Sort != sortMode {
			return nil, nil, apperror.Validation("invalid pagination cursor", apperror.FieldError{
				Field:   "cursor",
				Message: "malformed or mismatched sort cursor",
			})
		}
		switch sortMode {
		case "price_asc":
			p, err := decimal.NewFromString(cur.Price)
			if err != nil {
				return nil, nil, apperror.Validation("invalid pagination cursor")
			}
			pArg := nextArg(p)
			idArg := nextArg(cur.ID)
			conds = append(conds, fmt.Sprintf("(rt.base_nightly_rate > %s OR (rt.base_nightly_rate = %s AND rt.id > %s))", pArg, pArg, idArg))
		case "price_desc":
			p, err := decimal.NewFromString(cur.Price)
			if err != nil {
				return nil, nil, apperror.Validation("invalid pagination cursor")
			}
			pArg := nextArg(p)
			idArg := nextArg(cur.ID)
			conds = append(conds, fmt.Sprintf("(rt.base_nightly_rate < %s OR (rt.base_nightly_rate = %s AND rt.id > %s))", pArg, pArg, idArg))
		case "capacity_asc":
			cArg := nextArg(int16(cur.Cap))
			idArg := nextArg(cur.ID)
			conds = append(conds, fmt.Sprintf("(rt.capacity > %s OR (rt.capacity = %s AND rt.id > %s))", cArg, cArg, idArg))
		case "capacity_desc":
			cArg := nextArg(int16(cur.Cap))
			idArg := nextArg(cur.ID)
			conds = append(conds, fmt.Sprintf("(rt.capacity < %s OR (rt.capacity = %s AND rt.id > %s))", cArg, cArg, idArg))
		case "created_asc":
			tArg := nextArg(cur.TS)
			idArg := nextArg(cur.ID)
			conds = append(conds, fmt.Sprintf("(rt.created_at > %s OR (rt.created_at = %s AND rt.id > %s))", tArg, tArg, idArg))
		case "created_desc":
			tArg := nextArg(cur.TS)
			idArg := nextArg(cur.ID)
			conds = append(conds, fmt.Sprintf("(rt.created_at < %s OR (rt.created_at = %s AND rt.id > %s))", tArg, tArg, idArg))
		}
	}

	var orderBy string
	switch sortMode {
	case "price_asc":
		orderBy = "rt.base_nightly_rate ASC, rt.id ASC"
	case "price_desc":
		orderBy = "rt.base_nightly_rate DESC, rt.id ASC"
	case "capacity_asc":
		orderBy = "rt.capacity ASC, rt.id ASC"
	case "capacity_desc":
		orderBy = "rt.capacity DESC, rt.id ASC"
	case "created_asc":
		orderBy = "rt.created_at ASC, rt.id ASC"
	case "created_desc":
		orderBy = "rt.created_at DESC, rt.id ASC"
	default:
		return nil, nil, apperror.Validation("unsupported sort parameter", apperror.FieldError{
			Field:   "sort",
			Message: "allowed values: price_asc, price_desc, capacity_asc, capacity_desc, created_asc, created_desc",
		})
	}

	limitArg := nextArg(limit + 1)
	sqlText := fmt.Sprintf(`
SELECT
    rt.id,
    rt.code,
    rt.name,
    rt.description,
    rt.base_nightly_rate,
    btrim(rt.currency)::text AS currency,
    rt.capacity,
    rt.area_sqm,
    rt.bed_type,
    rt.amenities,
    rt.is_bookable,
    rt.archived_at,
    rt.created_at,
    rt.updated_at,
    rt.version,
    (
        SELECT count(*)::int
        FROM rooms r
        WHERE r.room_type_id = rt.id
          AND r.archived_at IS NULL
          AND r.operational_status <> 'maintenance'
    ) AS operational_room_count
FROM room_types rt
WHERE %s
ORDER BY %s
LIMIT %s`, strings.Join(conds, " AND "), orderBy, limitArg)

	rows, err := db.Querier(ctx, r.pool).Query(ctx, sqlText, args...)
	if err != nil {
		return nil, nil, db.MapPgError(err)
	}
	defer rows.Close()

	items := make([]domain.RoomType, 0, limit)
	for rows.Next() {
		var (
			id              uuid.UUID
			code, name, cur string
			desc, bedType   pgtype.Text
			rate            decimal.Decimal
			capVal          int16
			area            decimal.NullDecimal
			amenities       []byte
			bookable        bool
			archivedAt      *time.Time
			createdAt       time.Time
			updatedAt       time.Time
			version         int32
			opCount         int32
		)
		if err := rows.Scan(
			&id, &code, &name, &desc, &rate, &cur, &capVal, &area, &bedType,
			&amenities, &bookable, &archivedAt, &createdAt, &updatedAt, &version, &opCount,
		); err != nil {
			return nil, nil, db.MapPgError(err)
		}
		mapped, err := mapRoomTypeRow(
			id, code, name, desc, rate, cur, capVal, area, bedType,
			amenities, bookable, archivedAt, createdAt, updatedAt, version, int(opCount),
		)
		if err != nil {
			return nil, nil, err
		}
		items = append(items, mapped)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, db.MapPgError(err)
	}

	var nextCursor *string
	if len(items) > limit {
		items = items[:limit]
		last := items[len(items)-1]
		encoded := encodeCursor(catalogCursor{
			Sort:  sortMode,
			Price: last.BaseNightlyRate.String(),
			Cap:   last.Capacity,
			TS:    last.CreatedAt,
			ID:    last.ID,
		})
		nextCursor = &encoded
	}
	return items, nextCursor, nil
}

func (r *Repository) ListStaffRoomTypes(ctx context.Context, q StaffRoomTypeQuery) ([]domain.RoomType, *string, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = 20
	}
	conds := []string{"1=1"}
	args := make([]any, 0, 6)
	nextArg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if !q.IncludeArchived {
		conds = append(conds, "rt.archived_at IS NULL")
	}
	if q.IsBookable != nil {
		conds = append(conds, fmt.Sprintf("rt.is_bookable = %s", nextArg(*q.IsBookable)))
	}
	if q.Cursor != "" {
		cur, err := decodeCursor[timeIDCursor](q.Cursor)
		if err != nil || cur.ID == uuid.Nil {
			return nil, nil, apperror.Validation("invalid pagination cursor", apperror.FieldError{
				Field:   "cursor",
				Message: "malformed cursor",
			})
		}
		tArg := nextArg(cur.TS)
		idArg := nextArg(cur.ID)
		conds = append(conds, fmt.Sprintf("(rt.created_at < %s OR (rt.created_at = %s AND rt.id < %s))", tArg, tArg, idArg))
	}

	limitArg := nextArg(limit + 1)
	sqlText := fmt.Sprintf(`
SELECT
    rt.id,
    rt.code,
    rt.name,
    rt.description,
    rt.base_nightly_rate,
    btrim(rt.currency)::text AS currency,
    rt.capacity,
    rt.area_sqm,
    rt.bed_type,
    rt.amenities,
    rt.is_bookable,
    rt.archived_at,
    rt.created_at,
    rt.updated_at,
    rt.version,
    (
        SELECT count(*)::int
        FROM rooms r
        WHERE r.room_type_id = rt.id
          AND r.archived_at IS NULL
          AND r.operational_status <> 'maintenance'
    ) AS operational_room_count
FROM room_types rt
WHERE %s
ORDER BY rt.created_at DESC, rt.id DESC
LIMIT %s`, strings.Join(conds, " AND "), limitArg)

	rows, err := db.Querier(ctx, r.pool).Query(ctx, sqlText, args...)
	if err != nil {
		return nil, nil, db.MapPgError(err)
	}
	defer rows.Close()

	items := make([]domain.RoomType, 0, limit)
	for rows.Next() {
		var (
			id              uuid.UUID
			code, name, cur string
			desc, bedType   pgtype.Text
			rate            decimal.Decimal
			capVal          int16
			area            decimal.NullDecimal
			amenities       []byte
			bookable        bool
			archivedAt      *time.Time
			createdAt       time.Time
			updatedAt       time.Time
			version         int32
			opCount         int32
		)
		if err := rows.Scan(
			&id, &code, &name, &desc, &rate, &cur, &capVal, &area, &bedType,
			&amenities, &bookable, &archivedAt, &createdAt, &updatedAt, &version, &opCount,
		); err != nil {
			return nil, nil, db.MapPgError(err)
		}
		mapped, err := mapRoomTypeRow(
			id, code, name, desc, rate, cur, capVal, area, bedType,
			amenities, bookable, archivedAt, createdAt, updatedAt, version, int(opCount),
		)
		if err != nil {
			return nil, nil, err
		}
		items = append(items, mapped)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, db.MapPgError(err)
	}

	var nextCursor *string
	if len(items) > limit {
		items = items[:limit]
		last := items[len(items)-1]
		encoded := encodeCursor(timeIDCursor{TS: last.CreatedAt, ID: last.ID})
		nextCursor = &encoded
	}
	return items, nextCursor, nil
}

func (r *Repository) CreateRoom(ctx context.Context, rm domain.Room) (domain.Room, error) {
	row, err := r.queries(ctx).CreateRoom(ctx, sqlcgen.CreateRoomParams{
		ID:          rm.ID,
		Code:        rm.Code,
		Name:        rm.Name,
		RoomTypeID:  rm.RoomTypeID,
		Floor:       int16(rm.Floor),
		Capacity:    int16(rm.Capacity),
		Description: toPgText(rm.Description),
		CreatedAt:   rm.CreatedAt,
	})
	if err != nil {
		if db.IsUniqueViolation(err) {
			return domain.Room{}, apperror.Conflict("room code already exists").WithCause(err)
		}
		return domain.Room{}, db.MapPgError(err)
	}
	return mapRoom(row), nil
}

func (r *Repository) GetRoomByID(ctx context.Context, id uuid.UUID) (domain.Room, error) {
	row, err := r.queries(ctx).GetRoomByID(ctx, id)
	if err != nil {
		return domain.Room{}, db.MapPgError(err)
	}
	return mapRoom(row), nil
}

func (r *Repository) GetRoomByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.Room, error) {
	row, err := r.queries(ctx).GetRoomByIDForUpdate(ctx, id)
	if err != nil {
		return domain.Room{}, db.MapPgError(err)
	}
	return mapRoom(row), nil
}

func (r *Repository) UpdateRoomMetadata(ctx context.Context, rm domain.Room, expectedVersion int, now time.Time) (domain.Room, error) {
	row, err := r.queries(ctx).UpdateRoomMetadata(ctx, sqlcgen.UpdateRoomMetadataParams{
		ID:              rm.ID,
		Code:            rm.Code,
		Name:            rm.Name,
		RoomTypeID:      rm.RoomTypeID,
		Floor:           int16(rm.Floor),
		Capacity:        int16(rm.Capacity),
		Description:     toPgText(rm.Description),
		UpdatedAt:       now,
		ExpectedVersion: int32(expectedVersion),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Room{}, apperror.VersionConflict("room version conflict").WithCause(err)
		}
		if db.IsUniqueViolation(err) {
			return domain.Room{}, apperror.Conflict("room code already exists").WithCause(err)
		}
		return domain.Room{}, db.MapPgError(err)
	}
	return mapRoom(row), nil
}

func (r *Repository) ArchiveRoom(ctx context.Context, id uuid.UUID, archivedAt time.Time) (domain.Room, error) {
	row, err := r.queries(ctx).ArchiveRoom(ctx, sqlcgen.ArchiveRoomParams{
		ID:         id,
		ArchivedAt: &archivedAt,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Room{}, apperror.Conflict("room cannot be archived (occupied or already archived)").WithCause(err)
		}
		return domain.Room{}, db.MapPgError(err)
	}
	return mapRoom(row), nil
}

func (r *Repository) ListBookableRoomsByType(ctx context.Context, roomTypeID uuid.UUID) ([]domain.Room, error) {
	rows, err := r.queries(ctx).ListBookableRoomsByType(ctx, roomTypeID)
	if err != nil {
		return nil, db.MapPgError(err)
	}
	out := make([]domain.Room, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapRoom(row))
	}
	return out, nil
}

func (r *Repository) ListStaffRooms(ctx context.Context, q StaffRoomQuery) ([]domain.Room, *string, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = 20
	}
	conds := []string{"1=1"}
	args := make([]any, 0, 8)
	nextArg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if !q.IncludeArchived {
		conds = append(conds, "rm.archived_at IS NULL")
	}
	if q.RoomTypeID != nil {
		conds = append(conds, fmt.Sprintf("rm.room_type_id = %s", nextArg(*q.RoomTypeID)))
	}
	if q.OperationalStatus != "" {
		conds = append(conds, fmt.Sprintf("rm.operational_status = %s", nextArg(q.OperationalStatus)))
	}
	if q.Floor != nil {
		conds = append(conds, fmt.Sprintf("rm.floor = %s", nextArg(int16(*q.Floor))))
	}
	if q.Cursor != "" {
		cur, err := decodeCursor[roomListCursor](q.Cursor)
		if err != nil || cur.ID == uuid.Nil {
			return nil, nil, apperror.Validation("invalid pagination cursor", apperror.FieldError{
				Field:   "cursor",
				Message: "malformed cursor",
			})
		}
		fArg := nextArg(int16(cur.Floor))
		cArg := nextArg(cur.Code)
		idArg := nextArg(cur.ID)
		conds = append(conds, fmt.Sprintf(
			"(rm.floor > %s OR (rm.floor = %s AND rm.code > %s) OR (rm.floor = %s AND rm.code = %s AND rm.id > %s))",
			fArg, fArg, cArg, fArg, cArg, idArg,
		))
	}

	limitArg := nextArg(limit + 1)
	sqlText := fmt.Sprintf(`
SELECT
    rm.id,
    rm.code,
    rm.name,
    rm.room_type_id,
    rt.code AS room_type_code,
    rt.name AS room_type_name,
    rm.floor,
    rm.capacity,
    rm.operational_status,
    rm.description,
    rm.archived_at,
    rm.created_at,
    rm.updated_at,
    rm.version
FROM rooms rm
JOIN room_types rt ON rt.id = rm.room_type_id
WHERE %s
ORDER BY rm.floor ASC, rm.code ASC, rm.id ASC
LIMIT %s`, strings.Join(conds, " AND "), limitArg)

	rows, err := db.Querier(ctx, r.pool).Query(ctx, sqlText, args...)
	if err != nil {
		return nil, nil, db.MapPgError(err)
	}
	defer rows.Close()

	items := make([]domain.Room, 0, limit)
	for rows.Next() {
		var (
			rm     domain.Room
			floor  int16
			capVal int16
			desc   pgtype.Text
			ver    int32
		)
		if err := rows.Scan(
			&rm.ID, &rm.Code, &rm.Name, &rm.RoomTypeID, &rm.RoomTypeCode, &rm.RoomTypeName,
			&floor, &capVal, &rm.OperationalStatus, &desc, &rm.ArchivedAt,
			&rm.CreatedAt, &rm.UpdatedAt, &ver,
		); err != nil {
			return nil, nil, db.MapPgError(err)
		}
		rm.Floor = int(floor)
		rm.Capacity = int(capVal)
		if desc.Valid {
			rm.Description = desc.String
		}
		rm.Version = int(ver)
		items = append(items, rm)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, db.MapPgError(err)
	}

	var nextCursor *string
	if len(items) > limit {
		items = items[:limit]
		last := items[len(items)-1]
		encoded := encodeCursor(roomListCursor{
			Floor: last.Floor,
			Code:  last.Code,
			ID:    last.ID,
		})
		nextCursor = &encoded
	}
	return items, nextCursor, nil
}

func mapRoomTypeRow(
	id uuid.UUID,
	code, name string,
	desc pgtype.Text,
	rate decimal.Decimal,
	currency string,
	capacity int16,
	areaSqm decimal.NullDecimal,
	bedType pgtype.Text,
	amenitiesRaw []byte,
	isBookable bool,
	archivedAt *time.Time,
	createdAt, updatedAt time.Time,
	version int32,
	opCount int,
) (domain.RoomType, error) {
	amenities := []string{}
	if len(amenitiesRaw) > 0 {
		if err := json.Unmarshal(amenitiesRaw, &amenities); err != nil {
			return domain.RoomType{}, apperror.Internal(err)
		}
	}
	var areaPtr *decimal.Decimal
	if areaSqm.Valid {
		v := areaSqm.Decimal
		areaPtr = &v
	}
	out := domain.RoomType{
		ID:                   id,
		Code:                 code,
		Name:                 name,
		BaseNightlyRate:      rate,
		Currency:             currency,
		Capacity:             int(capacity),
		AreaSqm:              areaPtr,
		Amenities:            amenities,
		IsBookable:           isBookable,
		OperationalRoomCount: opCount,
		ArchivedAt:           archivedAt,
		CreatedAt:            createdAt,
		UpdatedAt:            updatedAt,
		Version:              int(version),
	}
	if desc.Valid {
		out.Description = desc.String
	}
	if bedType.Valid {
		out.BedType = bedType.String
	}
	return out, nil
}

func mapRoom(row sqlcgen.Room) domain.Room {
	rm := domain.Room{
		ID:                row.ID,
		Code:              row.Code,
		Name:              row.Name,
		RoomTypeID:        row.RoomTypeID,
		Floor:             int(row.Floor),
		Capacity:          int(row.Capacity),
		OperationalStatus: row.OperationalStatus,
		ArchivedAt:        row.ArchivedAt,
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
		Version:           int(row.Version),
	}
	if row.Description.Valid {
		rm.Description = row.Description.String
	}
	return rm
}

func encodeAmenities(amenities []string) ([]byte, error) {
	if amenities == nil {
		amenities = []string{}
	}
	return json.Marshal(amenities)
}

func toPgText(s string) pgtype.Text {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: trimmed, Valid: true}
}

func toNullDecimal(d *decimal.Decimal) decimal.NullDecimal {
	if d == nil {
		return decimal.NullDecimal{Valid: false}
	}
	return decimal.NullDecimal{Decimal: *d, Valid: true}
}

func encodeCursor(v any) string {
	b, _ := json.Marshal(v)
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeCursor[T any](raw string) (T, error) {
	var zero T
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil {
		return zero, err
	}
	if err := json.Unmarshal(decoded, &zero); err != nil {
		return zero, err
	}
	return zero, nil
}
