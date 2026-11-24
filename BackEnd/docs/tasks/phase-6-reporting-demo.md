# Phase 6 — Báo cáo thực tế, Snapshot công suất 19:00 & Dữ liệu Demo (Reporting & Demo Readiness)

> **Mục tiêu Phase:** Thay thế hoàn toàn dữ liệu báo cáo giả lập bằng hệ thống báo cáo chính xác dựa trên dữ liệu đã commit trong PostgreSQL: lịch chụp snapshot công suất phòng lúc 19:00 giờ địa phương kèm tự động backfill ngày bị lỡ, báo cáo doanh thu thực thu ròng (gồm thanh toán online + thanh toán hóa đơn tại quầy trừ hoàn tiền) có xuất CSV, báo cáo khách hàng quay lại theo đúng định nghĩa cohort, API tra cứu audit và bộ CLI seed dữ liệu demo + hướng dẫn kiểm thử E2E hoàn chỉnh cho đồ án.
>
> **Phụ thuộc:** Hoàn thành Phase 5 (`T5.1`–`T5.4`).
>
> **Tiêu chí hoàn thành Phase (Exit Criteria):**
> - Job chụp snapshot 19:00 ghi đúng 1 bản ghi/ngày (`UNIQUE (business_date)`), loại trừ phòng `maintenance` khỏi mẫu số, không đếm hold/pending là đang có khách, và tự backfill idempotent khi service tắt qua ngày.
> - Báo cáo doanh thu chỉ tính tiền thực thu (online `payment_allocations` + counter `invoice_payments`) trừ `refunds`, tách biệt rõ `gross_collected`, `refunds`, `net_revenue` và xuất được CSV hợp lệ.
> - Báo cáo khách quay lại áp dụng đúng điều kiện đã thanh toán (loại booking hủy/hoàn 100%, tính 1 `booking_group` = 1 lần đặt) và phân kỳ theo `bookings.created_at` (7 ngày lăn / tháng / quý / năm).
> - Chạy 1 lệnh seed demo tạo đủ dữ liệu mẫu và chạy trọn kịch bản smoke test E2E (từ đăng ký → giữ phòng → đặt nhóm → thanh toán ví/SePay → check-in/out → báo cáo) pass 100%.

---

### T6.1 — Snapshot công suất phòng 19:00 hàng ngày (Migration 012), Tự động Backfill & API Báo cáo công suất

- **Commit gợi ý:** `feat(reporting): add 19:00 daily occupancy snapshot scheduler, backfill and occupancy report api`
- **Depends on:** `T5.3`, `T2.3`
- **Modules / Files:** `BackEnd/db/migration/000012_create_reporting_snapshots.{up,down}.sql`, `BackEnd/db/query/reporting.sql`, `internal/modules/reporting/`, `cmd/api/main.go`, `BackEnd/api/openapi.yaml`

#### Mục tiêu nghiệp vụ
Ghi nhận ảnh chụp công suất phòng bất biến lúc **19:00 giờ địa phương khách sạn** mỗi ngày (có tự động lấp ngày bị sót nếu server tắt), làm nền tảng duy nhất cho báo cáo công suất theo ngày/tuần/tháng/quý/năm.

#### Phạm vi thực hiện
- Migration `000012_create_reporting_snapshots`:
  - `daily_occupancy_snapshots`: `id uuid PK`, `business_date date NN UQ`, `captured_at timestamptz NN`, `total_rooms int NN CHECK (total_rooms >= 0)`, `maintenance_rooms int NN CHECK (maintenance_rooms >= 0)`, `available_rooms int NN CHECK (available_rooms = total_rooms - maintenance_rooms AND available_rooms >= 0)`, `occupied_rooms int NN CHECK (occupied_rooms >= 0)`, `occupancy_rate numeric(6,4) NN CHECK (occupancy_rate >= 0)`, `is_backfilled boolean NN default false`, `created_at timestamptz NN default now()`.
