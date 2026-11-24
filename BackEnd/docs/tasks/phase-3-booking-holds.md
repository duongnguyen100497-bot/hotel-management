# Phase 3 — Giữ phòng Redis nguyên tử & Đặt phòng Lẻ / Nhóm / Hủy hàng loạt (Booking & Holds)

> **Mục tiêu Phase:** Xây dựng lõi đặt phòng chống trùng phòng tuyệt đối bằng 2 lớp bảo vệ (Redis Lua hold nguyên tử + PostgreSQL `btree_gist` exclusion constraint), tách bạch Customer (người đặt/trả tiền) và Representative Guest snapshot (người lưu trú), hỗ trợ đặt lẻ, đặt nhóm nhiều phòng (aggregate status), hủy lẻ và hủy hàng loạt all-or-nothing.
>
> **Phụ thuộc:** Hoàn thành Phase 2 (`T2.1`–`T2.3`) và `T1.6`.
>
> **Tiêu chí hoàn thành Phase (Exit Criteria):**
> - Tạo hold lẻ (dùng `individual_hold_ttl_minutes` cấu hình từ `hotel_settings`) và hold nhóm (toàn bộ phòng con kế thừa `group_hold_ttl_minutes`) nguyên tử qua Redis Lua; Redis tắt → trả `503` fail-closed.
> - Test tranh chấp đồng thời (concurrent goroutines) chứng minh 0 trường hợp double-booking ở cả tầng Redis hold lẫn tầng PostgreSQL exclusion constraint.
> - Tạo booking lẻ & booking nhóm từ hold còn hạn, chụp đầy đủ guest snapshot + VAT/price snapshot, có idempotency key.
> - Job quét hết hạn DB + reconciler Redis tự động chuyển booking quá hạn sang `expired` và phục hồi trạng thái sau khi Redis khởi động lại.
> - Hủy lẻ 1 phòng con trong nhóm chỉ hủy phòng đó và cập nhật trạng thái tổng nhóm; `POST /api/v1/bookings/batch/cancel` đảm bảo all-or-nothing.

---

### T3.1 — Schema Booking, Group, Guest Snapshot & Holds (Migration 009) + Engine kiểm tra phòng trống & báo giá

- **Commit gợi ý:** `feat(booking): add booking, holds, guest snapshot schema with gist exclusion and quote engine`
- **Depends on:** `T2.1`, `T2.2`, `T1.6`
- **Modules / Files:** `BackEnd/db/migration/000009_create_bookings_holds.{up,down}.sql`, `BackEnd/db/query/booking.sql`, `internal/modules/booking/`, `BackEnd/api/openapi.yaml`

#### Mục tiêu nghiệp vụ
Định nghĩa lược đồ đặt phòng kèm ràng buộc PostgreSQL chống trùng lịch ở tầng database (`btree_gist`) và cung cấp API báo giá + kiểm tra phòng trống chính xác theo ngày lưu trú.

