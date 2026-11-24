# Phase 0 — Nền tảng kỹ thuật (Foundation)

> **Mục tiêu Phase:** Dựng khung Go modular monolith sạch sẽ, môi trường Docker Compose local (PostgreSQL, Redis, MinIO, Mailpit), tầng platform chung (`config`, `db`, `http`, `logging`, `audit`, `outbox`) và kiểm tra ranh giới module tự động. Chưa triển khai nghiệp vụ.
>
> **Phụ thuộc:** Không có (Phase khởi đầu).
>
> **Tiêu chí hoàn thành Phase (Exit Criteria):**
> - `docker compose up -d` khởi chạy thành công PostgreSQL 16, Redis 7, MinIO, Mailpit.
> - `go run ./cmd/api` khởi động sạch, `/healthz` và `/readyz` trả `200 OK`, nhấn `Ctrl+C` thực hiện graceful shutdown không leak goroutine.
> - Migrations `000001`–`000002` chạy `up` thành công trên PostgreSQL trống; `sqlc generate` ra code sạch không lỗi.
> - `go test -race ./...` xanh hoàn toàn (bao gồm test kiểm tra ranh giới import giữa các module và Testcontainers PostgreSQL cho Audit/Outbox).

---

### T0.1 — Khởi tạo Go module, cấu trúc thư mục và hạ tầng Docker Compose local

- **Commit gợi ý:** `chore(foundation): initialize go module, package layout and local docker stack`
- **Depends on:** Không có
- **Modules / Files:** `BackEnd/go.mod`, `BackEnd/Makefile`, `BackEnd/.env.example`, `BackEnd/docker-compose.yml`, `BackEnd/sqlc.yaml`, `BackEnd/cmd/api/main.go`, `BackEnd/internal/platform/`, `BackEnd/internal/modules/`, `BackEnd/db/{migration,query,seed}/`, `BackEnd/api/`

#### Mục tiêu nghiệp vụ
Thiết lập khung dự án Go modular monolith và toàn bộ dịch vụ hạ tầng chạy cục bộ để mọi task sau có thể code, migrate và chạy test ngay lập tức.

#### Phạm vi thực hiện
- Khởi tạo `BackEnd/go.mod` (Go 1.23+) với cấu trúc thư mục theo `package-structure.md`:
  - `cmd/api/main.go` (composition root), `cmd/bootstrap/main.go` (chỗ trống cho CLI bootstrap/seed sau này).
  - `internal/platform/{config,db,http,auth,logging,clock,id,redis,minio,smtp}`.
  - `internal/modules/{identity,rbac,people,property,media,pricing,booking,wallet,stayops,notification,reporting,audit}` (placeholder `.gitkeep` hoặc package stub tối thiểu).
  - `db/{migration,query,seed}`, `api/`.
- Tạo `BackEnd/docker-compose.yml` chạy 4 service cho đồ án:
  - `postgres:16-alpine` (healthcheck `pg_isready`)
  - `redis:7-alpine` (healthcheck `redis-cli ping`)
  - `minio/minio` (API port `9000`, Console port `9001`, tự tạo bucket mặc định qua init container `minio/mc`)
  - `axllent/mailpit` (SMTP port `1025`, Web UI xem email port `8025`)
- Tạo `BackEnd/.env.example` liệt kê đủ biến môi trường: `DATABASE_URL`, `REDIS_URL`, `MINIO_*`, `SEPAY_WEBHOOK_SECRET`, `SEPAY_RECEIVING_ACCOUNT`, `SMTP_HOST/PORT/USER/PASS/FROM`, `JWT_ACCESS_SECRET`, `JWT_REFRESH_SECRET`, `PUBLIC_BASE_URL`, `HOTEL_TIMEZONE` (`Asia/Ho_Chi_Minh`), `UPLOAD_MAX_BYTES`.
- Tạo `BackEnd/sqlc.yaml` (cấu hình `pgx/v5`, `emit_interface: false`, map `uuid` → `github.com/google/uuid`, `numeric` → `shopspring/decimal` hoặc `pgtype.Numeric` thống nhất toàn dự án — khuyến nghị `shopspring/decimal` cho money).
- Tạo `BackEnd/Makefile` với các target cơ bản: `up`, `down`, `migrate-up`, `migrate-down`, `sqlc`, `test`, `run`.

