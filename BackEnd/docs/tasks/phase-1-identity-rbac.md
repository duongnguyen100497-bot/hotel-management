# Phase 1 — Danh tính, Dynamic RBAC, Kích hoạt nhân viên & Hồ sơ (Identity, RBAC & People)

> **Mục tiêu Phase:** Hoàn thiện hệ thống tài khoản khách hàng & nhân viên, xác thực JWT + refresh cookie xoay vòng, quy trình gửi email SMTP qua outbox (quên mật khẩu & kích hoạt tài khoản nhân viên), phân quyền động Dynamic RBAC kèm cơ chế bảo vệ quyền Admin root bất biến, và quản lý hồ sơ khách hàng/nhân viên.
>
> **Phụ thuộc:** Hoàn thành Phase 0 (`T0.1`–`T0.5`).
>
> **Tiêu chí hoàn thành Phase (Exit Criteria):**
> - Bootstrap tạo được đúng 1 tài khoản Root Admin (`is_root = true`) qua CLI và seed đầy đủ danh mục quyền chuẩn + 4 role mặc định.
> - Khách hàng đăng ký, đăng nhập, refresh token (có phát hiện reuse), đổi/quên mật khẩu (xem email trên Mailpit) và cập nhật `/me` hoạt động trọn vẹn.
> - Admin tạo nhân viên mới → sinh staff user ở trạng thái chờ kích hoạt + gửi email kích hoạt qua Mailpit → nhân viên đặt mật khẩu qua link kích hoạt và đăng nhập thành công.
> - Bộ test bảo mật & negative RBAC pass 100%: root không thể bị tước quyền, không thể leo thang đặc quyền, user bị khóa/đổi quyền bị từ chối ngay.

---

### T1.1 — Schema Identity & RBAC (Migration 003), Seed danh mục quyền chuẩn & CLI Bootstrap Root Admin

- **Commit gợi ý:** `feat(identity): add identity and rbac schema, permission seed and root admin bootstrap cli`
- **Depends on:** `T0.4`, `T0.5`
- **Modules / Files:** `BackEnd/db/migration/000003_create_identity_rbac.{up,down}.sql`, `BackEnd/db/query/{identity,rbac}.sql`, `BackEnd/db/seed/rbac_seed.sql`, `BackEnd/cmd/bootstrap/main.go`, `internal/modules/{identity,rbac}/`

#### Mục tiêu nghiệp vụ
Thiết lập mô hình dữ liệu tài khoản và Dynamic RBAC, seed sẵn toàn bộ mã quyền chuẩn của hệ thống + 4 vai trò mặc định, và cung cấp lệnh bootstrap an toàn tạo tài khoản Root Admin duy nhất ban đầu.

#### Phạm vi thực hiện
- Migration `000003_create_identity_rbac`:
  - `users`: `id uuid PK`, `kind varchar(16) NN CHECK (kind IN ('customer','staff','admin'))`, `email citext NN UQ`, `username citext ? UQ`, `display_name text NN`, `status varchar(24) NN CHECK (status IN ('pending_activation','active','disabled'))`, `authorization_version int NN default 1`, `last_login_at timestamptz ?`, `created_at/updated_at/version`.
  - `user_credentials`: `user_id uuid PK FK -> users`, `password_hash text NN`, `password_changed_at timestamptz NN`, `failed_login_count int NN default 0`, `locked_until timestamptz ?`.
  - `auth_sessions`: `id uuid PK`, `user_id uuid FK`, `refresh_token_hash bytea NN UQ`, `family_id uuid NN`, `issued_at/expires_at timestamptz NN`, `revoked_at timestamptz ?`, `replaced_by uuid ?`, `ip_hash bytea ?`, `user_agent_hash bytea ?`.
  - `activation_tokens` & `password_reset_tokens`: `id uuid PK`, `user_id uuid FK`, `token_hash bytea NN UQ`, `expires_at timestamptz NN`, `used_at timestamptz ?`, `created_at timestamptz NN`.
  - `roles`: `id uuid PK`, `code varchar(64) NN UQ`, `name text NN`, `description text ?`, `is_system boolean NN default false`, `is_root boolean NN default false`, `archived_at timestamptz ?`, `created_at/updated_at/version`. Partial unique index bảo đảm tối đa 1 role `is_root = true` active.
  - `permissions`: `id uuid PK`, `code varchar(100) NN UQ`, `name text NN`, `description text ?`, `is_system boolean NN default true`, `archived_at timestamptz ?`.
  - `user_roles (user_id, role_id, granted_by, granted_at)` và `role_permissions (role_id, permission_id, granted_by, granted_at)` composite PK + reverse index.
