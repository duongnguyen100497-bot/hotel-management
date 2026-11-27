package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	auditapp "github.com/trandinh0506/hotel-management/backend/internal/modules/audit/application"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/pricing/domain"
	propertyapp "github.com/trandinh0506/hotel-management/backend/internal/modules/property/application"
	rbacapp "github.com/trandinh0506/hotel-management/backend/internal/modules/rbac/application"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/auth"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/clock"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/db"
	platformhttp "github.com/trandinh0506/hotel-management/backend/internal/platform/http"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/id"
)

// Exported application-level aliases for cross-module callers (booking, stayops).
type (
	VATRate         = domain.VATRate
	PricingSnapshot = domain.PricingSnapshot
)

// QuoteCalculator computes VAT-exclusive pricing snapshots using exact decimal arithmetic.
type QuoteCalculator interface {
	Calculate(ctx context.Context, subtotal decimal.Decimal) (PricingSnapshot, error)
	CalculateWithRate(subtotal decimal.Decimal, activeVAT *VATRate, currency string) (PricingSnapshot, error)
}

// VATReader provides read access to the currently active VAT rate.
type VATReader interface {
	GetActiveVATRate(ctx context.Context) (*VATRate, error)
}

// Repository defines the persistence contract for VAT rates.
type Repository interface {
	ListVATRates(ctx context.Context) ([]domain.VATRate, error)
	GetVATRateByID(ctx context.Context, id uuid.UUID) (domain.VATRate, error)
	GetVATRateByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.VATRate, error)
	GetActiveVATRate(ctx context.Context) (*domain.VATRate, error)
	CreateVATRate(ctx context.Context, v domain.VATRate) (domain.VATRate, error)
	DeactivateOtherVATRates(ctx context.Context, excludeID uuid.UUID, now time.Time) ([]domain.VATRate, error)
	UpdateVATRate(ctx context.Context, id uuid.UUID, name string, rate decimal.Decimal, isActive bool, expectedVersion int, now time.Time) (domain.VATRate, error)
	DeactivateVATRate(ctx context.Context, id uuid.UUID, now time.Time) (domain.VATRate, error)
}

// CreateVATRateParams defines inputs for POST /api/v1/vat-rates.
type CreateVATRateParams struct {
	Code     string
	Name     string
	Rate     decimal.Decimal
	IsActive bool
}

// UpdateVATRateParams defines inputs for PATCH /api/v1/vat-rates/{id}.
type UpdateVATRateParams struct {
	ID              uuid.UUID
	Name            *string
	Rate            *decimal.Decimal
	IsActive        *bool
	ExpectedVersion int
}

// Service implements VAT rate management and QuoteCalculator.
type Service struct {
	txRunner db.TxRunner
	repo     Repository
	settings propertyapp.SettingsReader
	authz    rbacapp.Authorizer
	audit    auditapp.Recorder
	clk      clock.Clock
	ids      id.Generator
}

var (
	_ QuoteCalculator = (*Service)(nil)
	_ VATReader       = (*Service)(nil)
)

// NewService constructs a pricing application service.
func NewService(
	txRunner db.TxRunner,
	repo Repository,
	settings propertyapp.SettingsReader,
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
		settings: settings,
		authz:    authz,
		audit:    audit,
		clk:      clk,
		ids:      ids,
	}
}

// Calculate computes a PricingSnapshot using the currently active VAT rate and hotel currency from PostgreSQL.
func (s *Service) Calculate(ctx context.Context, subtotal decimal.Decimal) (domain.PricingSnapshot, error) {
	currency := domain.DefaultCurrency
	if s.settings != nil {
		cfg, err := s.settings.GetSettings(ctx)
		if err != nil {
			return domain.PricingSnapshot{}, err
		}
		if cfg.Currency != "" {
			currency = cfg.Currency
		}
	}

	activeVAT, err := s.repo.GetActiveVATRate(ctx)
	if err != nil {
		return domain.PricingSnapshot{}, err
	}
	return domain.CalculateQuote(subtotal, activeVAT, currency)
}

// CalculateWithRate computes a PricingSnapshot directly from the supplied subtotal, optional VATRate, and currency.
func (s *Service) CalculateWithRate(subtotal decimal.Decimal, activeVAT *domain.VATRate, currency string) (domain.PricingSnapshot, error) {
	return domain.CalculateQuote(subtotal, activeVAT, currency)
}

// GetActiveVATRate returns the currently active VAT rate, or nil if no VAT rate is active.
func (s *Service) GetActiveVATRate(ctx context.Context) (*domain.VATRate, error) {
	return s.repo.GetActiveVATRate(ctx)
}

// ListVATRates lists all VAT rates (active first, then chronological).
func (s *Service) ListVATRates(ctx context.Context) ([]domain.VATRate, error) {
	return s.repo.ListVATRates(ctx)
}

