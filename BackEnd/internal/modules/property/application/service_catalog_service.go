package application

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	auditapp "github.com/trandinh0506/hotel-management/backend/internal/modules/audit/application"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/property/domain"
	propertypg "github.com/trandinh0506/hotel-management/backend/internal/modules/property/persistence/postgres"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/auth"
	platformhttp "github.com/trandinh0506/hotel-management/backend/internal/platform/http"
)

// ServiceCatalogReader provides authoritative VAT-exclusive service lookup for Phase 5 (stayops T5.2).
type ServiceCatalogReader interface {
	GetActiveService(ctx context.Context, id uuid.UUID) (ServiceCatalogItem, error)
	GetServiceByID(ctx context.Context, id uuid.UUID) (ServiceCatalogItem, error)
}

var _ ServiceCatalogReader = (*Service)(nil)

// ListServicesFilter defines query parameters for GET /api/v1/services.
type ListServicesFilter struct {
	Category string
	IsActive *bool
	Cursor   string
	Limit    int
}

// CreateServiceParams defines inputs for POST /api/v1/services.
type CreateServiceParams struct {
	Code        string
	Name        string
	Category    string
	UnitPrice   decimal.Decimal
	Currency    string
	Description string
	IsActive    *bool
}

// UpdateServiceParams defines inputs for PATCH /api/v1/services/{id}.
type UpdateServiceParams struct {
	ID              uuid.UUID
	Name            *string
	Category        *string
	UnitPrice       *decimal.Decimal
	Currency        *string
	Description     *string
	IsActive        *bool
	ExpectedVersion int
}

// GetActiveService returns a service catalog item if and only if it exists and is_active = true.
// Deactivated services cannot be added to new stay consumptions (returns 409 Conflict).
func (s *Service) GetActiveService(ctx context.Context, id uuid.UUID) (domain.ServiceCatalogItem, error) {
	item, err := s.repo.GetServiceCatalogItemByID(ctx, id)
	if err != nil {
		return domain.ServiceCatalogItem{}, err
	}
	if !item.IsActive {
		return domain.ServiceCatalogItem{}, apperror.Conflict("service is deactivated and cannot be added to a stay")
	}
	return item, nil
}

// GetServiceByID returns a service catalog item by ID regardless of active status.
func (s *Service) GetServiceByID(ctx context.Context, id uuid.UUID) (domain.ServiceCatalogItem, error) {
	return s.repo.GetServiceCatalogItemByID(ctx, id)
}

// ListServices lists ancillary hotel services (`service.read`) with category/active filters and cursor pagination.
func (s *Service) ListServices(ctx context.Context, actor auth.Principal, filter ListServicesFilter) (platformhttp.PageResponse[domain.ServiceCatalogItem], error) {
	if s.authz != nil {
		if err := s.authz.Authorize(ctx, actor, "service.read"); err != nil {
			return platformhttp.PageResponse[domain.ServiceCatalogItem]{}, err
		}
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	items, nextCursor, err := s.repo.ListServiceCatalogItems(ctx, propertypg.ServiceCatalogListQuery{
		Category: filter.Category,
		IsActive: filter.IsActive,
		Cursor:   filter.Cursor,
		Limit:    limit,
	})
	if err != nil {
		return platformhttp.PageResponse[domain.ServiceCatalogItem]{}, err
	}

	var curPtr *string
	if filter.Cursor != "" {
		c := filter.Cursor
		curPtr = &c
	}
	return platformhttp.PageResponse[domain.ServiceCatalogItem]{
		Data: items,
		Page: platformhttp.PageInfo{
			Limit:      limit,
			Cursor:     curPtr,
			NextCursor: nextCursor,
		},
	}, nil
}

// CreateService creates a new ancillary service item (`service.manage`).
func (s *Service) CreateService(ctx context.Context, actor auth.Principal, params CreateServiceParams) (domain.ServiceCatalogItem, error) {
	if s.authz != nil {
		if err := s.authz.Authorize(ctx, actor, "service.manage"); err != nil {
			return domain.ServiceCatalogItem{}, err
		}
	}

	isActive := true
	if params.IsActive != nil {
		isActive = *params.IsActive
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
	candidate := domain.ServiceCatalogItem{
		ID:          s.ids.NewUUID(),
		Code:        params.Code,
		Name:        params.Name,
		Category:    params.Category,
		UnitPrice:   params.UnitPrice,
		Currency:    currency,
		Description: params.Description,
		IsActive:    isActive,
		CreatedAt:   now,
	}
	if err := candidate.NormalizeAndValidate(false, 0); err != nil {
		return domain.ServiceCatalogItem{}, err
	}

	var created domain.ServiceCatalogItem
	err := s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		item, err := s.repo.CreateServiceCatalogItem(txCtx, candidate)
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
				Action:        "property.service_created",
				ResourceType:  "service_catalog",
				ResourceID:    &item.ID,
				RequestID:     platformhttp.RequestIDUUIDFromContext(txCtx),
				CorrelationID: platformhttp.CorrelationIDUUIDFromContext(txCtx),
				AfterData:     serviceAuditMap(item),
			}); err != nil {
				return err
			}
		}
		created = item
		return nil
	})
	return created, err
}