#### Quy tắc & Invariant bắt buộc
- Không commit file `.env` thật hay bất kỳ secret cứng nào trong repo.
- Mọi giá trị tiền tệ (`numeric(14,2)`) phải được biểu diễn bằng kiểu decimal chính xác, tuyệt đối không dùng `float64`.

#### Tiêu chí nghiệm thu & Kiểm thử
- `docker compose -f BackEnd/docker-compose.yml up -d` tất cả 4 container ở trạng thái `healthy`.
- `cd BackEnd && go build ./cmd/api` biên dịch thành công.

---

### T0.2 — Nạp cấu hình typed env, structured logging, health/readiness và graceful shutdown

- **Commit gợi ý:** `feat(platform): add typed config, slog logger, health probes and graceful shutdown`
- **Depends on:** `T0.1`
- **Modules / Files:** `internal/platform/config/`, `internal/platform/logging/`, `internal/platform/clock/`, `internal/platform/id/`, `cmd/api/main.go`

#### Mục tiêu nghiệp vụ
Đảm bảo service khởi động fail-fast khi thiếu cấu hình bắt buộc, log JSON có cấu trúc và che dữ liệu nhạy cảm, đồng thời tắt sạch khi nhận tín hiệu dừng.

#### Phạm vi thực hiện
- `internal/platform/config`: Parse env vào struct có kiểu rõ ràng (`Config`), validate fail-fast các biến bắt buộc (`DATABASE_URL`, `REDIS_URL`, `JWT_ACCESS_SECRET`, `JWT_REFRESH_SECRET`, `SEPAY_WEBHOOK_SECRET`, v.v.); hỗ trợ chế độ `APP_ENV=development|test|production`.
- `internal/platform/logging`: Khởi tạo `log/slog` JSON handler kèm helper redact (che `password`, `token`, `secret`, `authorization`, `government_id`, `bank_description`).
- `internal/platform/clock` & `internal/platform/id`: Interface `Clock` (`Now() time.Time`) và `IDGenerator` (`NewUUID() uuid.UUID`) để unit test có thể giả lập thời gian/UUID xác định.
- Endpoint probe:
  - `GET /healthz`: luôn trả `200 {"status":"ok"}` khi process đang sống.
  - `GET /readyz`: chạy healthcheck kết nối PostgreSQL và Redis (và MinIO nếu bật), trả `200` khi sẵn sàng hoặc `503` kèm tên dependency lỗi.
- `cmd/api/main.go`: Bắt `SIGINT`/`SIGTERM`, ngừng nhận HTTP request mới, drain request đang chạy trong timeout cấu hình, dừng background worker, đóng DB/Redis pool.

#### Quy tắc & Invariant bắt buộc
- Thiếu secret bắt buộc ở startup phải dừng process với lỗi rõ ràng, không tự dùng fallback rỗng.
- Không bao giờ in mật khẩu, JWT secret, token kích hoạt/reset hay webhook secret ra log.

#### Tiêu chí nghiệm thu & Kiểm thử
- Unit test cho `config.Load()` (thiếu biến bắt buộc trả lỗi; đủ biến trả struct đúng giá trị mặc định) và `logging` redaction.
- Integration test gọi `GET /healthz` (`200`) và `GET /readyz` (`200` khi dependency sẵn sàng, `503` khi mock checker fail).
- Chạy `cd BackEnd && go test -race ./internal/platform/...`.

---

### T0.3 — HTTP Router (`chi`), chuẩn Error Envelope, Middleware nền, OpenAPI 3.1 khung và kiểm tra ranh giới module

- **Commit gợi ý:** `feat(http): add chi router, error envelope, base middlewares, openapi skeleton and boundary lint`
- **Depends on:** `T0.2`
- **Modules / Files:** `internal/platform/http/`, `BackEnd/api/openapi.yaml`, `internal/arch_test.go`

#### Mục tiêu nghiệp vụ
Thống nhất giao tiếp REST toàn hệ thống (error envelope, phân trang cursor, bảo vệ request cơ bản, OpenAPI 3.1 khung) và khóa chặt kiến trúc modular monolith bằng test tự động.

