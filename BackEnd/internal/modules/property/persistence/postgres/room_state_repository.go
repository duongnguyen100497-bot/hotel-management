package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/property/domain"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/db"
)

// UpdateRoomOperationalStatus atomically updates rooms.operational_status with optimistic version check.
func (r *Repository) UpdateRoomOperationalStatus(
	ctx context.Context,
	roomID uuid.UUID,
	targetStatus string,
	expectedVersion int,
	now time.Time,
) (domain.Room, error) {
	const sqlText = `
UPDATE rooms
SET
    operational_status = $1,
    updated_at         = $2,
    version            = version + 1
WHERE id = $3
  AND archived_at IS NULL
  AND version = $4
RETURNING
    id,
    code,
    name,
    room_type_id,
    floor,
    capacity,
    operational_status,
    description,
    archived_at,
    created_at,
    updated_at,
    version`

	var (
		rm     domain.Room
		floor  int16
		capVal int16
		desc   pgtype.Text
		ver    int32
	)
	err := db.Querier(ctx, r.pool).QueryRow(ctx, sqlText, targetStatus, now, roomID, int32(expectedVersion)).Scan(
		&rm.ID, &rm.Code, &rm.Name, &rm.RoomTypeID, &floor, &capVal,
		&rm.OperationalStatus, &desc, &rm.ArchivedAt, &rm.CreatedAt, &rm.UpdatedAt, &ver,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Room{}, apperror.VersionConflict("room version conflict").WithCause(err)
		}
		return domain.Room{}, db.MapPgError(err)
	}
	rm.Floor = int(floor)
	rm.Capacity = int(capVal)
	if desc.Valid {
		rm.Description = desc.String
	}
	rm.Version = int(ver)
	return rm, nil
}

// InsertRoomStateHistory appends an immutable transition record into room_state_history.
func (r *Repository) InsertRoomStateHistory(ctx context.Context, entry domain.RoomStateHistoryEntry) (domain.RoomStateHistoryEntry, error) {
	const sqlText = `
INSERT INTO room_state_history (
    id,
    room_id,
    from_status,
    to_status,
    reason,
    actor_user_id,
    occurred_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING
    id,
    room_id,
    from_status,
    to_status,
    reason,
    actor_user_id,
    occurred_at`

	var (
		out    domain.RoomStateHistoryEntry
		reason pgtype.Text
	)
	err := db.Querier(ctx, r.pool).QueryRow(
		ctx,
		sqlText,
		entry.ID,
		entry.RoomID,
		entry.FromStatus,
		entry.ToStatus,
		toPgText(entry.Reason),
		entry.ActorUserID,
		entry.OccurredAt,
	).Scan(
		&out.ID,
		&out.RoomID,
		&out.FromStatus,
		&out.ToStatus,
		&reason,
		&out.ActorUserID,
		&out.OccurredAt,
	)
	if err != nil {
		return domain.RoomStateHistoryEntry{}, db.MapPgError(err)
	}
	if reason.Valid {
		out.Reason = reason.String
	}
	return out, nil
}

// ListRoomStateHistory returns cursor-paginated state transition history for a room (newest first).
func (r *Repository) ListRoomStateHistory(ctx context.Context, roomID uuid.UUID, cursor string, limit int) ([]domain.RoomStateHistoryEntry, *string, error) {
	if limit <= 0 {
		limit = 20
	}
	args := []any{roomID}
	cond := "room_id = $1"
	if cursor != "" {
		cur, err := decodeCursor[timeIDCursor](cursor)
		if err != nil || cur.ID == uuid.Nil {
			return nil, nil, apperror.Validation("invalid pagination cursor", apperror.FieldError{
				Field:   "cursor",
				Message: "malformed cursor",
			})
		}
		args = append(args, cur.TS, cur.ID)
		cond += " AND (occurred_at < $2 OR (occurred_at = $2 AND id < $3))"
	}
	args = append(args, limit+1)
	sqlText := fmt.Sprintf(`
SELECT
    id,
    room_id,
    from_status,
    to_status,
    reason,
    actor_user_id,
    occurred_at
FROM room_state_history
WHERE %s
ORDER BY occurred_at DESC, id DESC
LIMIT $%d`, cond, len(args))

	rows, err := db.Querier(ctx, r.pool).Query(ctx, sqlText, args...)
	if err != nil {
		return nil, nil, db.MapPgError(err)
	}
	defer rows.Close()

	items := make([]domain.RoomStateHistoryEntry, 0, limit)
	for rows.Next() {
		var (
			item   domain.RoomStateHistoryEntry
			reason pgtype.Text
		)
		if err := rows.Scan(
			&item.ID,
			&item.RoomID,
			&item.FromStatus,
			&item.ToStatus,
			&reason,
			&item.ActorUserID,
			&item.OccurredAt,
		); err != nil {
			return nil, nil, db.MapPgError(err)
		}
		if reason.Valid {
			item.Reason = reason.String
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, db.MapPgError(err)
	}

	var nextCursor *string
	if len(items) > limit {
		items = items[:limit]
		last := items[len(items)-1]
		encoded := encodeCursor(timeIDCursor{TS: last.OccurredAt, ID: last.ID})
		nextCursor = &encoded
	}
	return items, nextCursor, nil
}

// CountOperationalRoomsAt reconstructs the total active rooms, maintenance rooms, and operational rooms
// at an arbitrary historical or current timestamp using room_state_history.
func (r *Repository) CountOperationalRoomsAt(ctx context.Context, snapshotAt time.Time) (totalActive int, maintenance int, operational int, err error) {
	const sqlText = `
WITH active_rooms AS (
    SELECT
        rm.id,
        COALESCE(
            (
                SELECT h.to_status
                FROM room_state_history h
                WHERE h.room_id = rm.id
                  AND h.occurred_at <= $1
                ORDER BY h.occurred_at DESC, h.id DESC
                LIMIT 1
            ),
            (
                SELECT h.from_status
                FROM room_state_history h
                WHERE h.room_id = rm.id
                  AND h.occurred_at > $1
                ORDER BY h.occurred_at ASC, h.id ASC
                LIMIT 1
            ),
            rm.operational_status
        ) AS status_at
    FROM rooms rm
    WHERE rm.created_at <= $1
      AND (rm.archived_at IS NULL OR rm.archived_at > $1)
)
SELECT
    count(*)::int AS total_active,
    count(*) FILTER (WHERE status_at = 'maintenance')::int AS maintenance_count,
    count(*) FILTER (WHERE status_at <> 'maintenance')::int AS operational_count
FROM active_rooms`

	err = db.Querier(ctx, r.pool).QueryRow(ctx, sqlText, snapshotAt).Scan(&totalActive, &maintenance, &operational)
	if err != nil {
		return 0, 0, 0, db.MapPgError(err)
	}
	return totalActive, maintenance, operational, nil
}