// CreateVATRate creates a new VAT rate. If params.IsActive is true, any previously active VAT rate
// is atomically deactivated within the same PostgreSQL transaction.
func (s *Service) CreateVATRate(ctx context.Context, actor auth.Principal, params CreateVATRateParams) (domain.VATRate, error) {
	if s.authz != nil {
		if err := s.authz.Authorize(ctx, actor, "vat.manage"); err != nil {
			return domain.VATRate{}, err
		}
	}

	code, name, rate, err := domain.NormalizeAndValidateCreate(params.Code, params.Name, params.Rate)
	if err != nil {
		return domain.VATRate{}, err
	}

	now := s.clk.Now()
	newID := s.ids.NewUUID()
	var created domain.VATRate

	err = s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		var deactivatedCodes []string
		if params.IsActive {
			prevActive, err := s.repo.DeactivateOtherVATRates(txCtx, newID, now)
			if err != nil {
				return err
			}
			for _, p := range prevActive {
				deactivatedCodes = append(deactivatedCodes, p.Code)
			}
		}

		v, err := s.repo.CreateVATRate(txCtx, domain.VATRate{
			ID:        newID,
			Code:      code,
			Name:      name,
			Rate:      rate,
			IsActive:  params.IsActive,
			CreatedAt: now,
		})
		if err != nil {
			return err
		}

		if s.audit != nil {
			var actorID *uuid.UUID
			if actor.UserID != uuid.Nil {
				actorID = &actor.UserID
			}
			afterData := vatAuditMap(v)
			if len(deactivatedCodes) > 0 {
				afterData["deactivated_codes"] = deactivatedCodes
			}
			if err := s.audit.Record(txCtx, auditapp.AuditEntry{
				ActorUserID:   actorID,
				Action:        "pricing.vat_rate_created",
				ResourceType:  "vat_rate",
				ResourceID:    &v.ID,
				RequestID:     platformhttp.RequestIDUUIDFromContext(txCtx),
				CorrelationID: platformhttp.CorrelationIDUUIDFromContext(txCtx),
				AfterData:     afterData,
			}); err != nil {
				return err
			}
		}

		created = v
		return nil
	})
	return created, err
}

// UpdateVATRate updates a VAT rate with optimistic locking. Activating a VAT rate atomically
// deactivates any previously active VAT rate in the same transaction.
func (s *Service) UpdateVATRate(ctx context.Context, actor auth.Principal, params UpdateVATRateParams) (domain.VATRate, error) {
	if s.authz != nil {
		if err := s.authz.Authorize(ctx, actor, "vat.manage"); err != nil {
			return domain.VATRate{}, err
		}
	}
	if params.ExpectedVersion < 1 {
		return domain.VATRate{}, apperror.Validation("invalid version", apperror.FieldError{
			Field:   "version",
			Message: "version must be a positive integer",
		})
	}

	now := s.clk.Now()
	var updated domain.VATRate

	err := s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		before, err := s.repo.GetVATRateByIDForUpdate(txCtx, params.ID)
		if err != nil {
			return err
		}
		if before.Version != params.ExpectedVersion {
			return apperror.VersionConflict("VAT rate version conflict")
		}

		nextName := before.Name
		if params.Name != nil {
			nextName = *params.Name
		}
		nextRate := before.Rate
		if params.Rate != nil {
			nextRate = *params.Rate
		}
		nextActive := before.IsActive
		if params.IsActive != nil {
			nextActive = *params.IsActive
		}

		validName, validRate, err := domain.ValidateUpdate(nextName, nextRate, params.ExpectedVersion)
		if err != nil {
			return err
		}

		var deactivatedCodes []string
		if nextActive && !before.IsActive {
			prevActive, err := s.repo.DeactivateOtherVATRates(txCtx, before.ID, now)
			if err != nil {
				return err
			}
			for _, p := range prevActive {
				deactivatedCodes = append(deactivatedCodes, p.Code)
			}
		}

		after, err := s.repo.UpdateVATRate(txCtx, before.ID, validName, validRate, nextActive, params.ExpectedVersion, now)
		if err != nil {
			return err
		}

		if s.audit != nil {
			var actorID *uuid.UUID
			if actor.UserID != uuid.Nil {
				actorID = &actor.UserID
			}
			afterData := vatAuditMap(after)
			if len(deactivatedCodes) > 0 {
				afterData["deactivated_codes"] = deactivatedCodes
			}
			if err := s.audit.Record(txCtx, auditapp.AuditEntry{
				ActorUserID:   actorID,
				Action:        "pricing.vat_rate_updated",
				ResourceType:  "vat_rate",
				ResourceID:    &after.ID,
				RequestID:     platformhttp.RequestIDUUIDFromContext(txCtx),
				CorrelationID: platformhttp.CorrelationIDUUIDFromContext(txCtx),
				BeforeData:    vatAuditMap(before),
				AfterData:     afterData,
			}); err != nil {
				return err
			}
		}

		updated = after
		return nil
	})
	return updated, err
}

// DeactivateVATRate sets is_active = false on a VAT rate (VAT rates are never hard-deleted).
func (s *Service) DeactivateVATRate(ctx context.Context, actor auth.Principal, vatID uuid.UUID, expectedVersion *int) (domain.VATRate, error) {
	if s.authz != nil {
		if err := s.authz.Authorize(ctx, actor, "vat.manage"); err != nil {
			return domain.VATRate{}, err
		}
	}

	now := s.clk.Now()
	var deactivated domain.VATRate

	err := s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		before, err := s.repo.GetVATRateByIDForUpdate(txCtx, vatID)
		if err != nil {
			return err
		}
		if expectedVersion != nil && before.Version != *expectedVersion {
			return apperror.VersionConflict("VAT rate version conflict")
		}

		after, err := s.repo.DeactivateVATRate(txCtx, vatID, now)
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
				Action:        "pricing.vat_rate_deactivated",
				ResourceType:  "vat_rate",
				ResourceID:    &after.ID,
				RequestID:     platformhttp.RequestIDUUIDFromContext(txCtx),
				CorrelationID: platformhttp.CorrelationIDUUIDFromContext(txCtx),
				BeforeData:    vatAuditMap(before),
				AfterData:     vatAuditMap(after),
			}); err != nil {
				return err
			}
		}

		deactivated = after
		return nil
	})
	return deactivated, err
}

func vatAuditMap(v domain.VATRate) map[string]any {
	return map[string]any{
		"code":      v.Code,
		"name":      v.Name,
		"rate":      v.Rate.StringFixed(domain.VATRateScale),
		"is_active": v.IsActive,
		"version":   v.Version,
	}
}