#### Phạm vi thực hiện
- `internal/platform/http`:
  - Khởi tạo `chi.NewRouter()` gắn base prefix `/api/v1`.
  - Middleware: `RequestID` / `CorrelationID` (propagated trong `context.Context` và header `X-Request-Id`), `PanicRecovery` (log stack + trả `500`), `StructuredRequestLogger`, `CORS` (exact allowlist từ config), `BodyLimit` (ví dụ 1 MiB mặc định), `RateLimit` in-memory/Redis interface cơ bản (dùng cho login/reset/hold/webhook sau này).
  - Error encoder chuẩn `{ "error": { "code": "...", "message": "...", "fields": [...], "requestId": "..." } }` ánh xạ domain error sang HTTP status: `422` (`validation_failed`), `401` (`unauthenticated`), `403` (`forbidden`), `404` (`not_found`), `409` (`conflict` / `version_conflict`), `429` (`rate_limited`), `503` (`service_unavailable`), `500` (`internal_error`).
  - Helper JSON decode (`DisallowUnknownFields` cho write DTO), cursor pagination envelope `{ "data": [...], "page": { "limit": ..., "cursor": ..., "nextCursor": ... } }`, helper đọc `Idempotency-Key` và `If-Match`.
- `BackEnd/api/openapi.yaml`: Khởi tạo khung OpenAPI 3.1 với `info`, `servers`, `components.securitySchemes` (Bearer JWT), `components.schemas.ErrorResponse`, `PageInfo`, và đường dẫn `/healthz`, `/readyz`.
- `internal/arch_test.go`: Viết Go test dùng `go/packages` (hoặc `go list`) kiểm tra luật phụ thuộc:
  - `domain` không được import `net/http`, `chi`, `pgx`, hay `persistence`.
  - Module A không được import `internal/modules/<B>/persistence` hoặc `domain` nội bộ của module B (chỉ được import `application` đã export).
  - `delivery/http` không được import `persistence`.

#### Quy tắc & Invariant bắt buộc
- Mọi lỗi API trả về client phải tuân thủ đúng envelope `{error:{code,message,fields,requestId}}`, không lộ SQL error hay stack trace ra response `500`.
- Kiến trúc ranh giới module được bảo vệ tự động mỗi lần chạy `go test`.

#### Tiêu chí nghiệm thu & Kiểm thử
- Unit test cho error mapper (đủ 401/403/404/409/422/429/500/503), panic recovery, body-limit rejection (`413`/`422`), JSON unknown field rejection.
- `arch_test.go` pass khi đúng ranh giới (và đã kiểm chứng fail khi thử vi phạm).
- Chạy `cd BackEnd && go test -race ./...`.

---

### T0.4 — PostgreSQL Pool (`pgxpool`), Transaction Runner chung và Migration 001 (Extensions)

- **Commit gợi ý:** `feat(db): add pgxpool, shared transaction runner and base extensions migration`
- **Depends on:** `T0.2`
- **Modules / Files:** `internal/platform/db/`, `BackEnd/db/migration/000001_enable_extensions.{up,down}.sql`, `internal/platform/testutil/`

#### Mục tiêu nghiệp vụ
Cung cấp kết nối PostgreSQL, cơ chế chạy chung một PostgreSQL transaction xuyên nhiều module (theo đúng thiết kế modular monolith) và tiện ích Testcontainers dùng chung cho mọi integration test.

#### Phạm vi thực hiện
- `internal/platform/db`:
  - Khởi tạo `pgxpool.Pool` với cấu hình timeout, max conns, health ping.
  - `TxRunner` interface (`WithinTx(ctx context.Context, fn func(ctx context.Context) error) error`): đặt `pgx.Tx` vào `context.Context` để các repository của nhiều module tham gia cùng 1 local transaction `READ COMMITTED` khi gọi qua application interface; tự rollback khi `fn` trả error hoặc panic; hỗ trợ tái sử dụng tx nếu đã ở trong tx.
  - Helper phát hiện PostgreSQL error code (`23505` unique violation, `23P01` exclusion violation, `40001` serialization failure, `40P01` deadlock) chuyển về domain error có nghĩa.
- Migration `000001_enable_extensions.up.sql` / `.down.sql`:
  - Bật `pgcrypto`, `citext`, `btree_gist`.
- `internal/platform/testutil`: Helper khởi tạo Testcontainers PostgreSQL (chạy tự động toàn bộ `db/migration/*.up.sql`) và Redis container dùng lại cho các phase sau.

#### Quy tắc & Invariant bắt buộc
- Mọi cross-module command (tạo nhân viên, checkout, thanh toán) phải chạy trong 1 PostgreSQL transaction qua `TxRunner`, không mở transaction lồng rời rạc làm mất tính nguyên tử.
- Không auto-migrate âm thầm ở production; migration chạy qua lệnh `migrate` rõ ràng (chỉ auto-migrate trong Testcontainers test helper).

