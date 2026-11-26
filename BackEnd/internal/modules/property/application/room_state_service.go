package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	auditapp "github.com/trandinh0506/hotel-management/backend/internal/modules/audit/application"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/property/domain"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/auth"
	platformhttp "github.com/trandinh0506/hotel-management/backend/internal/platform/http"
)

// RoomStateHistoryEntry aliases domain.RoomStateHistoryEntry for cross-module callers.
type RoomStateHistoryEntry = domain.RoomStateHistoryEntry

// RoomStateService exposes controlled room operational state transitions for Phase 5 (stayops)
// and point-in-time operational room counts (excluding maintenance) for Phase 6 (reporting).
type RoomStateService interface {
	MarkOccupied(ctx context.Context, roomID uuid.UUID, actorUserID *uuid.UUID, reason string) (Room, error)
	MarkCleaning(ctx context.Context, roomID uuid.UUID, actorUserID *uuid.UUID, reason string) (Room, error)
	MarkReady(ctx context.Context, actor auth.Principal, roomID uuid.UUID, reason string, expectedVersion *int) (Room, error)
	CountOperationalRoomsAt(ctx context.Context, snapshotAt time.Time) (totalActiveRooms int, maintenanceRooms int, operationalRooms int, err error)
}

var _ RoomStateService = (*Service)(nil)

// TransitionMaintenanceParams defines inputs for POST /api/v1/rooms/{id}/maintenance.
type TransitionMaintenanceParams struct {
	RoomID          uuid.UUID
	Action          string
	Reason          string
	ExpectedVersion *int
}

// TransitionMaintenance moves a room into (`enter`) or out of (`exit`) `maintenance` (`room.state.update`),
// recording both room_state_history and audit_events atomically in a single transaction.
func (s *Service) TransitionMaintenance(ctx context.Context, actor auth.Principal, params TransitionMaintenanceParams) (domain.Room, error) {
	if s.authz != nil {
		if err := s.authz.Authorize(ctx, actor, "room.state.update"); err != nil {
			return domain.Room{}, err
		}
	}

	var actorID *uuid.UUID
	if actor.UserID != uuid.Nil {
		actorID = &actor.UserID
	}

	return s.executeRoomTransition(ctx, params.RoomID, params.ExpectedVersion, actorID, func(before domain.Room) (string, string, error) {
		return domain.ValidateMaintenanceTransition(before.OperationalStatus, params.Action, params.Reason)
	})
}

// MarkReady transitions a room from `cleaning` (or `maintenance`) to `available` (`room.state.update`).
func (s *Service) MarkReady(ctx context.Context, actor auth.Principal, roomID uuid.UUID, reason string, expectedVersion *int) (domain.Room, error) {
	if s.authz != nil {
		if err := s.authz.Authorize(ctx, actor, "room.state.update"); err != nil {
			return domain.Room{}, err
		}
	}

	var actorID *uuid.UUID
	if actor.UserID != uuid.Nil {
		actorID = &actor.UserID
	}

	return s.executeRoomTransition(ctx, roomID, expectedVersion, actorID, func(before domain.Room) (string, string, error) {
		return domain.ValidateReadyTransition(before.OperationalStatus, reason)
	})
}

// MarkOccupied transitions a room from `available` to `occupied` (invoked by stayops check-in within TX).
func (s *Service) MarkOccupied(ctx context.Context, roomID uuid.UUID, actorUserID *uuid.UUID, reason string) (domain.Room, error) {
	return s.executeRoomTransition(ctx, roomID, nil, actorUserID, func(before domain.Room) (string, string, error) {
		normReason, err := domain.ValidateStayOpsTransition(before.OperationalStatus, domain.RoomStatusOccupied, reason)
		if err != nil {
			return "", "", err
		}
		return domain.RoomStatusOccupied, normReason, nil
	})
}

// MarkCleaning transitions a room from `occupied` to `cleaning` (invoked by stayops check-out within TX).
func (s *Service) MarkCleaning(ctx context.Context, roomID uuid.UUID, actorUserID *uuid.UUID, reason string) (domain.Room, error) {
	return s.executeRoomTransition(ctx, roomID, nil, actorUserID, func(before domain.Room) (string, string, error) {
		normReason, err := domain.ValidateStayOpsTransition(before.OperationalStatus, domain.RoomStatusCleaning, reason)
		if err != nil {
			return "", "", err
		}
		return domain.RoomStatusCleaning, normReason, nil
	})
}