- Seed idempotent (`ON CONFLICT (code) DO NOTHING`):
  - Toàn bộ danh mục permission chuẩn liệt kê trong `README.md` (`user.*`, `role.*`, `permission.*`, `employee.*`, `customer.*`, `hotel_settings.*`, `vat.manage`, `property.read`, `room_type.manage`, `room.*`, `service.*`, `media.manage`, `booking.*`, `wallet.*`, `payment.*`, `stay.*`, `invoice.read`, `report.*`, `audit.read`).
  - 4 role hệ thống mặc định: `system-administrator` (`is_system=true, is_root=true`), `hotel-manager`, `front-desk`, `accounting` với bộ quyền khởi điểm tương ứng.
- CLI `cmd/bootstrap/main.go`: Đọc `BOOTSTRAP_ADMIN_EMAIL`, `BOOTSTRAP_ADMIN_PASSWORD`, `BOOTSTRAP_ADMIN_NAME` từ env, chạy seed RBAC idempotent, tạo 1 tài khoản `kind='admin', status='active'` gán role `system-administrator` (chỉ tạo nếu chưa có user giữ root role), băm mật khẩu bằng Argon2id, ghi audit event.

#### Quy tắc & Invariant bắt buộc
- Không hardcode mật khẩu demo trong SQL migration; bootstrap admin chỉ lấy mật khẩu từ biến môi trường lúc chạy CLI.
- `users` không bao giờ bị `ON DELETE CASCADE` xóa dữ liệu tài chính/audit.

#### Tiêu chí nghiệm thu & Kiểm thử
- Integration test (Testcontainers): chạy Migration 003 + chạy bootstrap CLI 2 lần liên tiếp (idempotent, chỉ có đúng 1 root admin và không bị duplicate seed).
- Chạy `cd BackEnd && go test -race ./internal/modules/identity/... ./internal/modules/rbac/...`.

---

### T1.2 — Đăng ký khách hàng, đăng nhập Argon2id, JWT Access Token & Refresh Cookie xoay vòng

- **Commit gợi ý:** `feat(identity): implement customer registration, argon2id login and rotating session tokens`
- **Depends on:** `T1.1`, `T0.3`
- **Modules / Files:** `internal/platform/auth/`, `internal/modules/identity/{domain,application,delivery/http,persistence/postgres}`, `BackEnd/api/openapi.yaml`

#### Mục tiêu nghiệp vụ
Cho phép khách hàng tự đăng ký tài khoản, khách hàng/nhân viên đăng nhập bằng mật khẩu băm Argon2id, cấp JWT access token ngắn hạn + refresh cookie xoay vòng có phát hiện đánh cắp token.

#### Phạm vi thực hiện
- Password hasher Argon2id (`golang.org/x/crypto/argon2`) với salt ngẫu nhiên từng mật khẩu; kiểm tra độ mạnh mật khẩu thống nhất (tối thiểu 8 ký tự gồm chữ và số).
- REST endpoints:
  - `POST /api/v1/auth/register` (public): Tạo `users` (`kind='customer', status='active'`) + `user_credentials` + mở session trả access token + cookie refresh (`201`). Trả `409` nếu email đã tồn tại. *(Ghi chú: T1.6 sẽ nối tạo tự động bản ghi `customers` + T4.1 tạo `wallets` qua exported interface hoặc tạo sẵn `customers` ở T1.5/T1.6 — thiết kế interface `OnCustomerRegistered` sạch ngay từ đây).*
  - `POST /api/v1/auth/login` (public, rate-limit theo IP + email): Kiểm tra user `status='active'`, verify Argon2id, khóa tạm khi sai nhiều lần (`failed_login_count`/`locked_until`), trả `200` gồm JWT access token (chứa `sub`, `kind`, `sid`, `auth_ver`, `exp` — **không** nhúng danh sách permission) và cookie `HttpOnly; SameSite=Lax` (`Secure` bật qua config) chứa refresh token ngẫu nhiên (DB chỉ lưu SHA-256 hash).
  - `POST /api/v1/auth/refresh` (cookie session): Xoay vòng refresh token cùng `family_id` (đánh dấu token cũ `revoked_at`, `replaced_by`). Nếu phát hiện token cũ đã bị xoay bị dùng lại (**reuse detection**) → thu hồi **toàn bộ `family_id`**, ghi audit cảnh báo và trả `401`.
  - `POST /api/v1/auth/logout` (authenticated): Thu hồi session hiện tại (hoặc tất cả session nếu truyền `all=true`) và xóa cookie.
