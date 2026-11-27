package application

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	auditapp "github.com/trandinh0506/hotel-management/backend/internal/modules/audit/application"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/booking/domain"
	propertyapp "github.com/trandinh0506/hotel-management/backend/internal/modules/property/application"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/auth"
	platformhttp "github.com/trandinh0506/hotel-management/backend/internal/platform/http"
)

// CancelBookingParams defines inputs for POST /api/v1/bookings/{id}/cancel.
type CancelBookingParams struct {
	BookingID uuid.UUID
	Reason    string
}

// BatchCancelBookingsParams defines inputs for POST /api/v1/bookings/batch/cancel.
type BatchCancelBookingsParams struct {
	BookingIDs []uuid.UUID
	Reason     string
}

// CancelBooking cancels a single booking (pending or confirmed).
// Invariant: cancelling 1 child in a booking group NEVER cancels sibling rooms,
// and recomputes booking_groups.status atomically in the same transaction.
func (s *Service) CancelBooking(ctx context.Context, actor auth.Principal, params CancelBookingParams) (domain.Booking, error) {
	if params.BookingID == uuid.Nil {
		return domain.Booking{}, apperror.Validation("booking id is required", apperror.FieldError{
			Field:   "id",
			Message: "must be a valid UUID",
		})
	}
	cancelled, err := s.cancelBookingsAtomic(ctx, actor, []uuid.UUID{params.BookingID}, params.Reason)
	if err != nil {
		return domain.Booking{}, err
	}
	return cancelled[0], nil
}

// BatchCancelBookings cancels 1..50 bookings atomically (all-or-nothing) in sorted UUID lock order.
// If any booking is missing, unauthorized, or in an uncancellable status, the entire transaction rolls back.
func (s *Service) BatchCancelBookings(ctx context.Context, actor auth.Principal, params BatchCancelBookingsParams) ([]domain.Booking, error) {
	deduped := dedupeAndSortUUIDs(params.BookingIDs)
	if len(deduped) == 0 {
		return nil, apperror.Validation("bookingIds is required", apperror.FieldError{
			Field:   "bookingIds",
			Message: "provide between 1 and 50 valid booking UUIDs",
		})
	}
	if len(deduped) > domain.MaxBatchCancel {
		return nil, apperror.Validation("too many bookingIds", apperror.FieldError{
			Field:   "bookingIds",
			Message: fmt.Sprintf("at most %d booking IDs are allowed per batch", domain.MaxBatchCancel),
		})
	}
	return s.cancelBookingsAtomic(ctx, actor, deduped, params.Reason)
}