// ListRoomStateHistory returns the immutable operational state history for a room (`property.read`).
func (s *Service) ListRoomStateHistory(ctx context.Context, actor auth.Principal, roomID uuid.UUID, cursor string, limit int) (platformhttp.PageResponse[domain.RoomStateHistoryEntry], error) {
	if s.authz != nil {
		if err := s.authz.Authorize(ctx, actor, "property.read"); err != nil {
			return platformhttp.PageResponse[domain.RoomStateHistoryEntry]{}, err
		}
	}
	if _, err := s.repo.GetRoomByID(ctx, roomID); err != nil {
		return platformhttp.PageResponse[domain.RoomStateHistoryEntry]{}, err
	}
	if limit <= 0 {
		limit = 20
	}
	items, nextCursor, err := s.repo.ListRoomStateHistory(ctx, roomID, cursor, limit)
	if err != nil {
		return platformhttp.PageResponse[domain.RoomStateHistoryEntry]{}, err
	}

	var curPtr *string
	if cursor != "" {
		c := cursor
		curPtr = &c
	}
	return platformhttp.PageResponse[domain.RoomStateHistoryEntry]{
		Data: items,
		Page: platformhttp.PageInfo{
			Limit:      limit,
			Cursor:     curPtr,
			NextCursor: nextCursor,
		},
	}, nil
}

// CountOperationalRoomsAt returns total active rooms, maintenance rooms, and operational rooms at snapshotAt.
func (s *Service) CountOperationalRoomsAt(ctx context.Context, snapshotAt time.Time) (int, int, int, error) {
	return s.repo.CountOperationalRoomsAt(ctx, snapshotAt)
}

func (s *Service) executeRoomTransition(
	ctx context.Context,
	roomID uuid.UUID,
	expectedVersion *int,
	actorUserID *uuid.UUID,
	validateFn func(before domain.Room) (targetStatus string, normReason string, err error),
) (domain.Room, error) {
	now := s.clk.Now()
	var updated domain.Room

	err := s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		before, err := s.repo.GetRoomByIDForUpdate(txCtx, roomID)
		if err != nil {
			return err
		}
		if before.ArchivedAt != nil {
			return apperror.Conflict("cannot change operational state of an archived room")
		}
		if expectedVersion != nil && before.Version != *expectedVersion {
			return apperror.VersionConflict("room version conflict")
		}

		targetStatus, normReason, err := validateFn(before)
		if err != nil {
			return err
		}

		after, err := s.repo.UpdateRoomOperationalStatus(txCtx, before.ID, targetStatus, before.Version, now)
		if err != nil {
			return err
		}

		if _, err := s.repo.InsertRoomStateHistory(txCtx, domain.RoomStateHistoryEntry{
			ID:          s.ids.NewUUID(),
			RoomID:      before.ID,
			FromStatus:  before.OperationalStatus,
			ToStatus:    targetStatus,
			Reason:      normReason,
			ActorUserID: actorUserID,
			OccurredAt:  now,
		}); err != nil {
			return err
		}

		if s.audit != nil {
			if err := s.audit.Record(txCtx, auditapp.AuditEntry{
				ActorUserID:   actorUserID,
				Action:        "property.room_state_changed",
				ResourceType:  "room",
				ResourceID:    &after.ID,
				RequestID:     platformhttp.RequestIDUUIDFromContext(txCtx),
				CorrelationID: platformhttp.CorrelationIDUUIDFromContext(txCtx),
				Reason:        normReason,
				BeforeData: map[string]any{
					"code":               before.Code,
					"operational_status": before.OperationalStatus,
					"version":            before.Version,
				},
				AfterData: map[string]any{
					"code":               after.Code,
					"operational_status": after.OperationalStatus,
					"version":            after.Version,
				},
			}); err != nil {
				return err
			}
		}

		updated = after
		return nil
	})
	return updated, err
}
