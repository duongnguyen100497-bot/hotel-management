package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
)

const (
	MaintenanceActionEnter = "enter"
	MaintenanceActionExit  = "exit"
)

// RoomStateHistoryEntry represents an immutable operational state transition record for a room.
type RoomStateHistoryEntry struct {
	ID          uuid.UUID  `json:"id"`
	RoomID      uuid.UUID  `json:"roomId"`
	FromStatus  string     `json:"fromStatus"`
	ToStatus    string     `json:"toStatus"`
	Reason      string     `json:"reason,omitempty"`
	ActorUserID *uuid.UUID `json:"actorUserId,omitempty"`
	OccurredAt  time.Time  `json:"occurredAt"`
}

// ValidateMaintenanceTransition validates a direct staff maintenance transition (`enter` or `exit`).
// Returns the target operational status and normalized reason, or an apperror (422 on invalid input, 409 on illegal state transition).
func ValidateMaintenanceTransition(currentStatus, action, reason string) (targetStatus string, normReason string, err error) {
	act := strings.ToLower(strings.TrimSpace(action))
	normReason = strings.TrimSpace(reason)

	switch act {
	case MaintenanceActionEnter:
		if normReason == "" {
			return "", "", apperror.Validation("reason is required when entering maintenance", apperror.FieldError{
				Field:   "reason",
				Message: "reason is required when putting a room into maintenance",
			})
		}
		if currentStatus != RoomStatusAvailable {
			return "", "", apperror.Conflict(fmt.Sprintf("cannot enter maintenance from %q state (room must be available)", currentStatus))
		}
		return RoomStatusMaintenance, normReason, nil

	case MaintenanceActionExit:
		if currentStatus != RoomStatusMaintenance {
			return "", "", apperror.Conflict(fmt.Sprintf("cannot exit maintenance when room is in %q state", currentStatus))
		}
		if normReason == "" {
			normReason = "maintenance completed"
		}
		return RoomStatusAvailable, normReason, nil

	default:
		return "", "", apperror.Validation("invalid maintenance action", apperror.FieldError{
			Field:   "action",
			Message: "must be 'enter' or 'exit'",
		})
	}
}

// ValidateReadyTransition validates transitioning a room from `cleaning` or `maintenance` to `available`.
func ValidateReadyTransition(currentStatus, reason string) (targetStatus string, normReason string, err error) {
	normReason = strings.TrimSpace(reason)
	switch currentStatus {
	case RoomStatusCleaning:
		if normReason == "" {
			normReason = "housekeeping completed"
		}
		return RoomStatusAvailable, normReason, nil
	case RoomStatusMaintenance:
		if normReason == "" {
			normReason = "maintenance completed"
		}
		return RoomStatusAvailable, normReason, nil
	case RoomStatusOccupied:
		return "", "", apperror.Conflict("cannot mark an occupied room as available before check-out")
	case RoomStatusAvailable:
		return "", "", apperror.Conflict("room is already available")
	default:
		return "", "", apperror.Conflict(fmt.Sprintf("cannot transition room from %q to available", currentStatus))
	}
}

// ValidateStayOpsTransition validates internal stayops-driven transitions (`available -> occupied` and `occupied -> cleaning`).
func ValidateStayOpsTransition(currentStatus, targetStatus, reason string) (string, error) {
	normReason := strings.TrimSpace(reason)
	switch {
	case currentStatus == RoomStatusAvailable && targetStatus == RoomStatusOccupied:
		if normReason == "" {
			normReason = "guest checked in"
		}
		return normReason, nil
	case currentStatus == RoomStatusOccupied && targetStatus == RoomStatusCleaning:
		if normReason == "" {
			normReason = "guest checked out"
		}
		return normReason, nil
	default:
		return "", apperror.Conflict(fmt.Sprintf("illegal stayops room transition from %q to %q", currentStatus, targetStatus))
	}
}