- Auth middleware (`internal/platform/auth`): Verify chữ ký JWT, đọc `users.status` & `users.authorization_version` (có thể cache ngắn hạn có invalidate), gắn `Principal` vào `context.Context`.

#### Quy tắc & Invariant bắt buộc
- Lỗi đăng nhập luôn trả `401` chung chung (`invalid credentials`), không tiết lộ email tồn tại hay user đang bị khóa (chống account enumeration).
- User ở trạng thái `pending_activation` hoặc `disabled` không bao giờ đăng nhập hoặc gọi API thành công.
- JWT access token không lưu permission list tĩnh; thay đổi `authorization_version` phải vô hiệu hóa token cũ ngay lập tức.

#### Tiêu chí nghiệm thu & Kiểm thử
- Unit test Argon2id hash/verify, JWT sign/parse, chính sách mật khẩu.
- Integration & HTTP test: register thành công/trùng email (`409`), login đúng/sai (`401` đồng nhất), login khi `disabled`/`pending_activation` (`401`), refresh rotation thành công, dùng lại refresh token cũ thu hồi cả `family_id`, logout xong gọi lại refresh bị `401`.
- Chạy `cd BackEnd && go test -race ./internal/platform/auth/... ./internal/modules/identity/...`.

---

### T1.3 — Quên mật khẩu & Adapter gửi email SMTP qua Transactional Outbox (Mailpit / Gmail)

- **Commit gợi ý:** `feat(notification): add smtp outbox delivery and password reset flow`
- **Depends on:** `T1.2`, `T0.5`
- **Modules / Files:** `internal/platform/smtp/`, `internal/modules/notification/`, `internal/modules/identity/`, `BackEnd/api/openapi.yaml`

#### Mục tiêu nghiệp vụ
Cài đặt cơ chế gửi email thực tế qua SMTP (Mailpit ở local, tương thích Gmail SMTP qua env) kết nối với outbox worker, và hoàn thiện nghiệp vụ quên/đặt lại mật khẩu an toàn.

#### Phạm vi thực hiện
- `internal/platform/smtp`: Adapter gửi email SMTP chuẩn (hỗ trợ cả plain SMTP cho Mailpit local lẫn STARTTLS/Auth cho Gmail SMTP).
- `internal/modules/notification`:
  - Đăng ký các outbox event handler template email: `identity.password_reset_requested` (và khung sẵn cho `identity.employee_activation_requested`, `booking.confirmed`, `payment.late_credited`).
  - Render template HTML/text gọn gàng dùng `PUBLIC_BASE_URL` tạo link, không chứa mật khẩu/PII nhạy cảm.
- REST endpoints trong `identity`:
  - `POST /api/v1/auth/password-reset-requests` (public, rate-limit): Nếu email thuộc user `active`, tạo token ngẫu nhiên 32 byte, lưu SHA-256 hash vào `password_reset_tokens` (TTL 30 phút) và enqueue `identity.password_reset_requested` trong cùng transaction. Luôn trả `202 Accepted` kể cả khi email không tồn tại.
  - `POST /api/v1/auth/password-reset-confirm` (public): Lock dòng token theo hash (`FOR UPDATE`), kiểm tra chưa `used_at` và chưa hết hạn, cập nhật Argon2id `password_hash`, đánh dấu `used_at = now()`, thu hồi **toàn bộ `auth_sessions`** của user, tăng `authorization_version`, ghi audit — tất cả trong 1 transaction.