#### Phạm vi thực hiện
- Migration `000009_create_bookings_holds`:
  - `booking_groups`: `id uuid PK`, `reference_code varchar(32) NN UQ`, `customer_id uuid NN FK -> customers`, `status varchar(24) NN CHECK (status IN ('holding','pending','partially_confirmed','confirmed','partially_cancelled','cancelled','expired','completed'))`, `expires_at timestamptz ?`, `idempotency_key varchar(128) ?`, `created_at/updated_at/version`. Partial unique index `(customer_id, idempotency_key) WHERE idempotency_key IS NOT NULL`.
  - `bookings`: `id uuid PK`, `reference_code varchar(32) NN UQ`, `booking_group_id uuid ? FK -> booking_groups`, `customer_id uuid NN FK -> customers`, `room_id uuid NN FK -> rooms`, `room_type_id uuid NN FK -> room_types`, `stay daterange NN CHECK (lower(stay) < upper(stay) AND lower_inc(stay) AND NOT upper_inc(stay))`, `adults smallint NN CHECK (adults > 0)`, `children smallint NN default 0 CHECK (children >= 0)`, `status varchar(24) NN CHECK (status IN ('pending','confirmed','checked_in','checked_out','cancelled','expired'))`, `nightly_rate_snapshot numeric(14,2) NN`, `vat_rate_id uuid NN FK -> vat_rates`, `vat_rate_snapshot numeric(5,4) NN`, `subtotal_amount numeric(14,2) NN`, `vat_amount numeric(14,2) NN`, `total_amount numeric(14,2) NN`, `currency char(3) NN default 'VND'`, `special_request text ?`, `expires_at timestamptz ?`, `cancelled_at timestamptz ?`, `cancel_reason text ?`, `idempotency_key varchar(128) ?`, `created_at/updated_at/version`.
  - **Ràng buộc chống double booking:**
    ```sql
    ALTER TABLE bookings ADD CONSTRAINT bookings_no_room_overlap
      EXCLUDE USING gist (room_id WITH =, stay WITH &&)
      WHERE (status IN ('pending', 'confirmed', 'checked_in'));
    ```
  - `booking_guest_snapshots`: `booking_id uuid PK FK -> bookings`, `full_name text NN`, `phone text NN`, `email text ?`, `nationality_code varchar(8) ?`, `special_request text ?`, `created_at timestamptz NN default now()` (bất biến sau khi tạo).
  - `booking_status_history`: `id uuid PK`, `booking_id uuid NN FK -> bookings`, `from_status varchar(24) ?`, `to_status varchar(24) NN`, `actor_user_id uuid ?`, `reason text ?`, `correlation_id uuid ?`, `occurred_at timestamptz NN default now()`.
  - `reservation_holds`: `id uuid PK`, `customer_id uuid NN FK -> customers`, `booking_group_id uuid ?`, `room_id uuid NN FK -> rooms`, `room_type_id uuid NN FK -> room_types`, `stay daterange NN`, `hold_mode varchar(16) NN CHECK (hold_mode IN ('individual','group'))`, `hold_token varchar(64) NN`, `redis_key text NN`, `ttl_seconds int NN`, `status varchar(24) NN CHECK (status IN ('active','converted','released','expired'))`, `expires_at timestamptz NN`, `released_reason text ?`, `created_at/updated_at`. Index `(status, expires_at)`.
- REST endpoint:
  - `POST /api/v1/availability/quote` (public): Nhận `checkIn`, `checkOut`, danh sách `{roomTypeId, roomCount}`, `adults`, `children`; validate khoảng ngày (`checkIn >= today`, `checkOut > checkIn`), sức chứa phòng, loại trừ phòng đang `maintenance`, phòng có `bookings` active chồng lấn và phòng có `reservation_holds` đang `active` chưa hết hạn; gọi `pricing.QuoteCalculator` trả báo giá chi tiết (chưa VAT, tiền VAT theo rate active, tổng tiền) mà **chưa giữ chỗ**.
- Nối thêm query lọc phòng trống vào `GET /api/v1/catalog/room-types` khi client truyền `checkIn` & `checkOut`.

#### Quy tắc & Invariant bắt buộc
- Khoảng ngày lưu trú luôn theo quy ước nửa mở `[check_in, check_out)` để khách trả phòng ngày X và khách mới nhận phòng ngày X không bị coi là chồng lấn.
- `bookings_no_room_overlap` bắt buộc nằm ở tầng PostgreSQL, không chỉ dựa vào kiểm tra ở tầng ứng dụng.

#### Tiêu chí nghiệm thu & Kiểm thử
- Integration test (Testcontainers PostgreSQL):
  - Thử insert đồng thời 2 booking chồng ngày cho cùng 1 `room_id` ở trạng thái `pending`/`confirmed`/`checked_in` → PostgreSQL bắt buộc reject 1 lệnh bằng exclusion violation (`23P01`).
  - Booking cũ `cancelled`/`expired`/`checked_out`, hoặc booking mới check-in đúng ngày check-out của booking cũ → insert thành công.
  - Test `POST /api/v1/availability/quote` tính đúng số đêm × giá + VAT active và loại trừ phòng đang bảo trì.
- Chạy `cd BackEnd && go test -race ./internal/modules/booking/...`.

---

