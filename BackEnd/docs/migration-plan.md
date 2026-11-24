# Migration strategy

Use `golang-migrate` with reviewed, immutable SQL pairs in `BackEnd/db/migration/`, named `000001_description.up.sql` and `.down.sql`. Every migration is transactional where PostgreSQL permits it; `CREATE INDEX CONCURRENTLY` is a separately planned nontransactional production migration. SQLC queries live separately and are validated against the migrated schema.

## Ordered inventory

| # | Migration | Objects / dependency | Seed / rollback note |
|---:|---|---|---|
| 1 | `enable_extensions` | `pgcrypto`, `citext`, `btree_gist` | Extension down only if no dependents. |
| 2 | `create_identity_users` | users, credentials, sessions, reset tokens | Down only pre-production; never drop live identities. |
| 3 | `create_rbac` | roles, permissions, join tables | Seed system permissions/default roles and bootstrap procedure, not demo passwords. Down blocked after assignments. |
| 4 | `create_audit_outbox` | audit/outbox | Append-only; no production down. |
| 5 | `create_people` | customers/employees → users | migrate/seed data only by approved import. |
| 6 | `create_media_property_settings` | media, hotel settings | seed one settings row/approved timezone. |
| 7 | `create_room_catalog` | room types, rooms, amenities/mappings/media, state history | depends media; initial property data is an approved seed/import, not frontend mocks. |
| 8 | `create_booking_policies` | policies | seed exactly one provisional policy only after owner confirmation. |
| 9 | `create_reservations` | groups, reservations, history; GIST exclusion | depends people/property/policy; no down after bookings exist. |
| 10 | `create_service_catalog` | categories/services | approved catalogue seed optional. |
| 11 | `create_stay_operations` | stays/consumptions | depends reservations/services. |
| 12 | `create_billing` | invoices/lines/payments/refunds | depends reservations/stays; financial data never rolled back destructively. |
| 13 | `create_support` | tickets/messages | depends people/users. |
| 14 | `reporting_indexes_and_views` | explain-tested indexes/read views | reversible views/indexes only. |
| 15 | `retention_and_housekeeping` | archival indexes/optional jobs metadata | policy approval required. |

## Operational rules

- CI starts an empty PostgreSQL instance, runs every up migration, runs SQLC generation/verification, then tests a representative downgrade only for early disposable migrations. A fresh up migration is the normal recovery path; production down migrations cannot safely reverse data/backfills/financial effects.
- Release pipeline takes backup, runs migrations once with an advisory lock, verifies `schema_migrations`, then deploys code compatible with old/new schema (expand → deploy → backfill → contract). No startup auto-migration in production.
- Data seeds are idempotent (`ON CONFLICT` keyed by immutable code), separated from demo data, and audited. Initial hotel, roles/permissions, admin bootstrap, policies and property catalogue require named business owner approval.
- Every FK/index/check/exclusion constraint named explicitly. Backfills are resumable, throttled, observable, and avoid long locks.