#### Tiêu chí nghiệm thu & Kiểm thử
- Integration test (Testcontainers PostgreSQL): chạy Migration 001 thành công, verify extension `pgcrypto`, `citext`, `btree_gist` đã bật; verify `WithinTx` commit khi thành công và rollback sạch khi trả lỗi hoặc panic.
- Chạy `cd BackEnd && go test -race ./internal/platform/db/...`.

---

### T0.5 — Audit append-only & Transactional Outbox primitives (Migration 002)

- **Commit gợi ý:** `feat(audit): add append-only audit events, transactional outbox and worker skeleton`
- **Depends on:** `T0.3`, `T0.4`
- **Modules / Files:** `BackEnd/db/migration/000002_create_audit_outbox.{up,down}.sql`, `BackEnd/db/query/audit.sql`, `internal/modules/audit/`, `internal/modules/notification/` (dispatcher registry skeleton)

#### Mục tiêu nghiệp vụ
Cung cấp 2 dịch vụ nền tảng xuyên suốt toàn bộ hệ thống: ghi nhật ký kiểm toán bất biến (`audit_events`) và hàng đợi sự kiện trong cùng transaction (`outbox_events`) kèm background worker xử lý bất đồng bộ có retry.

#### Phạm vi thực hiện
- Migration `000002_create_audit_outbox`:
  - `audit_events`: `id uuid PK`, `occurred_at timestamptz NN default now()`, `actor_user_id uuid ?`, `action varchar(100) NN`, `resource_type varchar(100) NN`, `resource_id uuid ?`, `request_id uuid ?`, `correlation_id uuid ?`, `ip_hash bytea ?`, `reason text ?`, `before_data jsonb ?`, `after_data jsonb ?`. Index BRIN theo `occurred_at` và B-tree theo `(resource_type, resource_id)`.
  - `outbox_events`: `id uuid PK`, `aggregate_type varchar(100) NN`, `aggregate_id uuid NN`, `event_type varchar(100) NN`, `payload jsonb NN`, `occurred_at timestamptz NN default now()`, `available_at timestamptz NN default now()`, `processed_at timestamptz ?`, `attempts int NN default 0`, `max_attempts int NN default 5`, `last_error text ?`. Index partial `(available_at) WHERE processed_at IS NULL`.
- Exported interfaces:
  - `audit.Recorder` (`Record(ctx, AuditEntry) error`) — tự lấy tx từ `ctx` qua `db.TxRunner` để commit/rollback cùng lệnh nghiệp vụ, tự redact field nhạy cảm trong `before_data`/`after_data`.
  - `audit.OutboxEnqueuer` (`Enqueue(ctx, OutboxEvent) error`) — chèn sự kiện vào `outbox_events` trong cùng DB transaction.
- Background Outbox Worker (`internal/modules/notification/worker.go`):
  - Poll định kỳ theo interval cấu hình, claim batch bằng `SELECT ... FROM outbox_events WHERE processed_at IS NULL AND available_at <= now() AND attempts < max_attempts ORDER BY available_at LIMIT $1 FOR UPDATE SKIP LOCKED`.
  - Dispatch tới handler đăng ký theo `event_type`; thành công → set `processed_at = now()`; thất bại → tăng `attempts`, ghi `last_error`, lùi `available_at` theo exponential backoff; vượt `max_attempts` → log lỗi `dead_letter` (không chặn các event khác).

#### Quy tắc & Invariant bắt buộc
- `audit_events` là **append-only**: không tạo API/query `UPDATE` hay `DELETE` cho bảng này.
- Nếu transaction nghiệp vụ rollback, bản ghi `audit_events` và `outbox_events` phải rollback theo (không bao giờ phát email/event cho giao dịch thất bại).

#### Tiêu chí nghiệm thu & Kiểm thử
- Integration test (Testcontainers PostgreSQL):
  - Ghi audit + enqueue outbox trong `WithinTx` → rollback → xác nhận 0 dòng được lưu.
  - Commit thành công → outbox worker claim và dispatch 1 lần, đánh dấu `processed_at`.
  - Handler giả lập lỗi → worker tăng `attempts` và đẩy lùi `available_at` theo backoff; 2 worker chạy song song không xử lý trùng cùng 1 event (`FOR UPDATE SKIP LOCKED`).
- Chạy `cd BackEnd && go test -race ./internal/modules/audit/... ./internal/modules/notification/...`.
