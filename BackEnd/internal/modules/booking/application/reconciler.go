package application

import (
	"context"
	"log/slog"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
	auditapp "github.com/trandinh0506/hotel-management/backend/internal/modules/audit/application"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/booking/domain"
)

// ExpiryScanStats reports the number of expired holds and pending bookings processed in one scan cycle.
type ExpiryScanStats struct {
	ExpiredHolds    int
	ExpiredBookings int
}

// ReconcileStats reports DB->Redis restorations, DB conflicts expired, and Redis orphan keys deleted.
type ReconcileStats struct {
	RestoredKeys    int
	ConflictedHolds int
	OrphansDeleted  int
}

type redisReleaseBatch struct {
	keys  []string
	token string
}

// ScanAndExpireOnce runs one authoritative PostgreSQL expiry cycle:
//  1. Locks and expires due active reservation_holds (FOR UPDATE SKIP LOCKED), then releases Redis keys post-commit.
//  2. Locks and expires due pending bookings (FOR UPDATE SKIP LOCKED), records booking_status_history + audit_events,
//     invokes lifecycleHook.OnBookingExpired, and recomputes affected booking_groups.status.
func (s *Service) ScanAndExpireOnce(ctx context.Context) (ExpiryScanStats, error) {
	var stats ExpiryScanStats
	now := s.clk.Now()

	// Step 1: Expire due reservation_holds.
	var releaseBatches []redisReleaseBatch
	err := s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		expiredHolds, err := s.repo.ClaimExpiredReservationHolds(txCtx, now, 100)
		if err != nil {
			return err
		}
		if len(expiredHolds) == 0 {
			return nil
		}

		holdIDs := make([]uuid.UUID, 0, len(expiredHolds))
		seenGroupKey := make(map[uuid.UUID]struct{})
		for _, h := range expiredHolds {
			holdIDs = append(holdIDs, h.ID)
			keys := []string{h.RedisKey}
			if h.BookingGroupID != nil {
				if _, seen := seenGroupKey[*h.BookingGroupID]; !seen {
					seenGroupKey[*h.BookingGroupID] = struct{}{}
					keys = append(keys, domain.GroupHoldRedisKey(*h.BookingGroupID))
				}
			}
			releaseBatches = append(releaseBatches, redisReleaseBatch{
				keys:  keys,
				token: h.HoldToken,
			})
		}

		if err := s.repo.UpdateReservationHoldsStatus(txCtx, holdIDs, domain.HoldStatusExpired, "expired", now); err != nil {
			return err
		}

		if s.audit != nil {
			for _, h := range expiredHolds {
				idCopy := h.ID
				if err := s.audit.Record(txCtx, auditapp.AuditEntry{
					Action:       "booking.hold_expired",
					ResourceType: "reservation_hold",
					ResourceID:   &idCopy,
					AfterData: map[string]any{
						"room_id":    h.RoomID.String(),
						"hold_mode":  string(h.HoldMode),
						"status":     string(domain.HoldStatusExpired),
						"expires_at": h.ExpiresAt.UTC().Format(time.RFC3339),
					},
				}); err != nil {
					return err
				}
			}
		}

		stats.ExpiredHolds = len(expiredHolds)
		return nil
	})
	if err != nil {
		return stats, err
	}

	// Post-commit Redis cleanup for expired holds.
	if s.holdStore != nil {
		for _, b := range releaseBatches {
			if _, relErr := s.holdStore.ReleaseHoldKeys(ctx, b.keys, b.token); relErr != nil {
				s.logger.Warn("post-expiry redis key release failed; reconciler will clean up",
					slog.Any("error", relErr),
				)
			}
		}
	}

	// Step 2: Expire due pending bookings.
	err = s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		expiredBookings, err := s.repo.ClaimExpiredPendingBookings(txCtx, now, 100)
		if err != nil {
			return err
		}
		if len(expiredBookings) == 0 {
			return nil
		}

		groupSet := make(map[uuid.UUID]struct{})
		fromPending := domain.BookingStatusPending

		for _, b := range expiredBookings {
			if err := s.repo.MarkBookingExpired(txCtx, b.ID, now); err != nil {
				return err
			}
			if err := s.repo.InsertBookingStatusHistory(txCtx, domain.BookingStatusHistoryEntry{
				ID:         s.ids.NewUUID(),
				BookingID:  b.ID,
				FromStatus: &fromPending,
				ToStatus:   domain.BookingStatusExpired,
				Reason:     "hold_expired",
				OccurredAt: now,
			}); err != nil {
				return err
			}
			if s.lifecycleHook != nil {
				if err := s.lifecycleHook.OnBookingExpired(txCtx, b.ID, b.BookingGroupID); err != nil {
					return err
				}
			}
			if s.audit != nil {
				idCopy := b.ID
				if err := s.audit.Record(txCtx, auditapp.AuditEntry{
					Action:       "booking.expired",
					ResourceType: "booking",
					ResourceID:   &idCopy,
					AfterData: map[string]any{
						"reference_code": b.ReferenceCode,
						"status":         string(domain.BookingStatusExpired),
					},
				}); err != nil {
					return err
				}
			}
			if b.BookingGroupID != nil {
				groupSet[*b.BookingGroupID] = struct{}{}
			}
		}

		// Recompute affected booking groups in deterministic sorted UUID order.
		groupIDs := make([]uuid.UUID, 0, len(groupSet))
		for gid := range groupSet {
			groupIDs = append(groupIDs, gid)
		}
		sort.Slice(groupIDs, func(i, j int) bool {
			return groupIDs[i].String() < groupIDs[j].String()
		})
		for _, gid := range groupIDs {
			if _, err := s.repo.RecomputeBookingGroupStatus(txCtx, gid, now); err != nil {
				return err
			}
		}

		stats.ExpiredBookings = len(expiredBookings)
		return nil
	})
	return stats, err
}