### T3.2 — Giữ phòng nguyên tử bằng Redis Lua Script (`POST /api/v1/holds`, `DELETE /api/v1/holds/{id}`)

- **Commit gợi ý:** `feat(booking): add atomic redis lua room and group hold acquisition and release`
- **Depends on:** `T3.1`
- **Modules / Files:** `internal/platform/redis/`, `internal/modules/booking/{domain,application,delivery/http,persistence/redis}`, `BackEnd/api/openapi.yaml`

#### Mục tiêu nghiệp vụ
Cài đặt cơ chế giữ phòng tạm thời nguyên tử qua Redis Lua script (cho cả đặt lẻ và đặt nhóm), dùng TTL lấy động từ `hotel_settings` do Admin tùy chỉnh, và fail-closed an toàn khi Redis gặp sự cố.

#### Phạm vi thực hiện
- `internal/platform/redis`: Client `go-redis/v9` kèm healthcheck và Lua script runner.
- Hai Redis Lua script nguyên tử:
  1. **Acquire script**: Kiểm tra đồng thời toàn bộ danh sách key phòng `hold:room:{roomId}:{YYYY-MM-DD}:{YYYY-MM-DD}` (và key `hold:group:{groupId}` nếu là group hold) đều chưa tồn tại; nếu tất cả trống → `SET ... NX EX <ttl>` cùng 1 `hold_token` ngẫu nhiên; nếu bất kỳ key nào đã tồn tại → không set key nào và trả về lỗi xung đột.
  2. **Release script**: Chỉ `DEL` key nếu giá trị hiện tại khớp đúng `hold_token` (tránh xóa nhầm hold của người khác).
- Quy trình tạo hold (`POST /api/v1/holds`, authenticated customer hoặc staff `booking.create`):
  1. Đọc `individual_hold_ttl_minutes` / `group_hold_ttl_minutes` hiện hành từ `hotel_settings`. **Quy tắc đã chốt:** đặt lẻ dùng `individual_hold_ttl_minutes` (mặc định 15 phút); đặt nhóm dùng `group_hold_ttl_minutes` (mặc định 30 phút) cho **cả key group lẫn toàn bộ key phòng con** và `expires_at` dưới DB.
  2. Mở 1 ngắn transaction PostgreSQL chọn các phòng trống ứng viên theo thứ tự UUID xác định với `FOR UPDATE SKIP LOCKED`, ghi `reservation_holds` (`status='active'`, `hold_token`, `ttl_seconds`, `expires_at`).
  3. Gọi Redis Acquire Lua script. Nếu Redis không khả dụng → đánh dấu hold DB thất bại/rollback và trả **`503 Service Unavailable`** (fail-closed). Nếu trùng key Redis → thử lại tối đa N lần với ứng viên phòng khác hoặc trả `409 Conflict` (hết phòng).
  4. Trả về `holdId` (hoặc `groupHoldId`), `holdToken`, `expiresAt`, danh sách phòng đã giữ và báo giá tạm tính.
- Hủy hold thủ công (`DELETE /api/v1/holds/{id}`, chủ sở hữu hold hoặc staff): Kiểm tra quyền sở hữu + `hold_token`, cập nhật `reservation_holds.status = 'released'` trong DB trước, sau đó gọi Redis Release Lua script (lỗi Redis được reconciler ở `T3.3` xử lý bù).

#### Quy tắc & Invariant bắt buộc
- Key Redis **tuyệt đối không chứa PII** (chỉ lưu `hold_token`/reference ID).
- Không có API gia hạn hold thủ công; hết hạn thì phải tạo hold mới.
- Nếu Redis ngưng hoạt động, hệ thống **bắt buộc fail-closed (`503`)**, không bao giờ giả vờ giữ phòng thành công.

