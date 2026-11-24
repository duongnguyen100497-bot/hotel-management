# Testing strategy

## Test layers and acceptance

| Layer | Scope / acceptance |
|---|---|
| Unit/domain | State transitions, pricing snapshots, cancellation eligibility, ownership, RBAC escalation guards, validation. No DB/HTTP. High branch coverage for invariants. |
| Repository/PostgreSQL integration | Testcontainers PostgreSQL, real migrations, SQLC/repository queries, constraints/index plans. Must prove overlapping concurrent reservations cannot both commit. |
| Module/application integration | Transaction rollback for booking/checkin/checkout/payment/RBAC change; outbox/audit created atomically. |
| HTTP/OpenAPI | `httptest` contract, auth headers/cookies, errors, pagination/cursors/filter allowlists, DTO validation and version conflicts. |
| Security | password/reuse/revocation, reset enumeration, CSRF/CORS, request size, upload validation, redacted logs, SQL injection payloads, rate-limit behaviors. |
| Workflow/E2E | customer registration → booking → staff confirmation → checkin → checkout → payment → customer invoice; support response; role-management workflows. |
| Migration | fresh up path each CI; compatibility fixtures/backfills; unsafe downs explicitly skipped with rationale. |
| Load/concurrency | booking allocation races, payment idempotency retry, session/RBAC cache invalidation; baseline latency/pool saturation. |

## Mandatory negative authorization matrix

- unauthenticated access returns 401; authenticated without permission returns 403 without state change;
- customer A cannot read/cancel customer B’s reservations/invoices/tickets;
- front-desk cannot record a refund or edit roles; accountant cannot alter room state unless assigned permission;
- caller cannot grant a role/permission above its grantable set, mutate protected system role, delete assigned role, or remove final recovery administrator;
- suspended/deactivated user, revoked/rotated refresh token, and changed authorization version are denied;
- unauthorized cross-module commands do not create audit/outbox/business side effects except security-denial metrics.

Quality gates: `go test -race ./...`, static analysis, formatting, dependency scan, migration test, API contract test, and integration suite against PostgreSQL must pass before merge. Tests use no demo credentials or production PII.
