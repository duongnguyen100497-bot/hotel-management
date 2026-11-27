package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/people/domain"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/people/persistence/postgres/sqlcgen"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/db"
)

// Repository persists employees and customers in PostgreSQL.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository constructs a PostgreSQL people repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) queries(ctx context.Context) *sqlcgen.Queries {
	return sqlcgen.New(db.Querier(ctx, r.pool))
}

func (r *Repository) CreateEmployee(ctx context.Context, emp domain.Employee) (domain.Employee, error) {
	row, err := r.queries(ctx).CreateEmployee(ctx, sqlcgen.CreateEmployeeParams{
		ID:               emp.ID,
		UserID:           emp.UserID,
		EmployeeCode:     strings.TrimSpace(emp.EmployeeCode),
		FullName:         strings.TrimSpace(emp.FullName),
		Phone:            strings.TrimSpace(emp.Phone),
		Email:            strings.TrimSpace(emp.Email),
		PositionTitle:    strings.TrimSpace(emp.PositionTitle),
		EmploymentStatus: emp.EmploymentStatus,
		CreatedAt:        emp.CreatedAt.UTC(),
	})
	if err != nil {
		if db.IsUniqueViolation(err) {
			return domain.Employee{}, apperror.Conflict("employee code or email already exists").WithCause(err)
		}
		return domain.Employee{}, db.MapPgError(err)
	}
	return mapEmployeeRow(row.ID, row.UserID, row.EmployeeCode, row.FullName, row.Phone, row.Email, row.PositionTitle, row.EmploymentStatus, row.ArchivedAt, row.CreatedAt, row.UpdatedAt, row.Version), nil
}

func (r *Repository) GetEmployeeByID(ctx context.Context, id uuid.UUID) (domain.Employee, error) {
	row, err := r.queries(ctx).GetEmployeeByID(ctx, id)
	if err != nil {
		return domain.Employee{}, db.MapPgError(err)
	}
	return mapEmployeeRow(row.ID, row.UserID, row.EmployeeCode, row.FullName, row.Phone, row.Email, row.PositionTitle, row.EmploymentStatus, row.ArchivedAt, row.CreatedAt, row.UpdatedAt, row.Version), nil
}

func (r *Repository) GetEmployeeByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.Employee, error) {
	row, err := r.queries(ctx).GetEmployeeByIDForUpdate(ctx, id)
	if err != nil {
		return domain.Employee{}, db.MapPgError(err)
	}
	return mapEmployeeRow(row.ID, row.UserID, row.EmployeeCode, row.FullName, row.Phone, row.Email, row.PositionTitle, row.EmploymentStatus, row.ArchivedAt, row.CreatedAt, row.UpdatedAt, row.Version), nil
}

func (r *Repository) GetEmployeeByUserID(ctx context.Context, userID uuid.UUID) (domain.Employee, error) {
	row, err := r.queries(ctx).GetEmployeeByUserID(ctx, userID)
	if err != nil {
		return domain.Employee{}, db.MapPgError(err)
	}
	return mapEmployeeRow(row.ID, row.UserID, row.EmployeeCode, row.FullName, row.Phone, row.Email, row.PositionTitle, row.EmploymentStatus, row.ArchivedAt, row.CreatedAt, row.UpdatedAt, row.Version), nil
}

func (r *Repository) ListEmployees(ctx context.Context, limit int) ([]domain.Employee, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.queries(ctx).ListEmployees(ctx, int32(limit))
	if err != nil {
		return nil, db.MapPgError(err)
	}
	out := make([]domain.Employee, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapEmployeeRow(row.ID, row.UserID, row.EmployeeCode, row.FullName, row.Phone, row.Email, row.PositionTitle, row.EmploymentStatus, row.ArchivedAt, row.CreatedAt, row.UpdatedAt, row.Version))
	}
	return out, nil
}

func (r *Repository) UpdateEmployeeOptimistic(ctx context.Context, id uuid.UUID, fullName, phone, positionTitle string, expectedVersion int, now time.Time) (domain.Employee, error) {
	row, err := r.queries(ctx).UpdateEmployeeOptimistic(ctx, sqlcgen.UpdateEmployeeOptimisticParams{
		ID:            id,
		FullName:      strings.TrimSpace(fullName),
		Phone:         strings.TrimSpace(phone),
		PositionTitle: strings.TrimSpace(positionTitle),
		UpdatedAt:     now.UTC(),
		Version:       int32(expectedVersion),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if _, getErr := r.GetEmployeeByID(ctx, id); getErr != nil {
				return domain.Employee{}, getErr
			}
			return domain.Employee{}, apperror.VersionConflict("employee version conflict; refresh and retry")
		}
		return domain.Employee{}, db.MapPgError(err)
	}
	return mapEmployeeRow(row.ID, row.UserID, row.EmployeeCode, row.FullName, row.Phone, row.Email, row.PositionTitle, row.EmploymentStatus, row.ArchivedAt, row.CreatedAt, row.UpdatedAt, row.Version), nil
}

