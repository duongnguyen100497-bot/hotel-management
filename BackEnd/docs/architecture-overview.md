# Backend architecture overview

> Status: this is the original discovery overview. The fixed-requirements implementation specification, which supersedes any differing provisional recommendation here, is [implementation-plan.md](implementation-plan.md).

## Scope and evidence

This is a planning-only blueprint for the React/Vite hotel-management frontend. `BackEnd/` contained only a placeholder README when inspected. The frontend has two independently routed surfaces: a public/customer site and `/admin` operations console. Its services are in-memory mocks; online customer data, customer credentials, hotel settings, support messages, and the admin permission matrix are persisted only in browser storage. None is a production authority.

The product needs a single Go REST application and PostgreSQL database. The target architecture is a **modular monolith**: one deployable process, one database, explicit domain packages, and transactional workflows. It is not a microservice plan.

Evidence: `FrontEnd/README.md`, `src/shared/services/*`, `src/customer/services/*`, `src/admin/services/*`, routes, forms, and `admin/utils/permissions.js`.

## Confirmed capabilities

- Customer registration/login, profile/password management, room-type search, availability, multi-room online booking, own-booking view/cancellation, and support requests.
- Operations: rooms, customer records, reservations, check-in/out, room readiness, catalogued services, invoices/payments, dashboard/reports, employees, hotel settings, activity history, password reset.
- An online booking reserves concrete rooms as individual reservations sharing a group identifier; inventory overlap is prevented; checkout creates an invoice and changes the room to cleaning.

The following are frontend samples or incomplete flows, not confirmed policy: fixed room types/statuses, VAT 10%, service fee 5%, 48-hour free cancellation, homepage marketing content/offers/testimonials, synthetic report numbers, and direct “paid” marking without a payment provider.

## Architectural decisions

| Decision | Status | Why / alternatives / trade-off |
|---|---|---|
| Modular monolith with domain-owned packages and tables | Mandatory | Fits one hotel operational system and PostgreSQL transactions. Microservices add network and distributed consistency complexity without evidence. |
| PostgreSQL, UUID primary keys, `timestamptz` in UTC, `date` for stay dates, `numeric(14,2)` for money | Mandatory | Prevents ID enumeration assumptions, timezone ambiguity, and floating point money errors. `bigint` is viable but offers no demonstrated advantage. |
| Reservation occupancy enforced in PostgreSQL using a daterange exclusion constraint | Mandatory | Frontend checks race under concurrent requests. Application locks alone are fragile. |
| Server-calculated immutable price/tax snapshots | Mandatory | Frontend calculates totals but the server must be authoritative and preserve financial history after prices/policies change. |
| Separate customer and staff principals in one `users` identity model | Mandatory | The frontend has two sessions, but shared authentication/security controls and RBAC administration are needed. Principal type/context controls surface access. |
| Dynamic database RBAC using permissions, roles, assignments | Mandatory | Replaces UI-only four-role/module matrix. Roles are seed defaults, not hardcoded authorization. |
| Short access JWT plus rotating, server-stored refresh session in Secure HttpOnly cookie | Provisional, recommended | Prevents browser-token persistence and permits revocation. Server sessions/cookies only is acceptable for a same-origin web app; decide with frontend deployment topology. |
| Transactional outbox for emails/audit-notification delivery | Mandatory once email is enabled | Checkout/booking state and notification intent must not diverge. A job queue before actual integrations is unnecessary. |
| Object storage for logo/room/media uploads | Required for production media | Browser data URLs/Unsplash links cannot be an operational content system. Local disk is acceptable only for development. |

## Domain modules

| Module | Owns | Primary consumers |
|---|---|---|
| Identity & Access | users, credentials, sessions, RBAC, password reset | all authenticated APIs |
| People | customer profiles, employee records | booking, stays, admin |
| Property | hotel settings, room types, rooms, amenities, media, operational room state | catalog, booking, operations |
| Reservations | booking groups, reservations, status/history, cancellation, price snapshots | customer booking, operations |
| Stay Operations | check-in/out and service consumption | reception/housekeeping/accounting |
| Billing | invoices, invoice lines, payments/refunds | checkout, customer invoice view, reports |
| Service Catalog | services and categories | stay operations, billing |
| Support | customer support tickets/messages | customer site, staff |
| Reporting | query/read models only; no source-of-truth tables initially | dashboard/reports |
| Audit & Notifications | immutable audit entries, transactional outbox | cross-cutting but not business-owner shortcut |

