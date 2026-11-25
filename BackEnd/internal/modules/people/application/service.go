package application

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	auditapp "github.com/trandinh0506/hotel-management/backend/internal/modules/audit/application"
	identityapp "github.com/trandinh0506/hotel-management/backend/internal/modules/identity/application"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/people/domain"
	peoplepg "github.com/trandinh0506/hotel-management/backend/internal/modules/people/persistence/postgres"
	rbacapp "github.com/trandinh0506/hotel-management/backend/internal/modules/rbac/application"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/auth"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/clock"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/db"
	platformhttp "github.com/trandinh0506/hotel-management/backend/internal/platform/http"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/id"
)

// Exported application-level aliases for cross-module callers.
type (
	Employee = domain.Employee
	Customer = domain.Customer
)

const (
	EmploymentStatusActive   = domain.EmploymentStatusActive
	EmploymentStatusInactive = domain.EmploymentStatusInactive

	CustomerSourceOnline = domain.CustomerSourceOnline
	CustomerSourceWalkIn = domain.CustomerSourceWalkIn
)

// CustomerReader provides read-only customer profile queries for Booking/Wallet/StayOps modules.
type CustomerReader interface {
	GetCustomerByID(ctx context.Context, id uuid.UUID) (domain.Customer, error)
	GetCustomerByUserID(ctx context.Context, userID uuid.UUID) (domain.Customer, error)
}

// Repository defines the persistence contract for employees and customers.
type Repository interface {
	CreateEmployee(ctx context.Context, emp domain.Employee) (domain.Employee, error)
	GetEmployeeByID(ctx context.Context, id uuid.UUID) (domain.Employee, error)
	GetEmployeeByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.Employee, error)
	GetEmployeeByUserID(ctx context.Context, userID uuid.UUID) (domain.Employee, error)
	ListEmployees(ctx context.Context, limit int) ([]domain.Employee, error)
	UpdateEmployeeOptimistic(ctx context.Context, id uuid.UUID, fullName, phone, positionTitle string, expectedVersion int, now time.Time) (domain.Employee, error)
	DeactivateEmployee(ctx context.Context, id uuid.UUID, now time.Time) (domain.Employee, error)

	CreateCustomer(ctx context.Context, cust domain.Customer) (domain.Customer, error)
	GetCustomerByID(ctx context.Context, id uuid.UUID) (domain.Customer, error)
	GetCustomerByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.Customer, error)
	GetCustomerByUserID(ctx context.Context, userID uuid.UUID) (domain.Customer, error)
	ListCustomers(ctx context.Context, search string, cursor peoplepg.CustomerListCursor, limit int) ([]domain.Customer, error)
	UpdateCustomerOptimistic(ctx context.Context, cust domain.Customer, expectedVersion int, now time.Time) (domain.Customer, error)
	ArchiveCustomer(ctx context.Context, id uuid.UUID, now time.Time) (domain.Customer, error)
}

// IdentityPort defines the cross-module identity contract consumed by People.
type IdentityPort interface {
	CreatePendingStaffUser(ctx context.Context, params identityapp.CreatePendingStaffParams) (identityapp.PendingStaffProvision, error)
	ReissueActivationToken(ctx context.Context, userID uuid.UUID) (identityapp.PendingStaffProvision, error)
	DeactivateUser(ctx context.Context, userID uuid.UUID) (identityapp.User, error)
	UpdateDisplayName(ctx context.Context, userID uuid.UUID, displayName string) (identityapp.User, error)
	GetUserByID(ctx context.Context, userID uuid.UUID) (identityapp.User, error)
}

// RBACPort defines the cross-module RBAC contract consumed by People.
type RBACPort interface {
	rbacapp.Authorizer
	AssignInitialRoles(ctx context.Context, actor auth.Principal, targetUserID uuid.UUID, roleCodes []string, roleIDs []uuid.UUID) ([]rbacapp.Role, error)
	EnsureNotLastActiveRootAdmin(ctx context.Context, targetUserID uuid.UUID) error
	ListUserRoles(ctx context.Context, userID uuid.UUID) ([]rbacapp.Role, error)
}

