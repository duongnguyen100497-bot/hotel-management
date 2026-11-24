# Dependency-aware implementation roadmap

## Decisions required before coding

Resolve property tenancy, legal/tax/retention, pricing/cancellation policy, payment/manual-reconciliation rules, email provider/sender, production media storage, and role/permission approval (see [open-questions.md](open-questions.md)). These change schema or security boundaries.

| Phase | Objective, ordered work | Dependencies / acceptance |
|---|---|---|
| 0 — contract foundation | Confirm open questions; establish Go module, configuration/secrets, Gin/pgx/SQLC/migrate conventions, OpenAPI/error envelope, logging/health/readiness, CI/Testcontainers. | No business UI rewrites. Empty service starts/shuts down cleanly; migrations validated; architecture dependency checks documented. |
| 1 — identity and audit | Migrations 1–4; identity, sessions, password flow, RBAC seed/bootstrap, authorization middleware, audit/outbox primitives; staff/customer separation. | Requires role mapping approval. Authentication/RBAC negative suite and recovery-admin test pass. |
| 2 — people/property | Migrations 5–8; customers/employees, hotel settings/media, room types/rooms/amenities, policy publication; staff CRUD APIs and public catalogue. | Requires storage decision/property seed. PII masking/encryption and media validation proven. |
| 3 — reservations | Migration 9; quote, availability, online multi-room group creation/idempotency, staff reservations, confirmation/cancellation/history/ownership. | Needs property/policy. Parallel booking test proves zero overlap; price snapshots immutable. |
| 4 — operations/catalog/billing | Migrations 10–12; services, check-in/out, readiness, invoice lines, manual payments/refunds/receipt reads. | Needs reservation transitions/accounting rules. Checkout is atomic and audit/outbox verified. |
| 5 — customer/service operations completion | Migration 13; support ticket queue; password-reset email, booking/staff notification delivery; customer invoice and self-service APIs. | Needs email provider. Tickets are no longer browser-local; failed delivery observable. |
| 6 — reporting and frontend integration | Migration 14; replace synthetic report logic with approved definitions; dashboard/report APIs/exports; integrate frontend service layer and remove local mock persistence only after parity tests. | Needs business metric definitions. Reconciliation fixtures match report output. |
| 7 — production readiness | Migration 15 where approved; backup/restore, retention jobs, monitoring/alerts, load/security testing, deployment/runbooks, access reviews, penetration test. | All earlier flows. Recovery drill, incident runbook and go-live security checklist accepted. |
| Later | online card provider, reviews, housekeeping tasks, no-show/late check-out, SSE, MFA expansion. | Only after requirements/ROI evidence. |

Within a phase, schema before repository, domain/application before HTTP, then module/API tests. Modules can be independently implemented after their prerequisite phase; checkout must await all phase-4 collaborators. No phase substitutes browser mocks for server authority.
