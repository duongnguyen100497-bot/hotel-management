# REST API specification

Base path `/api/v1`; JSON UTF-8; camelCase request/response fields; UUID resource IDs. A successful list is `{data: [], page:{limit,cursor,nextCursor}}`; collection filters use documented query parameters and stable allowlisted sort keys. Cursor/keyset pagination is default; exports are separate operations.

Errors use `{error:{code,message,fields:[{field,code,message}],requestId}}`. Codes include `validation_failed` (422), `unauthenticated` (401), `forbidden` (403), `not_found` (404), `conflict` (409), `rate_limited` (429), and `internal_error` (500). `If-Match`/`version` gives `version_conflict` 409 for staff edits. Money serializes as decimal strings.

## Endpoint inventory

| Method/path | Module / permission | Purpose, validation, transaction / principal errors |
|---|---|---|
| POST `/auth/customer/register` | Identity / public | Create user+customer+credential; email/password/profile rules; 201/session. 409 existing email. |
| POST `/auth/login` | Identity / public | Customer/staff login with surface/type selection; rate limited; 200 access token + refresh cookie, generic 401. |
| POST `/auth/refresh`, `/auth/logout` | Identity / session | Rotate/revoke session; 401 invalid/reused token. |
| POST `/auth/password-reset-requests`, POST `/auth/password-resets` | Identity / public | Generic accepted response; token completion validates/reset/revokes sessions. |
| GET/PATCH `/me`, POST `/me/password`, GET/DELETE `/me/sessions` | Identity/People / self | Read/profile update/change password/session revocation; ownership only. |
| GET `/catalog/room-types`, GET `/catalog/room-types/{code}` | Property / public | Published types/content; optional `checkIn,checkOut,adults,children,rooms`; 422 date/capacity. |
| POST `/availability/quote` | Reservations / public | Server quote + availability, no hold; validates policy/dates/party. |
| GET `/room-types`, POST `/room-types`, PATCH `/room-types/{id}` | Property / property.read, room_type.manage | Staff catalogue management; version/state validation. |
| GET/POST/PATCH `/rooms`, POST `/rooms/{id}/archive`, POST `/rooms/{id}/state` | Property / property.read, room.*, room.state.update | Physical inventory and controlled operational state; 409 active stay/reservation. |
| GET `/amenities`, POST/PATCH `/amenities` | Property / property.read, room_type.manage | Amenity configuration. |
| POST `/media/uploads`, POST `/media/{id}/complete`, DELETE `/media/{id}` | Property / media.manage | Presigned upload + verified completion/delete; MIME/size/ownership checks. |
| GET/PUT `/hotel/settings`, GET/POST `/booking-policies` | Property / hotel_settings.*, room_type.manage | Config/policy version actions; policy publish validates effective dates. |
| GET/POST/PATCH `/customers`, POST `/customers/{id}/archive` | People / customer.* | Staff customer management; sensitive fields masked by permission; 409 active reservation. |
| GET/POST/PATCH `/employees`, POST `/employees/{id}/deactivate` | People / employee.* | Employee lifecycle only; account/role actions are Identity APIs. |
| POST `/booking-groups` | Reservations / authenticated customer | Online multi-room creation; `Idempotency-Key`; one transaction, 201 or 409 availability. |
| GET `/booking-groups/{id}`, GET `/me/booking-groups` | Reservations / reservation.read or read_self | Staff/all vs contextual customer scoped views. |
| GET/POST/PATCH `/reservations`, POST `/reservations/{id}/confirm`, `/cancel`, `/check-in`, `/check-out` | Reservations/Stay Ops / corresponding permission | Staff management/state commands; checkout returns invoice, transactionally coordinates modules. |
| POST `/me/reservations/{id}/cancel` | Reservations / cancel_self + ownership | Policy snapshot/time eligibility; 409 transition/policy conflict. |
| GET/POST/PATCH `/service-categories`, `/services` | Service Catalog / service.read/manage | Catalogue and deactivate; 409 referenced deletion. |
| GET `/invoices`, GET `/invoices/{id}`, GET `/me/invoices/{id}` | Billing / invoice.read or customer ownership | Invoice/line/payment view. |
| POST `/invoices/{id}/payments`, POST `/payments/{id}/refunds` | Billing / payment.record/refund | Idempotency required; amount/status validation; one transaction/audit. |
| GET `/dashboard`, GET `/reports/{revenue,occupancy,customers,services}` | Reporting / dashboard.read, report.read | Filtered real-data read models; 422 invalid periods. |
| POST `/reports/exports` | Reporting / report.export | async only if export size warrants; initially synchronous CSV for bounded report. |
| POST `/support-tickets`, GET `/me/support-tickets` | Support / public or self | Create/list own ticket; message limits and anti-spam. |
| GET `/support-tickets`, PATCH `/support-tickets/{id}`, POST `/support-tickets/{id}/messages` | Support / support.* | Staff queue/assign/status/respond. |
| GET/POST/PATCH `/users`, POST `/users/{id}/roles` | Identity / user.* / user_role.assign | Manage staff principal lifecycle/assign roles with escalation guard. |
| GET/POST/PATCH `/roles`, DELETE `/roles/{id}`, POST `/roles/{id}/permissions` | Identity / role.*, role_permission.assign | Dynamic role lifecycle; protected rules/last admin checks. |
| GET/POST/PATCH `/permissions`, DELETE `/permissions/{id}` | Identity / permission.* | Metadata lifecycle; codes immutable/restrictions. |
| GET `/audit-events` | Audit / audit.read | Filtered immutable security/business audit log. |

Authentication is required unless listed public. Customer requests use the same bearer/session mechanism but must meet owner scope. Rate-limit login/reset/registration/support and all writes; require idempotency for externally retried financial/booking commands.

OpenAPI 3.1 is the single API contract and must define schemas, examples, authorization security schemes, error responses, pagination and all state-command response semantics. Handler tests validate implementation against it.