// CreateEmployeeParams defines inputs for creating a new employee and provisioning their pending staff account.
// Note: Admin never supplies a password; the employee sets their password via the single-use SMTP activation link.
type CreateEmployeeParams struct {
	EmployeeCode  string
	FullName      string
	Phone         string
	Email         string
	PositionTitle string
	Username      string
	RoleCodes     []string
	RoleIDs       []uuid.UUID
}

// UpdateEmployeeParams defines inputs for updating an employee with optimistic locking (version).
type UpdateEmployeeParams struct {
	ID              uuid.UUID
	FullName        string
	Phone           string
	PositionTitle   string
	ExpectedVersion int
}

type employeeActivationOutboxPayload struct {
	UserID         uuid.UUID `json:"userId"`
	EmployeeID     uuid.UUID `json:"employeeId"`
	EmployeeCode   string    `json:"employeeCode"`
	Email          string    `json:"email"`
	FullName       string    `json:"fullName"`
	PositionTitle  string    `json:"positionTitle"`
	EncryptedToken string    `json:"encryptedToken"`
	ExpiresAt      time.Time `json:"expiresAt"`
}

// Service implements employee and customer lifecycle use cases.
type Service struct {
	txRunner db.TxRunner
	repo     Repository
	identity IdentityPort
	rbac     RBACPort
	audit    auditapp.Recorder
	outbox   auditapp.OutboxEnqueuer
	clk      clock.Clock
	ids      id.Generator
}

// Compile-time check that Service implements CustomerReader.
var _ CustomerReader = (*Service)(nil)

// NewService constructs a People application service.
func NewService(
	txRunner db.TxRunner,
	repo Repository,
	identity IdentityPort,
	rbac RBACPort,
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
		txRunner: txRunner,
		repo:     repo,
		identity: identity,
		rbac:     rbac,
		audit:    audit,
		outbox:   outbox,
		clk:      clk,
		ids:      ids,
	}
}