func (s *Service) cancelBookingsAtomic(ctx context.Context, actor auth.Principal, sortedIDs []uuid.UUID, reasonRaw string) ([]domain.Booking, error) {
	isStaff := actor.Kind == "staff"
	var actorCustomerID uuid.UUID
	if isStaff {
		if s.authz != nil {
			if err := s.authz.Authorize(ctx, actor, "booking.cancel"); err != nil {
				return nil, err
			}
		}
	} else {
		if actor.UserID == uuid.Nil {
			return nil, apperror.Unauthenticated("authentication required")
		}
		cust, err := s.customers.GetCustomerByUserID(ctx, actor.UserID)
		if err != nil {
			return nil, err
		}
		actorCustomerID = cust.ID
	}

	cfg, err := s.settings.GetSettings(ctx)
	if err != nil {
		return nil, err
	}

	reason := strings.TrimSpace(reasonRaw)
	if reason == "" {
		reason = "cancelled_by_user"
		if isStaff {
			reason = "cancelled_by_staff"
		}
	}
	var actorID *uuid.UUID
	if actor.UserID != uuid.Nil {
		uid := actor.UserID
		actorID = &uid
	}

	now := s.clk.Now()
	var (
		cancelledBookings []domain.Booking
		releaseBatches    []redisReleaseBatch
	)

	err = s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		// Phase 1: Lock ALL target bookings in ascending UUID order and validate every single one BEFORE mutating.
		locked := make([]domain.Booking, 0, len(sortedIDs))
		for _, bid := range sortedIDs {
			b, err := s.repo.GetBookingByIDForUpdate(txCtx, bid)
			if err != nil {
				return err
			}
			if !isStaff && b.CustomerID != actorCustomerID {
				return apperror.Forbidden(fmt.Sprintf("you do not own booking %s", bid.String()))
			}
			if b.Status != domain.BookingStatusPending && b.Status != domain.BookingStatusConfirmed {
				return apperror.Conflict(fmt.Sprintf("booking %s cannot be cancelled from status %s", bid.String(), b.Status))
			}
			locked = append(locked, b)
		}

		// Phase 2: All bookings validated — apply cancellations, history, hooks, audit, outbox, and group recompute.
		groupSet := make(map[uuid.UUID]struct{})
		for _, before := range locked {
			updated, err := s.repo.UpdateBookingStatusCancelled(txCtx, before.ID, reason, now)
			if err != nil {
				return err
			}

			fromSt := before.Status
			if err := s.repo.InsertBookingStatusHistory(txCtx, domain.BookingStatusHistoryEntry{
				ID:            s.ids.NewUUID(),
				BookingID:     before.ID,
				FromStatus:    &fromSt,
				ToStatus:      domain.BookingStatusCancelled,
				ActorUserID:   actorID,
				Reason:        reason,
				CorrelationID: platformhttp.CorrelationIDUUIDFromContext(txCtx),
				OccurredAt:    now,
			}); err != nil {
				return err
			}

			freeCancelEligible := isFreeCancelEligible(now, before.CheckInTime, cfg)
			if s.lifecycleHook != nil {
				if err := s.lifecycleHook.OnBookingCancelled(txCtx, before.ID, before.BookingGroupID, before.CustomerID, freeCancelEligible); err != nil {
					return err
				}
			}

			if s.audit != nil {
				idCopy := before.ID
				if err := s.audit.Record(txCtx, auditapp.AuditEntry{
					ActorUserID:   actorID,
					Action:        "booking.cancelled",
					ResourceType:  "booking",
					ResourceID:    &idCopy,
					RequestID:     platformhttp.RequestIDUUIDFromContext(txCtx),
					CorrelationID: platformhttp.CorrelationIDUUIDFromContext(txCtx),
					BeforeData:    map[string]any{"status": string(before.Status)},
					AfterData: map[string]any{
						"status":               string(updated.Status),
						"reason":               reason,
						"free_cancel_eligible": freeCancelEligible,
					},
				}); err != nil {
					return err
				}
			}

			if s.outbox != nil {
				payload, _ := json.Marshal(map[string]any{
					"bookingId":          updated.ID.String(),
					"referenceCode":      updated.ReferenceCode,
					"customerId":         updated.CustomerID.String(),
					"reason":             reason,
					"freeCancelEligible": freeCancelEligible,
				})
				if err := s.outbox.Enqueue(txCtx, auditapp.OutboxEvent{
					AggregateType: "booking",
					AggregateID:   updated.ID,
					EventType:     EventBookingCancelled,
					Payload:       payload,
				}); err != nil {
					return err
				}
			}

			if before.Status == domain.BookingStatusPending {
				if holds, err := s.repo.FindConvertedHoldsForRoomStay(txCtx, before.RoomID, before.CheckInTime, before.CheckOutTime); err == nil {
					for _, h := range holds {
						releaseBatches = append(releaseBatches, redisReleaseBatch{
							keys:  []string{h.RedisKey},
							token: h.HoldToken,
						})
					}
				}
			}

			if before.BookingGroupID != nil {
				groupSet[*before.BookingGroupID] = struct{}{}
			}

			full, err := s.repo.GetBookingByID(txCtx, before.ID)
			if err != nil {
				return err
			}
			cancelledBookings = append(cancelledBookings, full)
		}

		// Recompute all affected booking groups in deterministic sorted UUID order.
		groupIDs := make([]uuid.UUID, 0, len(groupSet))
		for gid := range groupSet {
			groupIDs = append(groupIDs, gid)
		}
		sort.Slice(groupIDs, func(i, j int) bool { return groupIDs[i].String() < groupIDs[j].String() })
		for _, gid := range groupIDs {
			if _, err := s.repo.RecomputeBookingGroupStatus(txCtx, gid, now); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Post-commit best-effort Redis hold key release.
	if s.holdStore != nil {
		for _, b := range releaseBatches {
			if _, relErr := s.holdStore.ReleaseHoldKeys(ctx, b.keys, b.token); relErr != nil {
				s.logger.Warn("post-cancel redis hold release failed", slog.Any("error", relErr))
			}
		}
	}

	return cancelledBookings, nil
}

// isFreeCancelEligible checks whether now <= (checkInDate at hotel checkInTime in hotel timezone) - freeCancelHours.
func isFreeCancelEligible(now time.Time, checkInDate time.Time, cfg propertyapp.HotelSettings) bool {
	loc, err := cfg.Location()
	if err != nil || loc == nil {
		loc = time.UTC
	}
	hour, min := 14, 0
	parts := strings.Split(strings.TrimSpace(cfg.CheckInTime), ":")
	if len(parts) == 2 {
		if h, err := strconv.Atoi(parts[0]); err == nil {
			hour = h
		}
		if m, err := strconv.Atoi(parts[1]); err == nil {
			min = m
		}
	}
	checkInLocal := time.Date(checkInDate.Year(), checkInDate.Month(), checkInDate.Day(), hour, min, 0, 0, loc)
	cutoff := checkInLocal.Add(-cfg.FreeCancelWindow())
	return !now.After(cutoff)
}