- **19:00 Scheduler & Backfill Logic:**
  - Chạy định kỳ trong process (kiểm tra theo `hotel_settings.timezone`, mặc định `Asia/Ho_Chi_Minh`) với PostgreSQL advisory lock (`pg_try_advisory_lock`) để bảo đảm chạy đồng thời nhiều lần không trùng lặp.
  - Tại thời điểm chụp `T = business_date 19:00:00` theo giờ khách sạn:
    - `total_rooms`: tổng phòng chưa bị archive tồn tại tại `T`.
    - `maintenance_rooms`: số phòng có trạng thái `maintenance` tại `T` (suy từ `room_state_history` gần nhất `<= T` hoặc trạng thái hiện tại cho hôm nay).
    - `available_rooms = total_rooms - maintenance_rooms`.
    - `occupied_rooms`: số phòng thực sự đang ở trạng thái **đã check-in (`stays.checked_in_at <= T AND (stays.checked_out_at IS NULL OR stays.checked_out_at > T)`)**. **Tuyệt đối không tính** `reservation_holds` hay `bookings` đang `pending`/`confirmed` chưa check-in.
    - `occupancy_rate = 0` nếu `available_rooms == 0`, ngược lại `occupied_rooms / available_rooms`.
  - Chèn idempotent (`ON CONFLICT (business_date) DO NOTHING` — snapshot lịch sử bất biến).
  - **Missed-date backfill:** Khi khởi động service và mỗi chu kỳ scheduler, quét các ngày từ `max(last_snapshot_date + 1, earliest_room_date)` đến ngày hôm qua (hoặc hôm nay nếu đã qua 19:00) chưa có snapshot và tự tính toán + lưu với `is_backfilled = true`.
- REST endpoint `GET /api/v1/reports/occupancy` (`report.read`):
  - Tham số: `period` (`daily` | `weekly` | `monthly` | `quarterly` | `yearly`), `from`, `to`.
  - Trả về danh sách từng kỳ kèm: `dateCount` (số ngày có snapshot), `averageOccupancyRate`, `totalOccupiedRoomDays` (tử số cộng dồn), `totalAvailableRoomDays` (mẫu số cộng dồn) và mảng chi tiết theo ngày.

#### Quy tắc & Invariant bắt buộc
- Mỗi `business_date` có đúng **1 snapshot bất biến**; phòng bảo trì luôn bị trừ khỏi mẫu số `available_rooms`; hold và booking chưa check-in không bao giờ được tính là đang có khách.

#### Tiêu chí nghiệm thu & Kiểm thử
- Unit & Integration test (Testcontainers PostgreSQL với `Clock` giả lập):
  - Khách sạn có 10 phòng: 2 phòng `maintenance`, 3 phòng đã `checked_in` trước 19:00, 2 phòng `pending`/`confirmed` chưa check-in → snapshot lúc 19:00 ghi `total=10, maintenance=2, available=8, occupied=3, rate=0.3750`.
  - Chạy scheduler lại cùng ngày → không ghi đè hay báo lỗi (`ON CONFLICT DO NOTHING`).
  - Giả lập bỏ lỡ 3 ngày liên tiếp rồi khởi động job → tự động backfill đủ 3 ngày với `is_backfilled = true` dựa trên dữ liệu `stays` và `room_state_history`.
- Chạy `cd BackEnd && go test -race ./internal/modules/reporting/...`.

---

### T6.2 — Báo cáo doanh thu thực thu ròng (`GET /api/v1/reports/revenue`) & Xuất CSV

- **Commit gợi ý:** `feat(reporting): add net revenue report api and bounded csv export`
- **Depends on:** `T5.4`, `T4.6`
- **Modules / Files:** `internal/modules/reporting/{application,delivery/http,persistence/postgres}`, `BackEnd/api/openapi.yaml`

#### Mục tiêu nghiệp vụ
Cung cấp báo cáo doanh thu dựa hoàn toàn trên dòng tiền thực tế đã thu (từ cả thanh toán online lẫn thanh toán hóa đơn tại quầy) trừ các khoản đã hoàn về ví, hỗ trợ nhóm theo ngày/tháng/quý/năm và xuất CSV.