// CreateEmployee atomically creates a pending_activation staff user, issues an AES-256-GCM encrypted activation token,
// assigns initial RBAC roles (subject to caller grantable-set rules), inserts the employee profile,
// enqueues identity.employee_activation_requested in outbox_events, and records an audit event.
func (s *Service) CreateEmployee(ctx context.Context, actor auth.Principal, params CreateEmployeeParams) (domain.Employee, error) {
	if err := s.rbac.Authorize(ctx, actor, "employee.create"); err != nil {
		return domain.Employee{}, err
	}
	code := strings.TrimSpace(params.EmployeeCode)
	fullName := strings.TrimSpace(params.FullName)
	phone := strings.TrimSpace(params.Phone)
	email := strings.ToLower(strings.TrimSpace(params.Email))
	position := strings.TrimSpace(params.PositionTitle)

	var fields []apperror.FieldError
	if code == "" {
		fields = append(fields, apperror.FieldError{Field: "employeeCode", Message: "employeeCode is required"})
	}
	if fullName == "" {
		fields = append(fields, apperror.FieldError{Field: "fullName", Message: "fullName is required"})
	}
	if phone == "" {
		fields = append(fields, apperror.FieldError{Field: "phone", Message: "phone is required"})
	}
	if email == "" {
		fields = append(fields, apperror.FieldError{Field: "email", Message: "email is required"})
	}
	if position == "" {
		fields = append(fields, apperror.FieldError{Field: "positionTitle", Message: "positionTitle is required"})
	}
	if len(fields) > 0 {
		return domain.Employee{}, apperror.Validation("invalid employee payload", fields...)
	}

	now := s.clk.Now()
	var created domain.Employee

	err := s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		prov, err := s.identity.CreatePendingStaffUser(txCtx, identityapp.CreatePendingStaffParams{
			Email:       email,
			DisplayName: fullName,
			Username:    params.Username,
		})
		if err != nil {
			return err
		}

		assignedRoles, err := s.rbac.AssignInitialRoles(txCtx, actor, prov.User.ID, params.RoleCodes, params.RoleIDs)
		if err != nil {
			return err
		}

		emp, err := s.repo.CreateEmployee(txCtx, domain.Employee{
			ID:               s.ids.NewUUID(),
			UserID:           prov.User.ID,
			EmployeeCode:     code,
			FullName:         fullName,
			Phone:            phone,
			Email:            prov.User.Email,
			PositionTitle:    position,
			EmploymentStatus: domain.EmploymentStatusActive,
			CreatedAt:        now,
		})
		if err != nil {
			return err
		}

		roleCodes := make([]string, 0, len(assignedRoles))
		for _, r := range assignedRoles {
			roleCodes = append(roleCodes, r.Code)
		}
		emp.AccountStatus = prov.User.Status
		emp.Roles = roleCodes

		payloadBytes, err := json.Marshal(employeeActivationOutboxPayload{
			UserID:         prov.User.ID,
			EmployeeID:     emp.ID,
			EmployeeCode:   emp.EmployeeCode,
			Email:          emp.Email,
			FullName:       emp.FullName,
			PositionTitle:  emp.PositionTitle,
			EncryptedToken: prov.EncryptedToken,
			ExpiresAt:      prov.ExpiresAt,
		})
		if err != nil {
			return apperror.Internal(err)
		}

		if err := s.outbox.Enqueue(txCtx, auditapp.OutboxEvent{
			ID:            s.ids.NewUUID(),
			AggregateType: "employee",
			AggregateID:   emp.ID,
			EventType:     identityapp.EventEmployeeActivationRequested,
			Payload:       payloadBytes,
			OccurredAt:    now,
			AvailableAt:   now,
		}); err != nil {
			return err
		}

		if s.audit != nil {
			if err := s.audit.Record(txCtx, auditapp.AuditEntry{
				ActorUserID:   &actor.UserID,
				Action:        "people.employee_created",
				ResourceType:  "employee",
				ResourceID:    &emp.ID,
				RequestID:     platformhttp.RequestIDUUIDFromContext(txCtx),
				CorrelationID: platformhttp.CorrelationIDUUIDFromContext(txCtx),
				AfterData: map[string]any{
					"employee_code":  emp.EmployeeCode,
					"user_id":        emp.UserID.String(),
					"email":          emp.Email,
					"full_name":      emp.FullName,
					"position_title": emp.PositionTitle,
					"account_status": emp.AccountStatus,
					"roles":          roleCodes,
				},
			}); err != nil {
				return err
			}
		}

		created = emp
		return nil
	})
	return created, err
}