func (r *Repository) DeactivateEmployee(ctx context.Context, id uuid.UUID, now time.Time) (domain.Employee, error) {
	row, err := r.queries(ctx).DeactivateEmployee(ctx, sqlcgen.DeactivateEmployeeParams{
		ID:        id,
		UpdatedAt: now.UTC(),
	})
	if err != nil {
		return domain.Employee{}, db.MapPgError(err)
	}
	return mapEmployeeRow(row.ID, row.UserID, row.EmployeeCode, row.FullName, row.Phone, row.Email, row.PositionTitle, row.EmploymentStatus, row.ArchivedAt, row.CreatedAt, row.UpdatedAt, row.Version), nil
}

func (r *Repository) CreateCustomer(ctx context.Context, cust domain.Customer) (domain.Customer, error) {
	row, err := r.queries(ctx).CreateCustomer(ctx, sqlcgen.CreateCustomerParams{
		ID:              cust.ID,
		UserID:          cust.UserID,
		FullName:        strings.TrimSpace(cust.FullName),
		Phone:           strings.TrimSpace(cust.Phone),
		Email:           optionalText(cust.Email),
		NationalityCode: optionalText(cust.NationalityCode),
		GovernmentID:    optionalText(cust.GovernmentID),
		Address:         optionalText(cust.Address),
		Source:          cust.Source,
		CreatedAt:       cust.CreatedAt.UTC(),
	})
	if err != nil {
		if db.IsUniqueViolation(err) {
			return domain.Customer{}, apperror.Conflict("customer government_id or user_id already exists").WithCause(err)
		}
		return domain.Customer{}, db.MapPgError(err)
	}
	return mapCustomerRow(row.ID, row.UserID, row.FullName, row.Phone, row.Email, row.NationalityCode, row.GovernmentID, row.Address, row.Source, row.ArchivedAt, row.CreatedAt, row.UpdatedAt, row.Version), nil
}

func (r *Repository) GetCustomerByID(ctx context.Context, id uuid.UUID) (domain.Customer, error) {
	row, err := r.queries(ctx).GetCustomerByID(ctx, id)
	if err != nil {
		return domain.Customer{}, db.MapPgError(err)
	}
	return mapCustomerRow(row.ID, row.UserID, row.FullName, row.Phone, row.Email, row.NationalityCode, row.GovernmentID, row.Address, row.Source, row.ArchivedAt, row.CreatedAt, row.UpdatedAt, row.Version), nil
}

func (r *Repository) GetCustomerByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.Customer, error) {
	row, err := r.queries(ctx).GetCustomerByIDForUpdate(ctx, id)
	if err != nil {
		return domain.Customer{}, db.MapPgError(err)
	}
	return mapCustomerRow(row.ID, row.UserID, row.FullName, row.Phone, row.Email, row.NationalityCode, row.GovernmentID, row.Address, row.Source, row.ArchivedAt, row.CreatedAt, row.UpdatedAt, row.Version), nil
}

func (r *Repository) GetCustomerByUserID(ctx context.Context, userID uuid.UUID) (domain.Customer, error) {
	row, err := r.queries(ctx).GetCustomerByUserID(ctx, &userID)
	if err != nil {
		return domain.Customer{}, db.MapPgError(err)
	}
	return mapCustomerRow(row.ID, row.UserID, row.FullName, row.Phone, row.Email, row.NationalityCode, row.GovernmentID, row.Address, row.Source, row.ArchivedAt, row.CreatedAt, row.UpdatedAt, row.Version), nil
}

// CustomerListCursor represents a keyset pagination cursor (created_at, id).
type CustomerListCursor struct {
	HasCursor bool
	CreatedAt time.Time
	ID        uuid.UUID
}

