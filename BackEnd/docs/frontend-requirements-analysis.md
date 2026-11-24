# Frontend requirements analysis

## Inventory and evidence classification

| Feature / actors | Inputs and outputs | Backend operation + authorization | Evidence / gaps |
|---|---|---|---|
| Public room catalogue / visitor | dates, adults, children, rooms; type/price/amenity filters; room details | search room types and availability; public read | `customerRoomService`; presentation data (photos, amenities) is static and must become property content. |
| Customer account / customer | name, email, phone, password; profile; password | register, authenticate, refresh/logout, read/update own profile/password | `customerAuthService`; reset/verification absent. Email is immutable in UI but backend policy undecided. |
| Online booking / customer | guest contact/nationality/request, dates, room count/type, payment intent | recheck availability, allocate rooms, create group/reservations, quote/snapshot, cancel own eligible booking | `customerBookingService`; card explicitly unavailable; no confirmation notification. |
| Contact/support / visitor/customer | name/email/optional phone/topic/message | create support ticket; staff list/respond/close | `supportService` saves browser-only, no staff workflow. |
| Rooms / staff | room code/name/type/nightly price/floor/capacity/state/description | CRUD and controlled operational state transition | `RoomForm`, `roomService`; no room-type/content/media admin UI. |
| Customers / staff | identity/contact/nationality/address | CRUD, history; delete blocked with active booking | `CustomerForm`; ID document is sensitive; online registration deliberately leaves it blank. |
| Reservations / staff | customer, room/type, dates, guests, status, note | CRUD/rebook/confirm/cancel; conflict check | `BookingForm`, `bookingService`; status transition rules currently weak (form may edit status). |
| Check-in/out / reception | reservation; consumed services, quantities, surcharge, payment method | check-in, checkout/create invoice, mark room clean/ready | `checkInOutService`; no no-show/early checkout/damage evidence. |
| Services / staff | name, price, category, description, active | catalogue CRUD and list eligible services | `ServiceForm`; checkout snapshots only name/price, not service ID. |
| Billing / accounting | invoice inspection, payment method; paid action | view invoice, record payment, receipt | `Invoices`; no payment references, partial payments, refunds, tax invoice/legal requirements. |
| Reports/dashboard / staff | period day/month/year, export CSV/print | dashboard and reports computed from source data | `reportService` is synthetic; reporting definition must be made real. |
| Employees/settings / staff | employee identity/position/status/username; hotel identity/logo/check-in/out time | employee lifecycle, own profile/password, property configuration | employee CRUD does not create credentials/roles; logo is local data URL. |
| Admin RBAC / privileged staff | matrix roles × modules | full dynamic role/permission/user-role admin APIs | current matrix is browser `localStorage`; four seed role labels are evidence only. |

## Explicit frontend validations that the server must repeat

- Room: code 2–10 alphanumeric/hyphen; positive price/floor/capacity.
- Reservation: check-out after check-in; new staff booking cannot start in past; guests positive and within room capacity; rooms cannot overlap active reservations.
- Customer/employee/booking contact: validated email, Vietnamese-style phone formatting; customer government ID/passport minimum 6 and unique where supplied.
- Customer password: 8+ characters including letter and digit. The admin UI says 6, but production policy must be unified (recommend 12+ length/passphrase policy; decision required).
- Customer search: arrival not past, max 30 nights, 1–10 adults, 0–6 children, 1–5 rooms and rooms not greater than adults. These UI caps are provisional operational policies.
- Online request max 500 characters; support message 10–1,000 characters; logo only image and max 512 KB. Server must inspect content/magic bytes, not trust MIME.

## Existing simulation that must be replaced

- `createMockService` creates numeric display IDs and mutable in-memory records.
- `onlineDataStore`, auth contexts, settings context, and support service use `localStorage`/`sessionStorage`.
- Admin authorization is based on `user.role` and a browser-editable matrix; it is not security.
- Customer SHA-256+salt hashing is browser demo only; admin passwords are plain mock data.
- Availability selects room then creates reservations sequentially and compensates by deleting; backend needs one transaction + database constraint.
- Reports use seeded random revenue/occupancy and add mock invoices; they cannot be migrated as data.
- Homepage offers/testimonials/gallery and Unsplash URLs are sample content; publication/content ownership is unresolved.

## Current roles, interpreted but not adopted as fixed design

| UI role | Evidence-supported operational meaning | Seed-role recommendation |
|---|---|---|
| Quản trị viên / admin | system configuration and all UI modules | protected `system-administrator` recovery role |
| Quản lý / manager | most operations, excluding employees | `hotel-manager` with no access-management by default |
| Lễ tân / receptionist | bookings, guests, rooms, check-in/out, services | `front-desk` |
| Kế toán / accountant | invoices, reports, dashboard | `accounting` |

These are default assignments only. Permissions must describe actions (`reservation.confirm`, not page `bookings`), and new roles/users must be supported.

## Missing UI needed for a viable production product

| Missing UI | Priority | Backend support | Production blocker? |
|---|---:|---|---:|
| Staff support-ticket queue, response and close workflow | High | Support ticket/message APIs | Yes if public support form ships |
| Role, permission, user-role management (not just module matrix) | High | Identity & Access admin APIs | Yes for dynamic RBAC requirement |
| Employee account provisioning, reset/deactivate, role assignment | High | staff user lifecycle APIs | Yes |
| Customer/staff password reset with email, verification and session management | High | auth token/email APIs | Yes |
| Reservation confirmation/decline and cancellation reason/refund presentation | High | transition/history/payment APIs | Yes |
| Property content management: room type, amenities, media, policies, published site content | High | Property/media/config APIs | Yes if static data must be editable |
| Invoice payment/refund history and receipt download | High | payments/receipts APIs | Yes for accounting accuracy |
| Report definitions/data quality and export job progress | High | reporting APIs | Yes for real reports |
| Online card payment flow/webhooks | Later | payment-provider integration | No: frontend explicitly disables it |
| Customer reviews management | Later | reviews/moderation module | No: testimonials are samples |
| Housekeeping task assignment, no-show, early/late check out | Later | operations extension | No evidence, but operationally valuable |