// ResendEmployeeActivation invalidates prior activation tokens for a pending_activation employee
// and enqueues a new activation email in outbox_events.
func (s *Service) ResendEmployeeActivation(ctx context.Context, actor auth.Principal, employeeID uuid.UUID) error {
	if err := s.rbac.Authorize(ctx, actor, "employee.update"); err != nil {
		return err
	}
	now := s.clk.Now()
	return s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		emp, err := s.repo.GetEmployeeByIDForUpdate(txCtx, employeeID)
		if err != nil {
			return err
		}
		if emp.EmploymentStatus != domain.EmploymentStatusActive {
			return apperror.Conflict("cannot resend activation for an inactive employee")
		}

		prov, err := s.identity.ReissueActivationToken(txCtx, emp.UserID)
		if err != nil {
			return err
		}

		payloadBytes, err := json.Marshal(employeeActivationOutboxPayload{
			UserID:         emp.UserID,
			EmployeeID:     emp.ID,
			EmployeeCode:   emp.EmployeeCode,
			Email:          emp.Email,
			FullName:       emp.FullName,
			PositionTitle:  emp.PositionTitle,
			EncryptedToken: prov.EncryptedToken,
			ExpiresAt:      prov.ExpiresAt,
		})
		if err != nil {
			return apperror.Internal(err)
		}

		if err := s.outbox.Enqueue(txCtx, auditapp.OutboxEvent{
			ID:            s.ids.NewUUID(),
			AggregateType: "employee",
			AggregateID:   emp.ID,
			EventType:     identityapp.EventEmployeeActivationRequested,
			Payload:       payloadBytes,
			OccurredAt:    now,
			AvailableAt:   now,
		}); err != nil {
			return err
		}

		if s.audit != nil {
			if err := s.audit.Record(txCtx, auditapp.AuditEntry{
				ActorUserID:   &actor.UserID,
				Action:        "people.employee_activation_resent",
				ResourceType:  "employee",
				ResourceID:    &emp.ID,
				RequestID:     platformhttp.RequestIDUUIDFromContext(txCtx),
				CorrelationID: platformhttp.CorrelationIDUUIDFromContext(txCtx),
				AfterData: map[string]any{
					"user_id":    emp.UserID.String(),
					"expires_at": prov.ExpiresAt.UTC().Format(time.RFC3339),
				},
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

// ListEmployees returns active employees enriched with their account status and assigned role codes.
func (s *Service) ListEmployees(ctx context.Context, actor auth.Principal, limit int) ([]domain.Employee, error) {
	if err := s.rbac.Authorize(ctx, actor, "employee.read"); err != nil {
		return nil, err
	}
	emps, err := s.repo.ListEmployees(ctx, limit)
	if err != nil {
		return nil, err
	}
	for i := range emps {
		if err := s.enrichEmployee(ctx, &emps[i]); err != nil {
			return nil, err
		}
	}
	return emps, nil
}

// GetEmployee returns a single employee by ID enriched with account status and assigned roles.
func (s *Service) GetEmployee(ctx context.Context, actor auth.Principal, employeeID uuid.UUID) (domain.Employee, error) {
	if err := s.rbac.Authorize(ctx, actor, "employee.read"); err != nil {
		return domain.Employee{}, err
	}
	emp, err := s.repo.GetEmployeeByID(ctx, employeeID)
	if err != nil {
		return domain.Employee{}, err
	}
	if err := s.enrichEmployee(ctx, &emp); err != nil {
		return domain.Employee{}, err
	}
	return emp, nil
}

// UpdateEmployee updates an employee's fullName, phone, and positionTitle with optimistic locking (version).
func (s *Service) UpdateEmployee(ctx context.Context, actor auth.Principal, params UpdateEmployeeParams) (domain.Employee, error) {
	if err := s.rbac.Authorize(ctx, actor, "employee.update"); err != nil {
		return domain.Employee{}, err
	}
	if params.ExpectedVersion < 1 {
		return domain.Employee{}, apperror.Validation("version is required for optimistic locking", apperror.FieldError{
			Field:   "version",
			Message: "must be >= 1",
		})
	}

	now := s.clk.Now()
	var updated domain.Employee
	err := s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		before, err := s.repo.GetEmployeeByIDForUpdate(txCtx, params.ID)
		if err != nil {
			return err
		}
		fullName := strings.TrimSpace(params.FullName)
		if fullName == "" {
			fullName = before.FullName
		}
		phone := strings.TrimSpace(params.Phone)
		if phone == "" {
			phone = before.Phone
		}
		position := strings.TrimSpace(params.PositionTitle)
		if position == "" {
			position = before.PositionTitle
		}

		after, err := s.repo.UpdateEmployeeOptimistic(txCtx, params.ID, fullName, phone, position, params.ExpectedVersion, now)
		if err != nil {
			return err
		}
		if fullName != before.FullName {
			if _, err := s.identity.UpdateDisplayName(txCtx, after.UserID, fullName); err != nil {
				return err
			}
		}

		if s.audit != nil {
			if err := s.audit.Record(txCtx, auditapp.AuditEntry{
				ActorUserID:   &actor.UserID,
				Action:        "people.employee_updated",
				ResourceType:  "employee",
				ResourceID:    &after.ID,
				RequestID:     platformhttp.RequestIDUUIDFromContext(txCtx),
				CorrelationID: platformhttp.CorrelationIDUUIDFromContext(txCtx),
				BeforeData: map[string]any{
					"full_name":      before.FullName,
					"phone":          before.Phone,
					"position_title": before.PositionTitle,
					"version":        before.Version,
				},
				AfterData: map[string]any{
					"full_name":      after.FullName,
					"phone":          after.Phone,
					"position_title": after.PositionTitle,
					"version":        after.Version,
				},
			}); err != nil {
				return err
			}
		}

		if err := s.enrichEmployee(txCtx, &after); err != nil {
			return err
		}
		updated = after
		return nil
	})
	return updated, err
}

// DeactivateEmployee atomically sets employees.employment_status='inactive', sets users.status='disabled',
// revokes all active sessions, and blocks deactivating the last active Root Admin.
func (s *Service) DeactivateEmployee(ctx context.Context, actor auth.Principal, employeeID uuid.UUID, reason string) (domain.Employee, error) {
	if err := s.rbac.Authorize(ctx, actor, "employee.deactivate"); err != nil {
		return domain.Employee{}, err
	}
	now := s.clk.Now()
	var deactivated domain.Employee

	err := s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		before, err := s.repo.GetEmployeeByIDForUpdate(txCtx, employeeID)
		if err != nil {
			return err
		}
		if err := s.rbac.EnsureNotLastActiveRootAdmin(txCtx, before.UserID); err != nil {
			return err
		}
		after, err := s.repo.DeactivateEmployee(txCtx, before.ID, now)
		if err != nil {
			return err
		}
		if _, err := s.identity.DeactivateUser(txCtx, before.UserID); err != nil {
			return err
		}
		if s.audit != nil {
			if err := s.audit.Record(txCtx, auditapp.AuditEntry{
				ActorUserID:   &actor.UserID,
				Action:        "people.employee_deactivated",
				ResourceType:  "employee",
				ResourceID:    &after.ID,
				RequestID:     platformhttp.RequestIDUUIDFromContext(txCtx),
				CorrelationID: platformhttp.CorrelationIDUUIDFromContext(txCtx),
				Reason:        strings.TrimSpace(reason),
				BeforeData: map[string]any{
					"employment_status": before.EmploymentStatus,
				},
				AfterData: map[string]any{
					"employment_status": after.EmploymentStatus,
					"account_status":    identityapp.UserStatusDisabled,
				},
			}); err != nil {
				return err
			}
		}
		if err := s.enrichEmployee(txCtx, &after); err != nil {
			return err
		}
		deactivated = after
		return nil
	})
	return deactivated, err
}

func (s *Service) enrichEmployee(ctx context.Context, emp *domain.Employee) error {
	if emp == nil {
		return nil
	}
	if s.identity != nil {
		if u, err := s.identity.GetUserByID(ctx, emp.UserID); err == nil {
			emp.AccountStatus = u.Status
		}
	}
	if s.rbac != nil {
		if roles, err := s.rbac.ListUserRoles(ctx, emp.UserID); err == nil {
			codes := make([]string, 0, len(roles))
			for _, r := range roles {
				codes = append(codes, r.Code)
			}
			emp.Roles = codes
		}
	}
	return nil
}

// GetCustomerByID implements CustomerReader.
func (s *Service) GetCustomerByID(ctx context.Context, id uuid.UUID) (domain.Customer, error) {
	return s.repo.GetCustomerByID(ctx, id)
}

// GetCustomerByUserID implements CustomerReader.
func (s *Service) GetCustomerByUserID(ctx context.Context, userID uuid.UUID) (domain.Customer, error) {
	return s.repo.GetCustomerByUserID(ctx, userID)
}