#### Phạm vi thực hiện
- Công thức nguồn dữ liệu thực thu trong khoảng thời gian `[from, to]` (chuyển đổi theo `hotel_settings.timezone`):
  - `gross_collected`: Tổng `payment_allocations.amount` (theo `settled_at`) **+** tổng `invoice_payments.amount` (theo `recorded_at`). *(Lưu ý: nạp ví chưa thanh toán booking không phải doanh thu; booking `pending`/`expired`/`cancelled` chưa trả tiền không phải doanh thu).*
  - `refunds`: Tổng `refunds.amount` (theo `created_at`).
  - `net_revenue = gross_collected - refunds`.
  - Tách thêm trường tham khảo rõ nhãn `booking_face_value` (giá trị danh nghĩa của các booking `confirmed`/`checked_in`/`checked_out`) để đối chiếu nhưng **không trộn** vào `net_revenue`.
- REST endpoints:
  - `GET /api/v1/reports/revenue` (`report.read`): Tham số `from`, `to` (validate bắt buộc, `from <= to`, giới hạn tối đa 366 ngày), `groupBy` (`day` | `month` | `quarter` | `year`). Trả tổng kỳ (`grossCollected`, `refunds`, `netRevenue`, `currency`) + mảng breakdown theo từng mốc thời gian và tách theo kênh (`wallet`, `sepay`, `cash`, `bank_transfer`).
  - `POST /api/v1/reports/exports` (`report.export`): Xuất đồng bộ file `text/csv` (có UTF-8 BOM cho Excel đọc tiếng Việt đẹp) cho báo cáo doanh thu hoặc công suất trong phạm vi ngày giới hạn (`<= 366 ngày`), ghi `audit_events`.

#### Quy tắc & Invariant bắt buộc
- Tuyệt đối không dùng số liệu ngẫu nhiên; mọi con số khớp 100% với tổng `payment_allocations` + `invoice_payments` - `refunds` dưới PostgreSQL.
- Chống CSV injection (escape giá trị bắt đầu bằng `=`, `+`, `-`, `@` nếu có chuỗi tự do).

#### Tiêu chí nghiệm thu & Kiểm thử
- Integration test (Testcontainers PostgreSQL):
  - Dựng dữ liệu fixture gồm: 1 booking `pending` (0đ thu), 1 booking `expired` (0đ thu), 1 booking online SePay 2.000.000đ, 1 booking walk-in trả tiền mặt tại quầy 1.500.000đ, 1 hoàn tiền 500.000đ, 1 khoản nạp ví 1.000.000đ chưa tiêu → kiểm tra báo cáo trả chính xác `gross_collected = 3.500.000`, `refunds = 500.000`, `net_revenue = 3.000.000`.
  - Test xuất CSV trả `Content-Type: text/csv`, đúng header và dòng tổng; user không có `report.export` nhận `403`.
- Chạy `cd BackEnd && go test -race ./internal/modules/reporting/...`.

---

### T6.3 — Báo cáo khách hàng quay lại (`GET /api/v1/reports/returning-customers`)

- **Commit gợi ý:** `feat(reporting): add returning customers cohort report api`
- **Depends on:** `T5.4`, `T4.6`
- **Modules / Files:** `internal/modules/reporting/{application,delivery/http,persistence/postgres}`, `BackEnd/api/openapi.yaml`

#### Mục tiêu nghiệp vụ
Tính chính xác tỷ lệ và số lượng khách hàng quay lại theo đúng định nghĩa đặc thù trong `implementation-plan.md` và các quy tắc ngoại lệ đã chốt (nhóm nhiều phòng = 1 lần đặt; loại booking đã hủy/hoàn tiền 100%; lấy mốc ngày tạo booking `created_at` với điều kiện đã thanh toán làm cổng lọc).

#### Phạm vi thực hiện
- Định nghĩa tập **Booking Event hợp lệ (Eligible Paid Booking Event)** trong SQL query:
  - Gom mỗi `booking_group_id` thành **1 sự kiện đặt phòng duy nhất** (`event_created_at = MIN(bookings.created_at)` của nhóm); mỗi booking lẻ (`booking_group_id IS NULL`) là 1 sự kiện riêng (`event_created_at = bookings.created_at`).
  - **Cổng lọc đã thanh toán (Paid Gate):** Sự kiện phải có ít nhất 1 khoản thanh toán thực tế (`payment_allocations` hoặc `invoice_payments`) với số tiền ròng `settled - refunded > 0` **và** không ở trạng thái toàn bộ bị `cancelled`/`expired` (loại các booking đã hủy và hoàn tiền 100%).