#### Tiêu chí nghiệm thu & Kiểm thử
- Integration test (Testcontainers PostgreSQL + Redis):
  - 10 goroutine cùng yêu cầu giữ 1 phòng cuối cùng → đúng 1 goroutine thành công, 9 goroutine còn lại nhận `409`.
  - Group hold 3 phòng: toàn bộ 3 key phòng + key nhóm đều có cùng TTL = `group_hold_ttl_minutes`; nếu 1 trong 3 phòng bị giữ trước → 0 key nào được tạo (all-or-nothing).
  - Đổi `individual_hold_ttl_minutes` trong `hotel_settings` từ 15 sang 25 → hold mới nhận 25 phút, hold cũ giữ nguyên `expires_at` ban đầu.
  - Giả lập Redis lỗi kết nối → API trả `503`.
- Chạy `cd BackEnd && go test -race ./internal/modules/booking/...`.

---

### T3.3 — Worker quét hết hạn Hold trên PostgreSQL & Reconciler đồng bộ Redis

- **Commit gợi ý:** `feat(booking): add authoritative db hold expiry scanner and redis reconciler worker`
- **Depends on:** `T3.2`, `T0.5`
- **Modules / Files:** `internal/modules/booking/application/reconciler.go`, `cmd/api/main.go`

#### Mục tiêu nghiệp vụ
Đảm bảo PostgreSQL luôn là nguồn sự thật cuối cùng cho trạng thái hết hạn hold/booking `pending`, đồng thời tự động phục hồi đồng bộ 2 chiều giữa DB và Redis khi gặp sự cố crash giữa chừng hoặc Redis restart.

#### Phạm vi thực hiện
- **Authoritative DB Expiry Scanner** (chạy nền mỗi 10–15 giây):
  - Tìm các `reservation_holds` có `status = 'active'` và `expires_at <= now()` (và các `bookings` có `status = 'pending'` và `expires_at <= now()`).
  - Khóa từng nhóm/dòng theo thứ tự UUID (`FOR UPDATE SKIP LOCKED`), đánh dấu `reservation_holds.status = 'expired'`, chuyển các `bookings` vẫn đang `pending` sang `expired`, ghi `booking_status_history` + `audit_events`, gọi hook hủy payment intent / giải phóng wallet hold (nối ở Phase 4), recompute trạng thái `booking_groups` nếu có, rồi sau commit gọi Redis Release Lua script xóa key khớp token.
- **Bidirectional Redis Reconciliation Worker** (chạy khi khởi động và định kỳ mỗi 1–2 phút):
  - *DB → Redis (sau khi Redis restart):* Tìm tất cả `reservation_holds` đang `active` và `expires_at > now()`, khôi phục lại key vào Redis với TTL còn lại `remaining = expires_at - now()`. Nếu phát hiện va chạm key không thể khôi phục → bản ghi DB commit sớm hơn thắng, bản ghi xung đột bị đánh dấu `expired`.
  - *Redis → DB (dọn key mồ côi):* Quét các key `hold:room:*` / `hold:group:*` không còn tương ứng với hold `active` trong DB → xóa an toàn bằng Release Lua script (khớp token).

#### Quy tắc & Invariant bắt buộc
- Redis hết hạn key tự động **không bao giờ** tự thay đổi trạng thái nghiệp vụ; chỉ DB Expiry Scanner dưới row lock mới được chuyển `pending → expired`.
- Booking đã `confirmed` hoặc `checked_in` tuyệt đối không bị ảnh hưởng khi Redis restart hay gặp lỗi.

#### Tiêu chí nghiệm thu & Kiểm thử
- Integration test (Testcontainers PostgreSQL + Redis):
  - Tạo hold/booking `pending` với TTL ngắn (1–2s), chờ hết hạn → chạy expiry scanner → DB chuyển `expired`, có dòng `booking_status_history`, key Redis được xóa sạch.
  - Mô phỏng Redis `FLUSHALL` (mất sạch key) trong khi DB vẫn còn hold còn hạn → chạy reconciler → toàn bộ key còn hạn được khôi phục lại vào Redis với TTL còn lại.
  - Mô phỏng key mồ côi trên Redis không có trong DB → chạy reconciler → key mồ côi bị xóa sạch.
- Chạy `cd BackEnd && go test -race ./internal/modules/booking/...`.

---

### T3.4 — Đặt phòng lẻ từ Hold, Snapshot khách lưu trú đại diện, Idempotency & Xác nhận thủ công của nhân viên