#### Quy tắc & Invariant bắt buộc
- Module nghiệp vụ (`identity`) tuyệt đối không gọi SMTP trực tiếp trong HTTP request; mọi email đi qua `outbox_events`.
- Reset token chỉ dùng **1 lần duy nhất**, lưu dạng hash, tự hết hạn; sau khi reset mọi phiên đăng nhập cũ phải bị vô hiệu hóa ngay.

#### Tiêu chí nghiệm thu & Kiểm thử
- Integration test: yêu cầu reset cho email tồn tại vs không tồn tại đều trả `202`; confirm hợp lệ đổi được mật khẩu và vô hiệu hóa session cũ; dùng lại token lần 2 hoặc token hết hạn trả `400`/`422`/`401` không đổi trạng thái.
- Kiểm chứng thủ công/test với Docker Mailpit: email reset xuất hiện trên `http://localhost:8025` chứa đúng link reset.
- Chạy `cd BackEnd && go test -race ./internal/modules/identity/... ./internal/modules/notification/...`.

---

### T1.4 — Dynamic RBAC Middleware, Quyền Admin Root bất biến & API quản trị Role / Permission / Gán quyền

- **Commit gợi ý:** `feat(rbac): implement dynamic authorization middleware, root bypass and rbac management apis`
- **Depends on:** `T1.2`
- **Modules / Files:** `internal/modules/rbac/{domain,application,delivery/http,persistence/postgres}`, `BackEnd/api/openapi.yaml`

#### Mục tiêu nghiệp vụ
Cung cấp tầng kiểm tra quyền động đọc từ PostgreSQL (với Admin root bypass viết cứng trong code) và đầy đủ API để quản trị viên tạo/sửa/xóa role, quản lý permission, gán quyền cho role và gán role cho nhân viên mà không thể tự khóa hệ thống hay leo thang đặc quyền.

#### Phạm vi thực hiện
- Exported interface `rbac.Authorizer`:
  - `Authorize(ctx context.Context, principal auth.Principal, permission string) error`
  - HTTP middleware `RequirePermission(perm string)` dùng `Authorizer`; kiểm tra lại trong các use case nhạy cảm.
  - **Root bypass trong code**: nếu user sở hữu role có `is_root = true` (và user `active`), `Authorize` luôn cho phép **kể cả với mọi permission được thêm mới trong tương lai** mà không cần gán tay vào `role_permissions`.
- REST endpoints:
  - `GET /api/v1/permissions` (`permission.read`), `POST /api/v1/permissions` / `PATCH /api/v1/permissions/{id}` (`permission.manage` — `code` bất biến sau khi tạo).
  - `GET /api/v1/roles` (`role.read`), `POST /api/v1/roles` (`role.create`), `PATCH /api/v1/roles/{id}` (`role.update`), `DELETE /api/v1/roles/{id}` (`role.delete` — chặn nếu `is_system`/`is_root` hoặc đang có user được gán, yêu cầu archive sau khi gỡ gán).
  - `PUT /api/v1/roles/{id}/permissions` (`role_permission.assign`): Cập nhật tập quyền của role.
  - `GET /api/v1/users` (`user.read`), `PUT /api/v1/users/{id}/roles` (`user_role.assign`): Gán/gỡ role cho user (chỉ áp dụng cho `staff`/`admin`, không gán role staff cho `customer`).

#### Quy tắc & Invariant bắt buộc
- **Không leo thang đặc quyền (Grantable-set check):** Caller không phải root chỉ được gán những permission nằm trong tập quyền hiện có của chính mình; không được sửa role có `is_system = true` nếu không có `role.update_system`.
- **Bảo vệ Root Admin cuối cùng:** Cấm xóa role `is_root`, cấm gỡ quyền khỏi role `is_root`, cấm gỡ role `is_root` khỏi hoặc vô hiệu hóa user Root Admin hoạt động cuối cùng trong hệ thống.
- Mọi thay đổi `role_permissions` hoặc `user_roles` phải tăng `users.authorization_version` cho các user bị ảnh hưởng và ghi `audit_events` (before/after + reason) trong cùng 1 transaction.