- Quy tắc phân loại khách quay lại cho khoảng thời gian báo cáo (tính theo `hotel_settings.timezone` dựa trên `event_created_at`):
  - **Tuần (`weekly` — cửa sổ 7 ngày lăn):** Khách có ít nhất 1 sự kiện hợp lệ có `event_created_at` rơi vào cửa sổ 7 ngày đang xét **VÀ** có 1 sự kiện hợp lệ khác trước đó tối thiểu **7 ngày tròn** (`earlier.event_created_at <= later.event_created_at - interval '7 days'`).
  - **Tháng (`monthly`):** Khách có ít nhất **2** sự kiện hợp lệ khác nhau đều có `event_created_at` nằm trong **cùng tháng dương lịch** đó.
  - **Quý (`quarterly`):** Khách có ít nhất **2** sự kiện hợp lệ khác nhau đều có `event_created_at` nằm trong **cùng quý dương lịch** đó.
  - **Năm (`yearly`):** Khách có ít nhất **2** sự kiện hợp lệ khác nhau đều có `event_created_at` nằm trong **cùng năm dương lịch** đó.
- REST endpoint `GET /api/v1/reports/returning-customers` (`report.read`):
  - Tham số: `period` (`weekly` | `monthly` | `quarterly` | `yearly`), `anchorDate` (hoặc `year`, `month`, `quarter`).
  - Trả về: `returningCustomerCount`, `totalActivePaidCustomers`, `returningRate`, metadata chú thích rõ quy tắc cohort (`created_at` với điều kiện thanh toán — lưu ý thanh toán trễ sang tháng sau sẽ làm cập nhật lại cohort của tháng tạo booking).

#### Quy tắc & Invariant bắt buộc
- Một khách hàng lần đầu đặt 1 `booking_group` gồm 3 phòng cùng lúc chỉ được tính là **1 sự kiện đặt phòng**, tuyệt đối không bị đếm nhầm thành khách quay lại.
- Booking tạo tháng 3 nhưng thanh toán trong tháng 4 vẫn thuộc cohort tháng 3 sau khi thanh toán thành công.

#### Tiêu chí nghiệm thu & Kiểm thử
- Integration test (Testcontainers PostgreSQL) bao phủ đủ 6 trường hợp:
  1. Khách A đặt 1 group 3 phòng trong tháng 3 (đã thanh toán) → **không** phải khách quay lại tháng 3 (`count = 0`).
  2. Khách B đặt 2 đơn rời nhau trong tháng 3 (cả 2 đã thanh toán) → **là** khách quay lại tháng 3.
  3. Khách C đặt 2 đơn trong tháng 3 nhưng đơn thứ 2 bị hủy & hoàn 100% → **không** phải khách quay lại.
  4. Khách D đặt đơn 1 ngày 1/3 và đơn 2 ngày 10/3 (cách ≥ 7 ngày) → **là** khách quay lại trong kỳ `weekly` chứa ngày 10/3; nhưng nếu cách chỉ 3 ngày (1/3 và 4/3) → **không** tính là quay lại ở kỳ `weekly` (vẫn tính ở `monthly`).
  5. Khách E tạo 2 đơn trong tháng 3, đơn thứ 2 mãi đến 2/4 mới thanh toán → trước 2/4 không tính, sau 2/4 tính vào cohort tháng 3.
  6. Khách F thanh toán tiền mặt tại quầy cho đơn walk-in thứ 2 → được tính qua Paid Gate.
- Chạy `cd BackEnd && go test -race ./internal/modules/reporting/...`.

---

### T6.4 — API tra cứu Audit Log, CLI Seed dữ liệu Demo idempotent & Kịch bản Smoke Test E2E

- **Commit gợi ý:** `feat(demo): add audit query api, idempotent demo seed cli and e2e verification guide`
- **Depends on:** `T6.1`, `T6.2`, `T6.3`
- **Modules / Files:** `internal/modules/audit/delivery/http/`, `BackEnd/cmd/seed-demo/main.go`, `BackEnd/docs/demo-walkthrough.md`, `BackEnd/api/openapi.yaml`