func (r *Repository) ListCustomers(ctx context.Context, search string, cursor CustomerListCursor, limit int) ([]domain.Customer, error) {
	if limit <= 0 {
		limit = 20
	}
	cursorTS := cursor.CreatedAt.UTC()
	if !cursor.HasCursor {
		cursorTS = time.Now().UTC()
	}
	rows, err := r.queries(ctx).ListCustomers(ctx, sqlcgen.ListCustomersParams{
		Column1: strings.TrimSpace(search),
		Column2: cursor.HasCursor,
		Column3: cursorTS,
		Column4: cursor.ID,
		Limit:   int32(limit),
	})
	if err != nil {
		return nil, db.MapPgError(err)
	}
	out := make([]domain.Customer, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapCustomerRow(row.ID, row.UserID, row.FullName, row.Phone, row.Email, row.NationalityCode, row.GovernmentID, row.Address, row.Source, row.ArchivedAt, row.CreatedAt, row.UpdatedAt, row.Version))
	}
	return out, nil
}

func (r *Repository) UpdateCustomerOptimistic(ctx context.Context, cust domain.Customer, expectedVersion int, now time.Time) (domain.Customer, error) {
	row, err := r.queries(ctx).UpdateCustomerOptimistic(ctx, sqlcgen.UpdateCustomerOptimisticParams{
		ID:              cust.ID,
		FullName:        strings.TrimSpace(cust.FullName),
		Phone:           strings.TrimSpace(cust.Phone),
		Email:           optionalText(cust.Email),
		NationalityCode: optionalText(cust.NationalityCode),
		GovernmentID:    optionalText(cust.GovernmentID),
		Address:         optionalText(cust.Address),
		UpdatedAt:       now.UTC(),
		Version:         int32(expectedVersion),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if _, getErr := r.GetCustomerByID(ctx, cust.ID); getErr != nil {
				return domain.Customer{}, getErr
			}
			return domain.Customer{}, apperror.VersionConflict("customer version conflict; refresh and retry")
		}
		if db.IsUniqueViolation(err) {
			return domain.Customer{}, apperror.Conflict("customer government_id already exists").WithCause(err)
		}
		return domain.Customer{}, db.MapPgError(err)
	}
	return mapCustomerRow(row.ID, row.UserID, row.FullName, row.Phone, row.Email, row.NationalityCode, row.GovernmentID, row.Address, row.Source, row.ArchivedAt, row.CreatedAt, row.UpdatedAt, row.Version), nil
}

func (r *Repository) ArchiveCustomer(ctx context.Context, id uuid.UUID, now time.Time) (domain.Customer, error) {
	ts := now.UTC()
	row, err := r.queries(ctx).ArchiveCustomer(ctx, sqlcgen.ArchiveCustomerParams{
		ID:         id,
		ArchivedAt: &ts,
	})
	if err != nil {
		return domain.Customer{}, db.MapPgError(err)
	}
	return mapCustomerRow(row.ID, row.UserID, row.FullName, row.Phone, row.Email, row.NationalityCode, row.GovernmentID, row.Address, row.Source, row.ArchivedAt, row.CreatedAt, row.UpdatedAt, row.Version), nil
}

func optionalText(val string) pgtype.Text {
	trimmed := strings.TrimSpace(val)
	if trimmed == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: trimmed, Valid: true}
}

func mapEmployeeRow(
	id uuid.UUID,
	userID uuid.UUID,
	code string,
	fullName string,
	phone string,
	email string,
	positionTitle string,
	employmentStatus string,
	archivedAt *time.Time,
	createdAt time.Time,
	updatedAt time.Time,
	version int32,
) domain.Employee {
	return domain.Employee{
		ID:               id,
		UserID:           userID,
		EmployeeCode:     code,
		FullName:         fullName,
		Phone:            phone,
		Email:            email,
		PositionTitle:    positionTitle,
		EmploymentStatus: employmentStatus,
		ArchivedAt:       archivedAt,
		CreatedAt:        createdAt,
		UpdatedAt:        updatedAt,
		Version:          int(version),
	}
}

func mapCustomerRow(
	id uuid.UUID,
	userID *uuid.UUID,
	fullName string,
	phone string,
	email string,
	nationality string,
	govID string,
	address string,
	source string,
	archivedAt *time.Time,
	createdAt time.Time,
	updatedAt time.Time,
	version int32,
) domain.Customer {
	return domain.Customer{
		ID:              id,
		UserID:          userID,
		FullName:        fullName,
		Phone:           phone,
		Email:           email,
		NationalityCode: nationality,
		GovernmentID:    govID,
		Address:         address,
		Source:          source,
		ArchivedAt:      archivedAt,
		CreatedAt:       createdAt,
		UpdatedAt:       updatedAt,
		Version:         int(version),
	}
}