#### Tiêu chí nghiệm thu & Kiểm thử
- Mandatory negative RBAC test matrix:
  - Chưa đăng nhập → `401`; đăng nhập không đủ quyền → `403` và 0 thay đổi DB/audit nghiệp vụ.
  - Root admin truy cập được 1 permission mới hoàn toàn chưa từng gán trong `role_permissions`.
  - Cố gỡ root role khỏi admin cuối cùng → `409`.
  - User chỉ có quyền `booking.read` + `role_permission.assign` cố gán `payment.refund` cho role khác → `403`.
  - Sau khi gỡ role của user A, request tiếp theo với access token cũ của user A bị từ chối ngay do lệch `authorization_version`.
- Chạy `cd BackEnd && go test -race ./internal/modules/rbac/...`.

---

### T1.5 — Schema People (Migration 004), CRUD nhân viên & Luồng kích hoạt tài khoản nhân viên qua Email SMTP

- **Commit gợi ý:** `feat(people): add employee crud and atomic smtp email activation workflow`
- **Depends on:** `T1.3`, `T1.4`
- **Modules / Files:** `BackEnd/db/migration/000004_create_people.{up,down}.sql`, `BackEnd/db/query/people.sql`, `internal/modules/people/`, `internal/modules/identity/` (activation endpoint), `internal/modules/notification/` (activation email template), `BackEnd/api/openapi.yaml`

#### Mục tiêu nghiệp vụ
Quản lý hồ sơ nhân viên gắn liền với tài khoản nội bộ: khi Admin tạo nhân viên mới, hệ thống tự động tạo staff user ở trạng thái chờ kích hoạt và gửi email link kích hoạt để nhân viên tự đặt mật khẩu lần đầu.

#### Phạm vi thực hiện
- Migration `000004_create_people`:
  - `employees`: `id uuid PK`, `user_id uuid NN UQ FK -> users`, `employee_code varchar(32) NN UQ`, `full_name text NN`, `phone text NN`, `email citext NN UQ`, `position_title text NN`, `employment_status varchar(24) NN CHECK (employment_status IN ('active','inactive'))`, `archived_at timestamptz ?`, `created_at/updated_at/version`.
  - `customers`: `id uuid PK`, `user_id uuid ? UQ FK -> users`, `full_name text NN`, `phone text NN`, `email citext ?`, `nationality_code varchar(8) ?`, `government_id text ?`, `address text ?`, `source varchar(24) NN CHECK (source IN ('online','walk_in'))`, `archived_at timestamptz ?`, `created_at/updated_at/version`. Partial unique index cho `government_id` khi khác null và chưa archive.
- REST endpoints:
  - `POST /api/v1/admin/employees` (`employee.create`): **Không nhận trường mật khẩu từ admin.** Trong 1 PostgreSQL transaction: gọi exported interface của `identity` tạo `users` (`kind='staff', status='pending_activation'`) + `activation_tokens` (hash, TTL 24h) → gán role ban đầu (nếu truyền và thỏa grantable-set) → tạo dòng `employees` → enqueue `identity.employee_activation_requested` vào `outbox_events` → ghi `audit_events`.
  - `POST /api/v1/admin/employees/{id}/resend-activation` (`employee.update`): Phát lại token kích hoạt mới (hủy hiệu lực token cũ) nếu user vẫn `pending_activation`.
  - `POST /api/v1/auth/activate` (public): Nhận `token` + `password` mới; lock `activation_tokens` (`FOR UPDATE`), kiểm tra hợp lệ/chưa dùng/chưa hết hạn, lưu `user_credentials` Argon2id, chuyển `users.status = 'active'`, đánh dấu token `used_at = now()`, ghi audit, và cấp session đăng nhập.
  - `GET /api/v1/admin/employees`, `GET/PATCH /api/v1/admin/employees/{id}` (`employee.read`, `employee.update` — dùng `version` optimistic lock), `POST /api/v1/admin/employees/{id}/deactivate` (`employee.deactivate` — vô hiệu hóa đồng thời `employees.employment_status='inactive'`, `users.status='disabled'`, thu hồi mọi session, chặn nếu là root admin cuối cùng).