// UpdateService updates an ancillary service item with optimistic locking (`service.manage`).
func (s *Service) UpdateService(ctx context.Context, actor auth.Principal, params UpdateServiceParams) (domain.ServiceCatalogItem, error) {
	if s.authz != nil {
		if err := s.authz.Authorize(ctx, actor, "service.manage"); err != nil {
			return domain.ServiceCatalogItem{}, err
		}
	}
	if params.ExpectedVersion < 1 {
		return domain.ServiceCatalogItem{}, apperror.Validation("invalid version", apperror.FieldError{
			Field:   "version",
			Message: "version must be a positive integer",
		})
	}

	now := s.clk.Now()
	var updated domain.ServiceCatalogItem

	err := s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		before, err := s.repo.GetServiceCatalogItemByIDForUpdate(txCtx, params.ID)
		if err != nil {
			return err
		}
		if before.Version != params.ExpectedVersion {
			return apperror.VersionConflict("service catalog version conflict")
		}

		candidate := before
		if params.Name != nil {
			candidate.Name = *params.Name
		}
		if params.Category != nil {
			candidate.Category = *params.Category
		}
		if params.UnitPrice != nil {
			candidate.UnitPrice = *params.UnitPrice
		}
		if params.Currency != nil {
			candidate.Currency = *params.Currency
		}
		if params.Description != nil {
			candidate.Description = *params.Description
		}
		if params.IsActive != nil {
			candidate.IsActive = *params.IsActive
		}

		if err := candidate.NormalizeAndValidate(true, params.ExpectedVersion); err != nil {
			return err
		}

		after, err := s.repo.UpdateServiceCatalogItem(txCtx, candidate, params.ExpectedVersion, now)
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
				Action:        "property.service_updated",
				ResourceType:  "service_catalog",
				ResourceID:    &after.ID,
				RequestID:     platformhttp.RequestIDUUIDFromContext(txCtx),
				CorrelationID: platformhttp.CorrelationIDUUIDFromContext(txCtx),
				BeforeData:    serviceAuditMap(before),
				AfterData:     serviceAuditMap(after),
			}); err != nil {
				return err
			}
		}
		updated = after
		return nil
	})
	return updated, err
}

// DeactivateService sets is_active = false on a service catalog item (`service.manage`) without deleting historical records.
func (s *Service) DeactivateService(ctx context.Context, actor auth.Principal, serviceID uuid.UUID, expectedVersion *int) (domain.ServiceCatalogItem, error) {
	if s.authz != nil {
		if err := s.authz.Authorize(ctx, actor, "service.manage"); err != nil {
			return domain.ServiceCatalogItem{}, err
		}
	}

	now := s.clk.Now()
	var deactivated domain.ServiceCatalogItem

	err := s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		before, err := s.repo.GetServiceCatalogItemByIDForUpdate(txCtx, serviceID)
		if err != nil {
			return err
		}
		if expectedVersion != nil && before.Version != *expectedVersion {
			return apperror.VersionConflict("service catalog version conflict")
		}

		after, err := s.repo.DeactivateServiceCatalogItem(txCtx, serviceID, now)
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
				Action:        "property.service_deactivated",
				ResourceType:  "service_catalog",
				ResourceID:    &after.ID,
				RequestID:     platformhttp.RequestIDUUIDFromContext(txCtx),
				CorrelationID: platformhttp.CorrelationIDUUIDFromContext(txCtx),
				BeforeData:    serviceAuditMap(before),
				AfterData:     serviceAuditMap(after),
			}); err != nil {
				return err
			}
		}
		deactivated = after
		return nil
	})
	return deactivated, err
}

func serviceAuditMap(item domain.ServiceCatalogItem) map[string]any {
	return map[string]any{
		"code":        item.Code,
		"name":        item.Name,
		"category":    item.Category,
		"unit_price":  item.UnitPrice.String(),
		"currency":    item.Currency,
		"description": item.Description,
		"is_active":   item.IsActive,
		"version":     item.Version,
	}
}
