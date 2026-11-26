package postgres

import (
	"context"
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

// ServiceCatalogListQuery defines filter and cursor options for listing hotel ancillary services.
type ServiceCatalogListQuery struct {
	Category string
	IsActive *bool
	Cursor   string
	Limit    int
}

func (r *Repository) CreateServiceCatalogItem(ctx context.Context, item domain.ServiceCatalogItem) (domain.ServiceCatalogItem, error) {
	row, err := r.queries(ctx).CreateServiceCatalogItem(ctx, sqlcgen.CreateServiceCatalogItemParams{
		ID:          item.ID,
		Code:        item.Code,
		Name:        item.Name,
		Category:    item.Category,
		UnitPrice:   item.UnitPrice,
		Currency:    item.Currency,
		Description: toPgText(item.Description),
		IsActive:    item.IsActive,
		CreatedAt:   item.CreatedAt,
	})
	if err != nil {
		if db.IsUniqueViolation(err) {
			return domain.ServiceCatalogItem{}, apperror.Conflict("service code already exists").WithCause(err)
		}
		return domain.ServiceCatalogItem{}, db.MapPgError(err)
	}
	return mapServiceCatalogFields(
		row.ID, row.Code, row.Name, row.Category, row.UnitPrice, row.Currency,
		row.Description, row.IsActive, row.CreatedAt, row.UpdatedAt, row.Version,
	), nil
}

func (r *Repository) GetServiceCatalogItemByID(ctx context.Context, id uuid.UUID) (domain.ServiceCatalogItem, error) {
	row, err := r.queries(ctx).GetServiceCatalogItemByID(ctx, id)
	if err != nil {
		return domain.ServiceCatalogItem{}, db.MapPgError(err)
	}
	return mapServiceCatalogFields(
		row.ID, row.Code, row.Name, row.Category, row.UnitPrice, row.Currency,
		row.Description, row.IsActive, row.CreatedAt, row.UpdatedAt, row.Version,
	), nil
}

func (r *Repository) GetServiceCatalogItemByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.ServiceCatalogItem, error) {
	row, err := r.queries(ctx).GetServiceCatalogItemByIDForUpdate(ctx, id)
	if err != nil {
		return domain.ServiceCatalogItem{}, db.MapPgError(err)
	}
	return mapServiceCatalogFields(
		row.ID, row.Code, row.Name, row.Category, row.UnitPrice, row.Currency,
		row.Description, row.IsActive, row.CreatedAt, row.UpdatedAt, row.Version,
	), nil
}

func (r *Repository) UpdateServiceCatalogItem(ctx context.Context, item domain.ServiceCatalogItem, expectedVersion int, now time.Time) (domain.ServiceCatalogItem, error) {
	row, err := r.queries(ctx).UpdateServiceCatalogItem(ctx, sqlcgen.UpdateServiceCatalogItemParams{
		ID:              item.ID,
		Name:            item.Name,
		Category:        item.Category,
		UnitPrice:       item.UnitPrice,
		Currency:        item.Currency,
		Description:     toPgText(item.Description),
		IsActive:        item.IsActive,
		UpdatedAt:       now,
		ExpectedVersion: int32(expectedVersion),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ServiceCatalogItem{}, apperror.VersionConflict("service catalog version conflict").WithCause(err)
		}
		return domain.ServiceCatalogItem{}, db.MapPgError(err)
	}
	return mapServiceCatalogFields(
		row.ID, row.Code, row.Name, row.Category, row.UnitPrice, row.Currency,
		row.Description, row.IsActive, row.CreatedAt, row.UpdatedAt, row.Version,
	), nil
}

func (r *Repository) DeactivateServiceCatalogItem(ctx context.Context, id uuid.UUID, now time.Time) (domain.ServiceCatalogItem, error) {
	row, err := r.queries(ctx).DeactivateServiceCatalogItem(ctx, sqlcgen.DeactivateServiceCatalogItemParams{
		ID:        id,
		UpdatedAt: now,
	})
	if err != nil {
		return domain.ServiceCatalogItem{}, db.MapPgError(err)
	}
	return mapServiceCatalogFields(
		row.ID, row.Code, row.Name, row.Category, row.UnitPrice, row.Currency,
		row.Description, row.IsActive, row.CreatedAt, row.UpdatedAt, row.Version,
	), nil
}

func (r *Repository) ListServiceCatalogItems(ctx context.Context, q ServiceCatalogListQuery) ([]domain.ServiceCatalogItem, *string, error) {
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

	if cat := strings.ToLower(strings.TrimSpace(q.Category)); cat != "" {
		conds = append(conds, fmt.Sprintf("category = %s", nextArg(cat)))
	}
	if q.IsActive != nil {
		conds = append(conds, fmt.Sprintf("is_active = %s", nextArg(*q.IsActive)))
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
		conds = append(conds, fmt.Sprintf("(created_at < %s OR (created_at = %s AND id < %s))", tArg, tArg, idArg))
	}

	limitArg := nextArg(limit + 1)
	sqlText := fmt.Sprintf(`
SELECT
    id,
    code,
    name,
    category,
    unit_price,
    btrim(currency)::text AS currency,
    description,
    is_active,
    created_at,
    updated_at,
    version
FROM service_catalog
WHERE %s
ORDER BY created_at DESC, id DESC
LIMIT %s`, strings.Join(conds, " AND "), limitArg)

	rows, err := db.Querier(ctx, r.pool).Query(ctx, sqlText, args...)
	if err != nil {
		return nil, nil, db.MapPgError(err)
	}
	defer rows.Close()

	items := make([]domain.ServiceCatalogItem, 0, limit)
	for rows.Next() {
		var (
			id                        uuid.UUID
			code, name, category, cur string
			price                     decimal.Decimal
			desc                      pgtype.Text
			active                    bool
			createdAt, updatedAt      time.Time
			ver                       int32
		)
		if err := rows.Scan(
			&id, &code, &name, &category, &price, &cur, &desc, &active, &createdAt, &updatedAt, &ver,
		); err != nil {
			return nil, nil, db.MapPgError(err)
		}
		items = append(items, mapServiceCatalogFields(id, code, name, category, price, cur, desc, active, createdAt, updatedAt, ver))
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

func mapServiceCatalogFields(
	id uuid.UUID,
	code, name, category string,
	unitPrice decimal.Decimal,
	currency string,
	desc pgtype.Text,
	isActive bool,
	createdAt, updatedAt time.Time,
	version int32,
) domain.ServiceCatalogItem {
	out := domain.ServiceCatalogItem{
		ID:        id,
		Code:      code,
		Name:      name,
		Category:  category,
		UnitPrice: unitPrice,
		Currency:  currency,
		IsActive:  isActive,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		Version:   int(version),
	}
	if desc.Valid {
		out.Description = desc.String
	}
	return out
}
