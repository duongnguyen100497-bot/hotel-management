package domain_test

import (
	"testing"

	"github.com/trandinh0506/hotel-management/backend/internal/modules/property/domain"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
)

func TestRoomStateTransitions_Unit(t *testing.T) {
	t.Parallel()

	// 1. Maintenance transitions
	t.Run("available -> maintenance requires reason", func(t *testing.T) {
		t.Parallel()
		_, _, err := domain.ValidateMaintenanceTransition(domain.RoomStatusAvailable, "enter", "   ")
		if appErr, ok := apperror.As(err); !ok || appErr.Code != apperror.CodeValidationFailed {
			t.Fatalf("expected validation_failed when reason is empty, got %v", err)
		}

		to, reason, err := domain.ValidateMaintenanceTransition(domain.RoomStatusAvailable, "enter", "AC repair")
		if err != nil || to != domain.RoomStatusMaintenance || reason != "AC repair" {
			t.Fatalf("unexpected result: to=%s reason=%s err=%v", to, reason, err)
		}
	})

	t.Run("occupied or cleaning -> maintenance rejected with conflict", func(t *testing.T) {
		t.Parallel()
		for _, st := range []string{domain.RoomStatusOccupied, domain.RoomStatusCleaning, domain.RoomStatusMaintenance} {
			_, _, err := domain.ValidateMaintenanceTransition(st, "enter", "plumbing")
			if appErr, ok := apperror.As(err); !ok || appErr.Code != apperror.CodeConflict {
				t.Fatalf("from %s expected conflict, got %v", st, err)
			}
		}
	})

	t.Run("maintenance -> available via exit", func(t *testing.T) {
		t.Parallel()
		to, reason, err := domain.ValidateMaintenanceTransition(domain.RoomStatusMaintenance, "exit", "")
		if err != nil || to != domain.RoomStatusAvailable || reason == "" {
			t.Fatalf("unexpected exit result: to=%s reason=%s err=%v", to, reason, err)
		}

		_, _, err = domain.ValidateMaintenanceTransition(domain.RoomStatusAvailable, "exit", "")
		if appErr, ok := apperror.As(err); !ok || appErr.Code != apperror.CodeConflict {
			t.Fatalf("exit from available expected conflict, got %v", err)
		}
	})

	// 2. Ready transitions
	t.Run("cleaning and maintenance -> available via ready; occupied and available rejected", func(t *testing.T) {
		t.Parallel()
		for _, validFrom := range []string{domain.RoomStatusCleaning, domain.RoomStatusMaintenance} {
			to, _, err := domain.ValidateReadyTransition(validFrom, "")
			if err != nil || to != domain.RoomStatusAvailable {
				t.Fatalf("ready from %s failed: to=%s err=%v", validFrom, to, err)
			}
		}
		for _, invalidFrom := range []string{domain.RoomStatusOccupied, domain.RoomStatusAvailable} {
			_, _, err := domain.ValidateReadyTransition(invalidFrom, "")
			if appErr, ok := apperror.As(err); !ok || appErr.Code != apperror.CodeConflict {
				t.Fatalf("ready from %s expected conflict, got %v", invalidFrom, err)
			}
		}
	})

	// 3. StayOps internal transitions
	t.Run("stayops available -> occupied -> cleaning", func(t *testing.T) {
		t.Parallel()
		if _, err := domain.ValidateStayOpsTransition(domain.RoomStatusAvailable, domain.RoomStatusOccupied, ""); err != nil {
			t.Fatalf("available -> occupied failed: %v", err)
		}
		if _, err := domain.ValidateStayOpsTransition(domain.RoomStatusOccupied, domain.RoomStatusCleaning, ""); err != nil {
			t.Fatalf("occupied -> cleaning failed: %v", err)
		}
		if _, err := domain.ValidateStayOpsTransition(domain.RoomStatusMaintenance, domain.RoomStatusOccupied, ""); err == nil {
			t.Fatal("expected error for maintenance -> occupied")
		}
		if _, err := domain.ValidateStayOpsTransition(domain.RoomStatusCleaning, domain.RoomStatusOccupied, ""); err == nil {
			t.Fatal("expected error for cleaning -> occupied")
		}
	})
}