#### Mục tiêu nghiệp vụ
Hoàn thiện khả năng tra cứu lịch sử kiểm toán cho Admin, cung cấp 1 lệnh seed dữ liệu demo sạch/idempotent để chuẩn bị bài báo cáo đồ án nhanh chóng, và viết kịch bản kiểm thử xuyên suốt (E2E) chứng minh toàn bộ 12 module hoạt động khớp nhau.

#### Phạm vi thực hiện
- REST endpoint `GET /api/v1/audit-events` (`audit.read`):
  - Lọc theo `actorUserId`, `action`, `resourceType`, `resourceId`, `from`, `to`; sắp xếp theo `occurred_at DESC`, cursor pagination.
  - Bảo đảm dữ liệu `before_data`/`after_data` đã được redact sạch thông tin nhạy cảm.
- CLI `BackEnd/cmd/seed-demo/main.go` (chạy qua `make seed-demo`, **chỉ dùng cho môi trường dev/demo**, hoàn toàn tách khỏi migration & RBAC seed):
  - Idempotent (chạy nhiều lần không tạo trùng): tạo sẵn 3–4 loại phòng (Standard, Deluxe, Suite, Family), ~12 phòng vật lý trên 3 tầng (gồm 1 phòng `maintenance`), 5–6 dịch vụ khách sạn (Ăn sáng, Giặt ủi, Minibar, Đưa đón sân bay…), 1 mức VAT 10%, 3 tài khoản nhân viên mẫu (Quản lý, Lễ tân, Kế toán) và 2 tài khoản khách hàng mẫu có sẵn số dư ví + vài booking/snapshot lịch sử để mở báo cáo thấy ngay dữ liệu thật.
- Kiểm tra toàn bộ `BackEnd/api/openapi.yaml` hợp lệ (không thiếu endpoint hay schema nào từ Phase 0–6).
- Tạo `BackEnd/docs/demo-walkthrough.md`: hướng dẫn từng bước (kèm lệnh `make` & `curl`/`scripts/simulate-sepay.sh`) để trình diễn 7 luồng chính khi bảo vệ đồ án:
  1. Bootstrap Root Admin + Dynamic RBAC + tạo nhân viên & kích hoạt qua email trên Mailpit.
  2. Admin chỉnh TTL giữ phòng, cấu hình VAT và upload ảnh phòng lên MinIO.
  3. Khách giữ phòng nhóm qua Redis Lua + tạo Group Booking đặt hộ khách đại diện.
  4. Thanh toán tách Ví + SePay (chạy script giả lập webhook, test trùng & chuyển trễ cộng ví) + rút tiền ví minh họa.
  5. Hủy lẻ 1 phòng con (tự hoàn tiền về ví) & hủy hàng loạt batch cancel.
  6. Lễ tân check-in (kể cả walk-in) → thêm dịch vụ → check-out sinh hóa đơn → thanh toán tại quầy → báo phòng sẵn sàng.
  7. Xem báo cáo Doanh thu ròng (+ xuất CSV), Công suất 19:00 và Khách hàng quay lại + Audit Log.

#### Quy tắc & Invariant bắt buộc
- `cmd/seed-demo` phải kiểm tra `APP_ENV != "production"` (từ chối chạy nếu ở production).
- Mọi kiểm thử E2E sử dụng hoàn toàn API thật và PostgreSQL/Redis/MinIO/Mailpit thật, không có mock.

#### Tiêu chí nghiệm thu & Kiểm thử
- Chạy `make up && make migrate-up && make bootstrap && make seed-demo` (chạy `seed-demo` 2 lần liên tiếp để chứng minh idempotency) thành công không lỗi.
- Viết ít nhất 1 E2E integration test xuyên suốt (`internal/e2e_test.go`) chạy trọn vòng đời: register → hold → group booking → split payment (wallet + simulated SePay) → cancel 1 child (auto-refund) → check-in → add service → check-out → counter pay → occupancy/revenue/returning reports.
- Chạy toàn bộ test gate cuối cùng: `cd BackEnd && go test -race ./...` xanh 100%.