- **Commit gợi ý:** `feat(booking): implement individual booking creation from hold, guest snapshot and staff confirmation`
- **Depends on:** `T3.2`, `T3.3`
- **Modules / Files:** `internal/modules/booking/{domain,application,delivery/http,persistence/postgres}`, `BackEnd/api/openapi.yaml`

#### Mục tiêu nghiệp vụ
Chuyển đổi 1 hold hợp lệ thành đơn đặt phòng lẻ (`bookings`) bền vững trên PostgreSQL với đầy đủ snapshot người lưu trú đại diện (tách biệt với Customer) và snapshot giá/VAT, kèm API xem danh sách/chi tiết và xác nhận thủ công cho khách đặt tại quầy.

#### Phạm vi thực hiện
- REST endpoints:
  - `POST /api/v1/bookings` (authenticated customer hoặc staff `booking.create`, bắt buộc header `Idempotency-Key`):
    - Đầu vào: `holdId`, `holdToken`, `representativeGuest { fullName, phone, email, nationalityCode, specialRequest }` (đặt hộ người khác thoải mái — `customer_id` vẫn là người đặt/trả tiền), hoặc cho phép staff tạo trực tiếp walk-in booking (tự cấp hold ngắn nội bộ và kiểm tra `bookings_no_room_overlap`).
    - Trong 1 PostgreSQL transaction: khóa `reservation_holds` (`FOR UPDATE`), kiểm tra quyền sở hữu + `status='active'` + `expires_at > now()` + khớp `hold_token`; tính lại giá & VAT từ `pricing.QuoteCalculator`; chèn `bookings` (`status='pending'`, `expires_at = hold.expires_at`), chèn `booking_guest_snapshots`, chèn `booking_status_history`, cập nhật `reservation_holds.status = 'converted'`, ghi `audit_events` + `outbox_events`.
    - Nếu trùng `Idempotency-Key` của cùng customer → trả lại nguyên vẹn kết quả booking đã tạo (`200`/`201`) không tạo trùng.
    - Nếu dính lỗi `bookings_no_room_overlap` (`23P01`) → rollback, giải phóng key Redis khớp token và trả `409`.
  - `GET /api/v1/bookings` (`booking.read` cho staff; hoặc `GET /api/v1/me/bookings` cho customer chỉ thấy booking của mình), `GET /api/v1/bookings/{id}` (staff `booking.read` hoặc customer chủ sở hữu).
  - `POST /api/v1/bookings/{id}/confirm` (`booking.confirm`): Cho phép nhân viên xác nhận thủ công booking từ `pending → confirmed` (dùng cho khách đặt tại quầy/walk-in), xóa `expires_at`, ghi `booking_status_history` + `audit_events` + release Redis hold sau commit.
- Exported interface `booking.BookingStatusService` (để Phase 4 tự động confirm khi thanh toán đủ 100%, và Phase 5 kiểm tra/chuyển trạng thái check-in/out).

#### Quy tắc & Invariant bắt buộc
- Sửa hồ sơ `customers` sau khi đặt phòng tuyệt đối không làm thay đổi dữ liệu trong `booking_guest_snapshots`.
- Sửa giá `room_types` hoặc `vat_rates` sau khi đặt không làm thay đổi snapshot tiền của booking.
- Khách hàng A không thể xem hoặc dùng `holdId` của khách hàng B (`403`/`404`).

#### Tiêu chí nghiệm thu & Kiểm thử
- Integration test:
  - Luồng hold → `POST /api/v1/bookings` thành công; gọi lại lần 2 với cùng `Idempotency-Key` trả về đúng booking cũ không sinh bản ghi thứ 2.
  - Hold đã hết hạn hoặc sai `holdToken` trả `409`/`422`.
  - Sửa `customers.full_name` và `vat_rates` → kiểm tra `GET /api/v1/bookings/{id}` vẫn giữ nguyên guest snapshot và tiền/VAT lúc đặt.
  - Staff xác nhận thủ công `pending → confirmed` thành công; customer gọi endpoint confirm bị `403`.
- Chạy `cd BackEnd && go test -race ./internal/modules/booking/...`.

---