// ReconcileRedisOnce runs a bidirectional PostgreSQL <-> Redis synchronization pass:
//  1. DB -> Redis: restores missing keys for active non-expired DB holds with remaining TTL (earliest commit wins on conflict).
//  2. Redis -> DB: deletes orphan hold:* keys in Redis that do not match any active DB hold token.
func (s *Service) ReconcileRedisOnce(ctx context.Context) (ReconcileStats, error) {
	var stats ReconcileStats
	if s.holdStore == nil {
		return stats, nil
	}

	now := s.clk.Now()
	activeHolds, err := s.repo.ListActiveNonExpiredHolds(ctx, now)
	if err != nil {
		return stats, err
	}

	redisKeys, err := s.holdStore.ScanHoldKeys(ctx)
	if err != nil {
		return stats, err
	}

	// Group active holds by their logical unit (groupHoldID if group, else individual hold ID),
	// preserving earliest-commit-first ordering from SQL ORDER BY created_at ASC, id ASC.
	type holdUnit struct {
		key       string
		groupID   *uuid.UUID
		token     string
		expiresAt time.Time
		holds     []domain.ReservationHold
	}
	unitOrder := make([]string, 0, len(activeHolds))
	unitMap := make(map[string]*holdUnit, len(activeHolds))
	for _, h := range activeHolds {
		uKey := "ind:" + h.ID.String()
		if h.BookingGroupID != nil {
			uKey = "grp:" + h.BookingGroupID.String()
		}
		u, exists := unitMap[uKey]
		if !exists {
			u = &holdUnit{
				key:       uKey,
				groupID:   h.BookingGroupID,
				token:     h.HoldToken,
				expiresAt: h.ExpiresAt,
			}
			unitMap[uKey] = u
			unitOrder = append(unitOrder, uKey)
		}
		u.holds = append(u.holds, h)
	}

	validKeyTokens := make(map[string]string, len(activeHolds)*2)

	for _, uKey := range unitOrder {
		u := unitMap[uKey]
		remainingSec := int(math.Ceil(u.expiresAt.Sub(now).Seconds()))
		if remainingSec < 1 {
			continue
		}

		expectedKeys := make([]string, 0, len(u.holds)+1)
		if u.groupID != nil {
			expectedKeys = append(expectedKeys, domain.GroupHoldRedisKey(*u.groupID))
		}
		for _, h := range u.holds {
			expectedKeys = append(expectedKeys, h.RedisKey)
		}

		// Detect whether any expected key already belongs to an earlier valid hold or a conflicting token.
		hasConflict := false
		missingKeys := make([]string, 0, len(expectedKeys))
		for _, k := range expectedKeys {
			if ownerToken, claimed := validKeyTokens[k]; claimed && ownerToken != u.token {
				hasConflict = true
				break
			}
			existingToken, inRedis := redisKeys[k]
			if !inRedis {
				missingKeys = append(missingKeys, k)
			} else if existingToken != u.token {
				hasConflict = true
				break
			}
		}

		if hasConflict {
			holdIDs := make([]uuid.UUID, 0, len(u.holds))
			for _, h := range u.holds {
				holdIDs = append(holdIDs, h.ID)
			}
			_ = s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
				return s.repo.UpdateReservationHoldsStatus(txCtx, holdIDs, domain.HoldStatusExpired, "reconcile_conflict", now)
			})
			stats.ConflictedHolds += len(holdIDs)
			continue
		}

		if len(missingKeys) > 0 {
			acquired, err := s.holdStore.AcquireHoldKeys(ctx, missingKeys, u.token, remainingSec)
			if err != nil {
				return stats, err
			}
			if !acquired {
				// Raced with another key: mark this unit conflicted.
				holdIDs := make([]uuid.UUID, 0, len(u.holds))
				for _, h := range u.holds {
					holdIDs = append(holdIDs, h.ID)
				}
				_ = s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
					return s.repo.UpdateReservationHoldsStatus(txCtx, holdIDs, domain.HoldStatusExpired, "reconcile_conflict", now)
				})
				stats.ConflictedHolds += len(holdIDs)
				continue
			}
			stats.RestoredKeys += len(missingKeys)
			for _, k := range missingKeys {
				redisKeys[k] = u.token
			}
		}

		for _, k := range expectedKeys {
			validKeyTokens[k] = u.token
		}
	}

	// Redis -> DB: remove orphan keys not matching any active DB hold token.
	for rKey, rToken := range redisKeys {
		if expectedToken, ok := validKeyTokens[rKey]; !ok || expectedToken != rToken {
			deleted, err := s.holdStore.ReleaseHoldKeys(ctx, []string{rKey}, rToken)
			if err != nil {
				return stats, err
			}
			stats.OrphansDeleted += int(deleted)
		}
	}

	return stats, nil
}

