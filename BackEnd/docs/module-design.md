# Module design

| Module | Own tables | Public application interfaces / key rules | Dependencies |
|---|---|---|---|
| Identity & Access | users, user_credentials, auth_sessions, password_reset_tokens, roles, permissions, user_roles, role_permissions | Authenticate, register customer, refresh/logout, reset/change password, manage users/roles/permissions/assignments. Enforce self-service and RBAC. | Audit/outbox |
| People | customers, employees | Customer/employee profile lifecycle, identities, searchable staff/customer lists. Government document encrypted. | Identity for principal linkage; Reservations read queries |
| Property | hotel_settings, room_types, rooms, amenities, room_type_amenities, media_assets, room_type_media, room_state_history, booking_policies | Publish catalogue; room inventory and state transitions; policy/config media. Cannot set occupied/cleaning by arbitrary CRUD. | Media storage; Reservations query port |
| Reservations | booking_groups, reservations, reservation_status_history | Quote/allocate/create/confirm/amend/cancel/list; owns price snapshot/status. Customer ownership guard. | People & Property read ports, Billing query for invoices |
| Service Catalog | service_categories, services | CRUD/deactivate services, price/version supplied to checkout. | none |
| Stay Operations | stays, stay_service_consumptions | Check-in/out, mark room ready, service consumption. | Reservations, Property, Service Catalog, Billing interfaces |
| Billing | invoices, invoice_lines, payments, refunds | Issue invoice; record payment/refund; receipt/read APIs. Financial data immutable by correction records. | Reservation/Stay snapshots |
| Support | support_tickets, support_messages | Submit ticket, customer/staff views, assign/respond/close. | Identity/People |
| Reporting | no source tables initially | dashboards/report queries/export data only. | read contracts across modules |
| Audit & Notifications | audit_events, outbox_events | append audit; enqueue and deliver notification intent. | shared transaction runner |

## Transaction boundaries

- Identity role and permission changes: one transaction, increment `users.authorization_version` for impacted users, audit.
- Property CRUD is isolated; state transitions use an application method and state history.
- Reservation group creation/amend/cancel: each command one transaction. Amendments after confirmation require a confirmed business policy; initially restrict to pending/confirmed and revalidate allocation.
- Check-in and checkout as defined in architecture overview. Payment/refund each independent atomic command.
- Ticket message + status/assignment update one transaction.

## Package layout

```text
BackEnd/
  cmd/api/main.go                    # composition only
  internal/platform/{config,db,http,auth,log,observability,clock,id}/
  internal/modules/
    identity/{domain,application,delivery/http,persistence/postgres}/
    people/{...}
    property/{...}
    reservations/{...}
    servicecatalog/{...}
    stayops/{...}
    billing/{...}
    support/{...}
    reporting/{application,delivery/http,persistence/postgres}/
    audit/{...}
  db/{migration,query,seed}/
  api/openapi.yaml
```

Within a module, exported types are small command/query DTOs and interfaces in `application`; repositories remain private. The composition root is the only place assembling concrete cross-module dependencies.