Detailed boundaries: [module-design.md](module-design.md). Requirements evidence: [frontend-requirements-analysis.md](frontend-requirements-analysis.md).

## Dependency rules

`transport/http -> module application -> module domain -> module repository port -> infrastructure/postgres`.

- HTTP handlers parse/authenticate/validate DTOs and map errors only; no business decisions or SQL.
- A module writes only tables it owns. Another module invokes its exported application interface; it never imports its repository or mutates its tables.
- Read-only reporting may join owned read projections through repository methods; writes never bypass ownership.
- Domain code imports neither `net/http`, router types, SQL driver types, nor other module internals.
- Shared packages are limited to configuration, DB transaction runner, auth principal, logging, errors, clock/ID interfaces and observability—not business models.
- `audit` and `outbox` are written via a narrow shared transaction service, not queried/mutated arbitrarily.

## Key cross-module workflows

| Initiator | Steps / atomicity | Failure behavior |
|---|---|---|
| Online booking | Identity verifies owner; Reservations validates policy/capacity; Property selects rooms; create group + reservations + snapshots + status history + outbox in one transaction | Exclusion constraint returns availability conflict; no partial group. Idempotency key returns prior completed result. |
| Staff reservation | People, Property, Reservations in one transaction | rejection leaves no reservation. |
| Check-in | Reservations validates `confirmed`; Property validates operational state; update reservation/stay and room state + audit atomically | failure leaves all unchanged. |
| Checkout | Stay Operations records actual checkout and service consumption; Billing creates immutable invoice/lines; Property changes room to cleaning; outbox/audit atomically | no invoice or room transition on failure. Payment is separate. |
| Mark paid/refund | Billing changes payment/invoice derived status and writes audit/outbox atomically | idempotency key prevents duplicate collection record. |
| RBAC change | Identity & Access updates assignments/version + audit; sessions are invalidated or authorization version is checked | actor cannot remove last recovery administrator. |

Normal PostgreSQL transactions are sufficient for these workflows. Outbox publishing is asynchronous only for email/notifications; it cannot alter committed business state.

## Runtime and lifecycle

1. Load typed environment configuration; fail closed for missing production secrets.
2. Initialize structured logger, PostgreSQL pool, migration-version check, repositories, modules, background outbox worker, and HTTP router.
3. Expose `/healthz` (process alive) and `/readyz` (DB ping/pool usable; migrations compatible).
4. Start HTTP server; use request IDs, panic recovery, CORS/CSRF policy, body limits, rate limits, and metrics.
5. On SIGTERM/SIGINT stop accepting traffic, drain server, stop worker, close pool/logger before deadline.

## Go stack

| Concern | Recommendation | Required? | Reason / alternative |
|---|---|---:|---|
| HTTP | `net/http` + Gin (`github.com/gin-gonic/gin`) | Yes | Route groups (`*gin.RouterGroup`), middleware chain (`gin.HandlerFunc`), handlers take `*gin.Context`. Do not add `chi` (blocked by `internal/arch_test.go`). |
| PostgreSQL | `pgx/v5` + `pgxpool` | Yes | Native PostgreSQL support/pooling. `database/sql` is a viable abstraction but less direct. |
| Queries | SQLC-generated typed query layer plus hand-written repository interfaces | Yes | Explicit SQL and compile-time DTOs; ORM would obscure exclusion constraints/reporting SQL. |
| Migrations | `golang-migrate/migrate` SQL files | Yes | Clear production workflow; Atlas is a viable alternative, choose one only. |
| Config | stdlib env parsing into typed config | Yes | avoids a dependency for a small config surface. |
| Logging | `log/slog` JSON handler | Yes | stdlib structured logging. |
| Validation | `go-playground/validator/v10` plus domain validation | Recommended | DTO syntax validation; it never replaces domain checks. Manual validation is viable. |
| Passwords | `golang.org/x/crypto/argon2id` | Yes | memory-hard server hashing; bcrypt acceptable if organizational standard. |
| Tests | stdlib `testing`, `httptest`, Testcontainers Go | Yes | realistic PostgreSQL/concurrency/migration tests. |
| Docs | OpenAPI 3.1 authored as contract; generated handler bindings optional | Yes | frontend integration and acceptance clarity. |
| Jobs | in-process polling outbox worker | Initial | no broker evidence; use `FOR UPDATE SKIP LOCKED`. |

See [package-structure.md](package-structure.md), [security-design.md](security-design.md), and [implementation-roadmap.md](implementation-roadmap.md).
