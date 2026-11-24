# Entity relationship diagrams

All IDs are UUID primary keys unless noted. `NN` means NOT NULL. Monetary fields are `numeric(14,2)`; timestamps are `timestamptz`.

## Overall ERD

```mermaid
erDiagram
 USERS ||--o| USER_CREDENTIALS : authenticates
 USERS ||--o{ AUTH_SESSIONS : owns
 USERS ||--o{ USER_ROLES : assigned
 ROLES ||--o{ USER_ROLES : grants
 ROLES ||--o{ ROLE_PERMISSIONS : has
 PERMISSIONS ||--o{ ROLE_PERMISSIONS : enables
 USERS ||--o| CUSTOMERS : represents
 USERS ||--o| EMPLOYEES : represents
 ROOM_TYPES ||--o{ ROOMS : contains
 ROOM_TYPES ||--o{ ROOM_TYPE_AMENITIES : has
 AMENITIES ||--o{ ROOM_TYPE_AMENITIES : tags
 ROOM_TYPES ||--o{ ROOM_TYPE_MEDIA : displays
 MEDIA_ASSETS ||--o{ ROOM_TYPE_MEDIA : linked
 CUSTOMERS ||--o{ BOOKING_GROUPS : owns
 BOOKING_POLICIES ||--o{ BOOKING_GROUPS : snapshots
 BOOKING_GROUPS ||--o{ RESERVATIONS : contains
 ROOMS ||--o{ RESERVATIONS : allocated
 ROOM_TYPES ||--o{ RESERVATIONS : snapshotted
 RESERVATIONS ||--o{ RESERVATION_STATUS_HISTORY : changes
 RESERVATIONS ||--o| STAYS : realizes
 STAYS ||--o{ STAY_SERVICE_CONSUMPTIONS : consumes
 SERVICES ||--o{ STAY_SERVICE_CONSUMPTIONS : source
 RESERVATIONS ||--o| INVOICES : billed
 INVOICES ||--o{ INVOICE_LINES : includes
 INVOICES ||--o{ PAYMENTS : collects
 PAYMENTS ||--o{ REFUNDS : reverses
 CUSTOMERS ||--o{ SUPPORT_TICKETS : opens
 SUPPORT_TICKETS ||--o{ SUPPORT_MESSAGES : contains
 USERS ||--o{ AUDIT_EVENTS : acts
 ROOMS ||--o{ ROOM_STATE_HISTORY : changes
```

## Core physical schema

```mermaid
erDiagram
 USERS { uuid id PK "NN" varchar principal_type "NN" citext email "NN UQ" citext username "UQ" varchar status "NN" int authorization_version "NN" timestamptz archived_at }
 ROLES { uuid id PK varchar code "NN UQ" varchar name "NN" boolean is_system "NN" boolean is_protected "NN" timestamptz archived_at }
 PERMISSIONS { uuid id PK varchar code "NN UQ" varchar name "NN" boolean is_system "NN" boolean is_protected "NN" timestamptz archived_at }
 USER_ROLES { uuid user_id FK "NN" uuid role_id FK "NN" uuid granted_by FK timestamptz granted_at "NN" }
 ROLE_PERMISSIONS { uuid role_id FK "NN" uuid permission_id FK "NN" uuid granted_by FK timestamptz granted_at "NN" }
 CUSTOMERS { uuid id PK uuid user_id FK "UQ" text full_name "NN" text phone "NN" citext email bytea government_id_ciphertext bytea government_id_hash "UQ" varchar source "NN" timestamptz archived_at }
 EMPLOYEES { uuid id PK uuid user_id FK "UQ" varchar employee_code "NN UQ" text full_name "NN" citext email "NN UQ" varchar employment_status "NN" timestamptz archived_at }
 ROOM_TYPES { uuid id PK varchar code "NN UQ" text name "NN" numeric base_nightly_rate "NN" char currency "NN" smallint capacity "NN" boolean is_bookable "NN" timestamptz archived_at }
 ROOMS { uuid id PK varchar code "NN UQ" text name "NN" uuid room_type_id FK "NN" smallint floor "NN" smallint capacity "NN" varchar operational_status "NN" timestamptz archived_at }
 BOOKING_GROUPS { uuid id PK varchar reference_code "NN UQ" uuid customer_id FK "NN" uuid policy_id FK "NN" varchar status "NN" varchar source "NN" uuid idempotency_key numeric total_amount "NN" }
 RESERVATIONS { uuid id PK varchar reference_code "NN UQ" uuid group_id FK "NN" uuid customer_id FK "NN" uuid room_id FK "NN" uuid room_type_id FK "NN" daterange stay "NN" smallint adults "NN" smallint children "NN" varchar status "NN" numeric room_amount "NN" numeric tax_amount "NN" timestamptz checked_in_at timestamptz checked_out_at }
 INVOICES { uuid id PK varchar reference_code "NN UQ" uuid reservation_id FK "NN UQ" uuid customer_id FK "NN" varchar status "NN" numeric total_amount "NN" timestamptz issued_at "NN" timestamptz paid_at }
 PAYMENTS { uuid id PK varchar reference_code "NN UQ" uuid invoice_id FK "NN" varchar method "NN" varchar status "NN" numeric amount "NN" varchar idempotency_key "UQ" }
 USERS ||--o{ USER_ROLES : "user_id" ROLES ||--o{ USER_ROLES : "role_id" ROLES ||--o{ ROLE_PERMISSIONS : "role_id" PERMISSIONS ||--o{ ROLE_PERMISSIONS : "permission_id" USERS ||--o| CUSTOMERS : "user_id" USERS ||--o| EMPLOYEES : "user_id" ROOM_TYPES ||--o{ ROOMS : "room_type_id" CUSTOMERS ||--o{ BOOKING_GROUPS : "customer_id" BOOKING_GROUPS ||--o{ RESERVATIONS : "group_id" ROOMS ||--o{ RESERVATIONS : "room_id" RESERVATIONS ||--o| INVOICES : "reservation_id" INVOICES ||--o{ PAYMENTS : "invoice_id"
```

The implementation migration contains the complete table catalogue in [database-design.md](database-design.md); Mermaid is intentionally split to remain readable.