// RunExpiryScanner runs ScanAndExpireOnce on a fixed interval until ctx is cancelled.
func (s *Service) RunExpiryScanner(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			stats, err := s.ScanAndExpireOnce(ctx)
			if err != nil && ctx.Err() == nil {
				s.logger.Error("booking expiry scanner failed", slog.Any("error", err))
			} else if stats.ExpiredHolds > 0 || stats.ExpiredBookings > 0 {
				s.logger.Info("booking expiry scanner completed",
					slog.Int("expired_holds", stats.ExpiredHolds),
					slog.Int("expired_bookings", stats.ExpiredBookings),
				)
			}
		}
	}
}

// RunReconciler executes an immediate startup reconciliation pass, then runs periodically until ctx is cancelled.
func (s *Service) RunReconciler(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 90 * time.Second
	}
	if stats, err := s.ReconcileRedisOnce(ctx); err != nil && ctx.Err() == nil {
		s.logger.Warn("initial booking redis reconciliation failed", slog.Any("error", err))
	} else if stats.RestoredKeys > 0 || stats.OrphansDeleted > 0 || stats.ConflictedHolds > 0 {
		s.logger.Info("initial booking redis reconciliation completed",
			slog.Int("restored_keys", stats.RestoredKeys),
			slog.Int("orphans_deleted", stats.OrphansDeleted),
			slog.Int("conflicted_holds", stats.ConflictedHolds),
		)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			stats, err := s.ReconcileRedisOnce(ctx)
			if err != nil && ctx.Err() == nil {
				s.logger.Warn("periodic booking redis reconciliation failed", slog.Any("error", err))
			} else if stats.RestoredKeys > 0 || stats.OrphansDeleted > 0 || stats.ConflictedHolds > 0 {
				s.logger.Info("periodic booking redis reconciliation completed",
					slog.Int("restored_keys", stats.RestoredKeys),
					slog.Int("orphans_deleted", stats.OrphansDeleted),
					slog.Int("conflicted_holds", stats.ConflictedHolds),
				)
			}
		}
	}
}