### T3.5 — Đặt phòng theo nhóm (Group Booking Aggregate) & Tự động tính lại trạng thái nhóm

- **Commit gợi ý:** `feat(booking): implement group booking aggregate creation and derived group status`
- **Depends on:** `T3.4`
- **Modules / Files:** `internal/modules/booking/{domain,application,delivery/http,persistence/postgres}`, `BackEnd/api/openapi.yaml`

#### Mục tiêu nghiệp vụ
Cho phép đặt nhiều phòng trong cùng một giao dịch đặt nhóm (`booking_groups`) có trạng thái tổng hợp (`aggregate status`) tự động cập nhật nhất quán mỗi khi bất kỳ phòng con nào đổi trạng thái.

#### Phạm vi thực hiện
- REST endpoints:
  - `POST /api/v1/booking-groups` (authenticated customer hoặc staff `booking.create`, bắt buộc `Idempotency-Key`):
    - Nhận `groupHoldId`, `holdToken`, và danh sách guest snapshot cho từng phòng (hoặc 1 đại diện chung sao chép xuống từng `booking_guest_snapshots`).
    - Trong **1 PostgreSQL transaction duy nhất**: khóa tất cả dòng `reservation_holds` thuộc nhóm theo thứ tự UUID (`FOR UPDATE`), xác thực tất cả đều `active` và chưa hết hạn; chèn 1 dòng `booking_groups` (`status='pending'`, `expires_at = group_hold.expires_at`) và N dòng `bookings` con + N dòng `booking_guest_snapshots` + `booking_status_history` + chuyển hold sang `converted` + `audit_events`. Bất kỳ phòng nào xung đột → rollback toàn bộ nhóm (không tạo nhóm nửa vời).
  - `GET /api/v1/booking-groups/{id}` và `GET /api/v1/me/booking-groups` (ownership guard cho customer, `booking.read` cho staff): trả về thông tin nhóm, tổng tiền nhóm và danh sách đầy đủ các `bookings` con.
- Domain function `ComputeGroupStatus(childStatuses []BookingStatus) GroupStatus` chạy bên trong **mọi transaction** làm thay đổi trạng thái phòng con:
  - Tất cả `expired` → `expired`
  - Tất cả `cancelled` → `cancelled`
  - Tất cả `checked_out` → `completed`
  - Tất cả `confirmed` (hoặc đang `checked_in`/`checked_out` không có hủy) → `confirmed`
  - Vừa có `confirmed` vừa có `pending` → `partially_confirmed`
  - Có ít nhất 1 `cancelled` và vẫn còn phòng chưa hủy (`pending`/`confirmed`/`checked_in`/`checked_out`) → `partially_cancelled`
  - Tất cả `pending` → `pending`

#### Quy tắc & Invariant bắt buộc
- `booking_groups` là aggregate thực sự chứa nhiều `bookings` con; không được tồn tại nhóm rỗng (0 phòng con).
- Mọi thay đổi trạng thái phòng con (confirm, cancel, expire, check-out) bắt buộc recompute `booking_groups.status` trong cùng transaction.

#### Tiêu chí nghiệm thu & Kiểm thử
- Unit test đầy đủ bảng tổ hợp cho `ComputeGroupStatus` (đủ 8 trạng thái nhóm).
- Integration test:
  - Tạo group 3 phòng thành công trong 1 tx; replay `Idempotency-Key` trả về đúng nhóm cũ.
  - Nếu 1 trong 3 phòng bị dính conflict ở DB → rollback toàn bộ (0 `booking_groups`, 0 `bookings` được tạo).
  - Confirm 1/3 phòng → nhóm chuyển `partially_confirmed`; confirm hết 3/3 → `confirmed`.
- Chạy `cd BackEnd && go test -race ./internal/modules/booking/...`.

---

### T3.6 — Hủy booking lẻ & Hủy hàng loạt All-or-Nothing (`POST /api/v1/bookings/batch/cancel`)

- **Commit gợi ý:** `feat(booking): implement single booking cancel and all-or-nothing batch cancellation`
- **Depends on:** `T3.5`
- **Modules / Files:** `internal/modules/booking/{domain,application,delivery/http}`, `BackEnd/api/openapi.yaml`