#### Quy tắc & Invariant bắt buộc
- Mọi nhân viên bắt buộc có 1 `users` liên kết (`user_id NOT NULL`).
- Nếu SMTP lỗi khi gửi mail kích hoạt, giao dịch tạo nhân viên vẫn giữ nguyên ở trạng thái `pending_activation` và outbox tự retry (hoặc admin bấm gửi lại).
- Token kích hoạt chỉ dùng 1 lần, lưu hash; không thể dùng để kích hoạt lại user đã `active` hoặc `disabled`.

#### Tiêu chí nghiệm thu & Kiểm thử
- Integration test luồng E2E: Admin tạo nhân viên → kiểm tra user ở trạng thái `pending_activation` không login được → outbox gửi email kích hoạt → gọi `POST /api/v1/auth/activate` đặt mật khẩu → login thành công bằng mật khẩu vừa đặt; dùng lại token kích hoạt trả lỗi; deactivate nhân viên lập tức khóa tài khoản và thu hồi session.
- Chạy `cd BackEnd && go test -race ./internal/modules/people/... ./internal/modules/identity/...`.

---

### T1.6 — Quản lý hồ sơ khách hàng (Staff CRUD) & Tự quản lý tài khoản `/me`

- **Commit gợi ý:** `feat(people): add customer profile management and self-service me endpoints`
- **Depends on:** `T1.5`
- **Modules / Files:** `internal/modules/people/`, `internal/modules/identity/`, `BackEnd/api/openapi.yaml`

#### Mục tiêu nghiệp vụ
Hoàn thiện quản lý hồ sơ khách hàng cho cả khách đặt online (gắn với `user_id`) lẫn khách walk-in do lễ tân tạo (có thể chưa có `user_id`), đồng thời nối tự động bước tạo hồ sơ khách hàng khi đăng ký ở `T1.2` và cung cấp cụm API `/me`.

#### Phạm vi thực hiện
- Nối hook trong `POST /api/v1/auth/register` (`T1.2`): trong cùng transaction đăng ký tài khoản khách hàng, gọi exported interface của `people` tạo bản ghi `customers` (`user_id`, `source='online'`).
- Staff Customer APIs:
  - `GET /api/v1/customers` (`customer.read`, hỗ trợ tìm theo tên/SĐT/email, cursor pagination; che bớt `government_id` trong danh sách).
  - `POST /api/v1/customers` (`customer.create` — cho phép lễ tân tạo khách walk-in `user_id = NULL`, `source = 'walk_in'`).
  - `GET /api/v1/customers/{id}`, `PATCH /api/v1/customers/{id}` (`customer.read`, `customer.update` với `version` optimistic lock).
  - `POST /api/v1/customers/{id}/archive` (`customer.archive` — archive thay vì xóa cứng; chuẩn bị hook kiểm tra booking active ở Phase 3).
- Customer/User Self-Service APIs:
  - `GET /api/v1/me` & `PATCH /api/v1/me` (authenticated self): xem và cập nhật thông tin cá nhân của chính mình (email đăng nhập bất biến trong phạm vi đồ án để tránh phức tạp xác minh lại).
  - `POST /api/v1/me/password` (authenticated self): kiểm tra mật khẩu cũ, đổi sang Argon2id mới, thu hồi các session khác.
  - `GET /api/v1/me/sessions` & `DELETE /api/v1/me/sessions/{id}` (authenticated self): xem và thu hồi phiên đăng nhập của chính mình.

#### Quy tắc & Invariant bắt buộc
- Khách hàng A tuyệt đối không thể đọc/sửa hồ sơ hoặc session của khách hàng B (contextual ownership guard).
- Không log `government_id` nguyên văn ra structured log hoặc `audit_events`.

#### Tiêu chí nghiệm thu & Kiểm thử
- Integration test: khách hàng register tạo đủ `users` + `customers`; lễ tân tạo được khách walk-in không có `user_id`; `/me` đổi mật khẩu thu hồi session khác; kiểm tra negative ownership (customer A truy cập/xóa session của customer B trả `403`/`404`).
- Chạy `cd BackEnd && go test -race ./internal/modules/people/... ./internal/modules/identity/...`.