#### Mục tiêu nghiệp vụ
Hỗ trợ hủy từng phòng riêng lẻ (hủy 1 phòng con không kéo theo hủy các phòng anh em trong nhóm) và hủy hàng loạt nhiều booking nguyên tử (tất cả thành công hoặc rollback toàn bộ), đồng thời chừa hook sạch để Phase 4 nhả payment intent/hoàn tiền tự động về ví.

#### Phạm vi thực hiện
- REST endpoints:
  - `POST /api/v1/bookings/{id}/cancel` (customer sở hữu booking hoặc staff `booking.cancel`):
    - Nhận `reason` (tùy chọn).
    - Trong 1 transaction: khóa `bookings` (và `booking_groups` nếu có) `FOR UPDATE`; kiểm tra trạng thái hợp lệ (`pending` hoặc `confirmed`; từ chối `409` nếu đã `checked_in`, `checked_out`, `cancelled`, `expired`); chuyển `status='cancelled'`, ghi `cancelled_at`, `cancel_reason`, `booking_status_history`, recompute `booking_groups.status`, ghi `audit_events` + `outbox_events`.
    - Gọi hook `OnBookingCancelled(ctx, booking, isFreeCancelEligible)` (đánh giá `now <= check_in_date - free_cancel_hours` lấy từ `hotel_settings`; Phase 3 mặc định no-op, Phase 4 gắn xử lý hủy payment intent + tự hoàn 100% về ví).
    - Sau commit: giải phóng key Redis hold tương ứng (nếu đang `pending`).
  - `POST /api/v1/bookings/batch/cancel` (customer sở hữu tất cả ID hoặc staff `booking.cancel`):
    - Nhận `bookingIds []uuid.UUID` (1..50 phần tử, loại trùng) và `reason`.
    - **Giao thức All-or-Nothing:**
      1. Sắp xếp `bookingIds` theo thứ tự UUID tăng dần (chống deadlock).
      2. Trong **1 PostgreSQL transaction duy nhất**: lock tất cả các dòng `bookings` theo thứ tự đã sort (`FOR UPDATE`), kiểm tra tồn tại, quyền sở hữu/phân quyền và trạng thái hợp lệ cho **từng dòng một**.
      3. Nếu **bất kỳ 1 booking nào không hợp lệ** (không tồn tại, không thuộc quyền, hoặc đã `checked_in`/`cancelled`) → **rollback toàn bộ transaction** và trả `409`/`403`/`404` nêu rõ booking ID lỗi (không một dòng nào bị hủy).
      4. Nếu tất cả hợp lệ → hủy toàn bộ, ghi history/audit/outbox, recompute mọi `booking_groups` liên quan, gọi hook Phase 4 trong cùng tx, rồi post-commit nhả Redis keys.

#### Quy tắc & Invariant bắt buộc
- Hủy 1 phòng con trong `booking_groups` **tuyệt đối không** tự hủy các phòng anh em (trừ khi người dùng gửi danh sách tất cả ID vào `batch/cancel`).
- Với booking thuộc nhóm đang có group `payment_intent` chưa thanh toán (`awaiting_bank_transfer`), hook Phase 4 sẽ tự hủy/nhả group intent đó trong cùng transaction để tránh intent bị lệch số tiền.

#### Tiêu chí nghiệm thu & Kiểm thử
- Integration test:
  - Hủy 1 trong 3 phòng con của nhóm → 2 phòng còn lại giữ nguyên trạng thái, nhóm chuyển `partially_cancelled`.
  - `POST /api/v1/bookings/batch/cancel` với 3 ID trong đó ID thứ 3 đã `checked_in` (hoặc thuộc khách khác) → trả `409`/`403` và xác nhận 2 booking đầu **vẫn giữ nguyên trạng thái cũ**, 0 dòng history/audit được ghi.
  - `batch/cancel` với 3 ID hợp lệ → cả 3 chuyển `cancelled` nguyên tử trong 1 transaction.
- Chạy `cd BackEnd && go test -race ./internal/modules/booking/...`.
