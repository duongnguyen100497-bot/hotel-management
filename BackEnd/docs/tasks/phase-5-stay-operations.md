# Phase 5 — Vận hành lưu trú, Dịch vụ phát sinh & Hóa đơn Check-out (Stay Operations & Invoicing)

> **Mục tiêu Phase:** Hoàn thiện quy trình vận hành thực tế của lễ tân và buồng phòng: check-in (hỗ trợ cả booking đã trả online lẫn booking walk-in do lễ tân confirm chưa trả trước), ghi nhận dịch vụ sử dụng trong kỳ lưu trú với snapshot giá/VAT bất biến, check-out nguyên tử sinh hóa đơn bất biến + chuyển phòng sang dọn dẹp, thanh toán phần còn thiếu tại quầy (`POST /api/v1/invoices/{id}/payments`), xem hóa đơn và báo phòng sẵn sàng.
>
> **Phụ thuộc:** Hoàn thành Phase 4 (`T4.1`–`T4.6`) và `T2.3`, `T2.4`.
>
> **Tiêu chí hoàn thành Phase (Exit Criteria):**
> - Check-in thành công cho cả booking đã thanh toán online lẫn booking walk-in được nhân viên confirm, chuyển phòng sang `occupied` nguyên tử.
> - Thêm dịch vụ phát sinh trong quá trình lưu trú chụp đúng snapshot tên/đơn giá/VAT hiện hành (đổi giá dịch vụ sau đó không đổi tiền stay).
> - Check-out nguyên tử trong 1 transaction: ghi nhận `checked_out_at`, tính hóa đơn gồm tiền phòng chưa thanh toán + dịch vụ phát sinh + VAT snapshot, đánh dấu hóa đơn `paid` nếu số tiền phải thu thêm = 0 hoặc `issued` nếu còn thiếu, và chuyển phòng sang `cleaning`.
> - Thanh toán phần còn thiếu trên hóa đơn tại quầy (tiền mặt / chuyển khoản quầy / trừ ví) idempotent và sẵn sàng cấp dữ liệu cho báo cáo doanh thu (`T6.2`).

---

### T5.1 — Schema Stay Operations & Billing (Migration 011) + Quy trình Check-in (`POST /api/v1/bookings/{id}/check-in`)

- **Commit gợi ý:** `feat(stayops): add stays and billing schema with check-in workflow`
- **Depends on:** `T4.4`, `T2.3`, `T2.4`
- **Modules / Files:** `BackEnd/db/migration/000011_create_stays_invoices.{up,down}.sql`, `BackEnd/db/query/stayops.sql`, `internal/modules/stayops/`, `BackEnd/api/openapi.yaml`

#### Mục tiêu nghiệp vụ
Tạo lược đồ lưu trú và hóa đơn, đồng thời cài đặt quy trình check-in nguyên tử gắn liền việc chuyển trạng thái booking và trạng thái phòng vật lý sang `occupied`.

#### Phạm vi thực hiện
- Migration `000011_create_stays_invoices`:
  - `stays`: `id uuid PK`, `booking_id uuid NN UQ FK -> bookings`, `room_id uuid NN FK -> rooms`, `checked_in_at timestamptz NN default now()`, `checked_out_at timestamptz ?`, `checked_in_by uuid NN FK -> users`, `checked_out_by uuid ? FK -> users`, `note text ?`, `created_at/updated_at/version`, `CHECK (checked_out_at IS NULL OR checked_out_at >= checked_in_at)`.
  - `stay_service_consumptions`: `id uuid PK`, `stay_id uuid NN FK -> stays`, `service_id uuid ? FK -> service_catalog`, `service_name_snapshot text NN`, `unit_price_snapshot numeric(14,2) NN CHECK (unit_price_snapshot >= 0)`, `vat_rate_id uuid NN FK -> vat_rates`, `vat_rate_snapshot numeric(5,4) NN`, `quantity numeric(10,2) NN CHECK (quantity > 0)`, `subtotal_amount numeric(14,2) NN`, `vat_amount numeric(14,2) NN`, `total_amount numeric(14,2) NN`, `consumed_at timestamptz NN default now()`, `added_by uuid NN FK -> users`, `created_at timestamptz NN default now()`.
  - `invoices`: `id uuid PK`, `reference_code varchar(32) NN UQ`, `booking_id uuid NN UQ FK -> bookings`, `stay_id uuid NN UQ FK -> stays`, `customer_id uuid NN FK -> customers`, `currency char(3) NN default 'VND'`, `subtotal_amount numeric(14,2) NN`, `vat_amount numeric(14,2) NN`, `total_amount numeric(14,2) NN`, `already_paid_amount numeric(14,2) NN default 0`, `remaining_due_amount numeric(14,2) NN CHECK (remaining_due_amount >= 0)`, `status varchar(24) NN CHECK (status IN ('issued','partially_paid','paid'))`, `issued_at timestamptz NN default now()`, `paid_at timestamptz ?`, `created_at/updated_at/version`.
  - `invoice_lines` (immutable): `id uuid PK`, `invoice_id uuid NN FK -> invoices`, `line_no smallint NN`, `line_type varchar(24) NN CHECK (line_type IN ('room_charge','service_charge'))`, `description text NN`, `quantity numeric(10,2) NN`, `unit_price numeric(14,2) NN`, `vat_rate numeric(5,4) NN`, `subtotal_amount numeric(14,2) NN`, `vat_amount numeric(14,2) NN`, `line_total numeric(14,2) NN`, `service_consumption_id uuid ? FK -> stay_service_consumptions`, `UNIQUE (invoice_id, line_no)`.
  - `invoice_payments` (append-only, thanh toán tại quầy): `id uuid PK`, `reference_code varchar(32) NN UQ`, `invoice_id uuid NN FK -> invoices`, `booking_id uuid NN FK -> bookings`, `method varchar(24) NN CHECK (method IN ('cash','bank_transfer','wallet'))`, `amount numeric(14,2) NN CHECK (amount > 0)`, `currency char(3) NN default 'VND'`, `wallet_ledger_entry_id uuid ? FK -> wallet_ledger_entries`, `idempotency_key varchar(128) NN UQ`, `note text ?`, `recorded_by uuid NN FK -> users`, `recorded_at timestamptz NN default now()`.
- REST endpoint `POST /api/v1/bookings/{id}/check-in` (`stay.check_in`):
  - Trong **1 PostgreSQL transaction**:
    1. Lock `bookings` và `rooms` (`FOR UPDATE`).
    2. Kiểm tra `booking.status == 'confirmed'` (cho phép cả booking đã thanh toán online lẫn booking walk-in do lễ tân confirm chưa thanh toán trước; từ chối `409` nếu đang `pending`/`cancelled`/`expired`/`checked_in`/`checked_out`).
    3. Kiểm tra `room.operational_status == 'available'` (nếu phòng đang `cleaning`/`maintenance`/`occupied` trả `409`).
    4. Tạo dòng `stays` (unique 1 stay / 1 booking), gọi `booking.MarkCheckedIn` (ghi `booking_status_history`, recompute group status), gọi `property.RoomStateService.MarkOccupied` (chuyển phòng → `occupied`, ghi `room_state_history`), ghi `audit_events`.

#### Quy tắc & Invariant bắt buộc
- Một booking chỉ có tối đa 1 `stays` (`UNIQUE booking_id`).
- Booking ở trạng thái `pending` tuyệt đối không được check-in.
- Nếu bất kỳ bước nào lỗi (ví dụ phòng chưa sẵn sàng), toàn bộ transaction rollback — không đổi trạng thái booking hay phòng.

#### Tiêu chí nghiệm thu & Kiểm thử
- Integration test (Testcontainers PostgreSQL):
  - Check-in booking `confirmed` (đã trả online và walk-in chưa trả) khi phòng `available` → tạo `stays`, booking → `checked_in`, phòng → `occupied`, có đủ `room_state_history` + `booking_status_history` + audit.
  - Check-in booking `pending` hoặc check-in khi phòng đang `cleaning`/`maintenance` → `409`, 0 thay đổi DB.
  - Check-in lặp lại lần 2 → `409`.
- Chạy `cd BackEnd && go test -race ./internal/modules/stayops/...`.

---

### T5.2 — Ghi nhận dịch vụ phát sinh trong quá trình lưu trú (`stay_service_consumptions`)

- **Commit gợi ý:** `feat(stayops): add stay service consumption recording with immutable price and vat snapshots`
- **Depends on:** `T5.1`, `T2.4`
- **Modules / Files:** `internal/modules/stayops/{domain,application,delivery/http}`, `BackEnd/api/openapi.yaml`

#### Mục tiêu nghiệp vụ
Cho phép lễ tân ghi nhận các dịch vụ khách sử dụng khi đang lưu trú (`checked_in`) bằng cách chụp snapshot tên, đơn giá và mức VAT hiện hành, bảo toàn dữ liệu tính hóa đơn khi danh mục dịch vụ hoặc VAT thay đổi về sau.

#### Phạm vi thực hiện
- REST endpoints:
  - `POST /api/v1/stays/{stayId}/services` (`stay.service.manage`):
    - Đầu vào: `serviceId`, `quantity (> 0)`, `consumedAt` (tùy chọn, mặc định `now()`).
    - Kiểm tra `stay.checked_out_at IS NULL` (stay đang mở); gọi exported read interface của `service_catalog` kiểm tra dịch vụ tồn tại và `is_active = true`; lấy mức VAT active từ `pricing.QuoteCalculator`; tính `subtotal = unit_price * quantity`, `vat = subtotal * vat_rate`, `total = subtotal + vat` bằng decimal; lưu `stay_service_consumptions` + `audit_events`.
  - `GET /api/v1/stays/{stayId}/services` (`stay.check_in` / `stay.check_out` / `booking.read` / customer chủ sở hữu booking): liệt kê toàn bộ dịch vụ đã dùng trong stay kèm tổng tiền tạm tính.

#### Quy tắc & Invariant bắt buộc
- Không thể thêm dịch vụ vào stay đã check-out (`409`) hoặc dùng dịch vụ đã bị deactivate (`422`/`409`).
- Thay đổi giá trong `service_catalog` hoặc đổi `vat_rates` sau khi ghi nhận không làm thay đổi các dòng `stay_service_consumptions` đã lưu.

#### Tiêu chí nghiệm thu & Kiểm thử
- Integration test: thêm 2 dịch vụ vào stay đang mở → đổi giá dịch vụ + deactivate dịch vụ → `GET /api/v1/stays/{stayId}/services` giữ nguyên snapshot cũ; thử thêm dịch vụ đã deactivate → bị từ chối; thử thêm vào stay đã check-out → `409`.
- Chạy `cd BackEnd && go test -race ./internal/modules/stayops/...`.

---

### T5.3 — Quy trình Check-out nguyên tử & Sinh hóa đơn bất biến (`POST /api/v1/bookings/{id}/check-out`)

- **Commit gợi ý:** `feat(stayops): implement atomic check-out workflow with immutable invoice generation`
- **Depends on:** `T5.2`, `T4.4`
- **Modules / Files:** `internal/modules/stayops/{domain,application,delivery/http,persistence/postgres}`, `BackEnd/api/openapi.yaml`

#### Mục tiêu nghiệp vụ
Thực hiện check-out hoàn toàn nguyên tử trong 1 PostgreSQL transaction: chốt thời điểm trả phòng, tổng hợp tiền phòng (đã chụp snapshot lúc đặt) + toàn bộ dịch vụ phát sinh thành hóa đơn và các dòng hóa đơn bất biến, tự khấu trừ số tiền đã thanh toán online ở Phase 4, chuyển phòng sang trạng thái dọn dẹp (`cleaning`), cập nhật trạng thái nhóm và ghi audit/outbox.

#### Phạm vi thực hiện
- REST endpoint `POST /api/v1/bookings/{id}/check-out` (`stay.check_out`):
  - Trong **1 PostgreSQL transaction duy nhất**:
    1. Lock `bookings`, `stays`, `rooms` (`FOR UPDATE`). Kiểm tra `booking.status == 'checked_in'` và `stay.checked_out_at IS NULL` (sai trả `409`).
    2. Cập nhật `stays.checked_out_at = now()`, `checked_out_by = actor`.
    3. Gom dữ liệu hóa đơn:
       - Dòng 1 (`line_no = 1`, `line_type = 'room_charge'`): lấy nguyên snapshot tiền phòng (`subtotal_amount`, `vat_rate_snapshot`, `vat_amount`, `total_amount`) từ `bookings`.
       - Các dòng tiếp theo (`line_no = 2..k`, `line_type = 'service_charge'`): map từng dòng `stay_service_consumptions` (giữ nguyên snapshot giá & VAT).
    4. Truy vấn số tiền đã thanh toán trước cho booking này (`already_paid = SUM(payment_allocations.amount) - SUM(refunds.amount)` từ module `wallet` qua exported read interface).
    5. Tính `total_amount = sum(line_total)`, `remaining_due = max(0, total_amount - already_paid)`.
       - Nếu `remaining_due == 0` → `invoices.status = 'paid'`, `paid_at = now()`.
       - Nếu `remaining_due > 0` (khách walk-in chưa trả tiền phòng hoặc có dịch vụ phát sinh mới) → `invoices.status = 'issued'`, `paid_at = NULL`.
    6. Chèn `invoices` + toàn bộ `invoice_lines` (bất biến).
    7. Gọi `booking.MarkCheckedOut` (booking → `checked_out`, recompute `booking_groups.status` → `completed` nếu tất cả phòng con đã check-out).
    8. Gọi `property.RoomStateService.MarkCleaning` (phòng → `cleaning`, ghi `room_state_history`).
    9. Ghi `audit_events` + enqueue `stay.checked_out` (email hóa đơn) vào `outbox_events`.
  - Trả về `200` gồm chi tiết `stay` và `invoice` + `invoice_lines` vừa tạo.

#### Quy tắc & Invariant bắt buộc
- Check-out phải **100% nguyên tử**: nếu sinh hóa đơn hoặc chuyển phòng sang `cleaning` lỗi, booking vẫn ở `checked_in` và không tồn tại hóa đơn rác.
- `invoice_lines` là bất biến sau khi phát hành (không có endpoint sửa/xóa dòng hóa đơn).

#### Tiêu chí nghiệm thu & Kiểm thử
- Integration test (Testcontainers PostgreSQL):
  - Kịch bản 1 (online đã trả đủ 100%, không dùng dịch vụ thêm): checkout → `invoice.status = 'paid'`, `remaining_due = 0`, phòng → `cleaning`, booking → `checked_out`, group → `completed`.
  - Kịch bản 2 (online đã trả tiền phòng + dùng thêm 2 dịch vụ, hoặc walk-in chưa trả tiền phòng): checkout → `invoice.status = 'issued'`, `remaining_due` chính xác bằng phần chưa trả.
  - Kịch bản rollback: giả lập lỗi khi chuyển trạng thái phòng → verify 0 dòng `invoices`/`invoice_lines`, `stay.checked_out_at` vẫn `NULL`.
- Chạy `cd BackEnd && go test -race ./internal/modules/stayops/...`.

---

### T5.4 — Thanh toán hóa đơn tại quầy (`POST /api/v1/invoices/{id}/payments`), Xem hóa đơn & Chuyển phòng sẵn sàng

- **Commit gợi ý:** `feat(stayops): add counter invoice payment settlement, invoice read apis and room-ready transition`
- **Depends on:** `T5.3`, `T4.2`
- **Modules / Files:** `internal/modules/stayops/{application,delivery/http}`, `internal/modules/wallet/` (exported `DebitForInvoicePayment` interface), `BackEnd/api/openapi.yaml`

#### Mục tiêu nghiệp vụ
Cho phép lễ tân/kế toán ghi nhận thanh toán phần tiền còn thiếu trên hóa đơn check-out (khách walk-in hoặc dịch vụ phát sinh) bằng tiền mặt, chuyển khoản tại quầy hoặc trừ ví khách hàng, cung cấp API xem hóa đơn cho staff/khách hàng và hoàn thiện vòng đời buồng phòng (`cleaning → available`).

#### Phạm vi thực hiện
- REST endpoints:
  - `POST /api/v1/invoices/{id}/payments` (`payment.record`, bắt buộc `Idempotency-Key`):
    - Đầu vào: `method` (`cash` | `bank_transfer` | `wallet`), `amount (> 0)`, `note`.
    - Trong **1 PostgreSQL transaction**:
      1. Kiểm tra `idempotency_key` (replay trả bản ghi thanh toán cũ).
      2. Lock `invoices` (`FOR UPDATE`); từ chối `409` nếu hóa đơn đã `paid` hoặc `amount > remaining_due_amount`.
      3. Nếu `method == 'wallet'`: gọi exported interface của `wallet` lock ví khách (`FOR UPDATE`), kiểm tra `available_balance >= amount`, trừ ví và ghi ledger `invoice_payment` (trả về `wallet_ledger_entry_id`).
      4. Chèn `invoice_payments`, cập nhật `invoices.already_paid_amount += amount`, `invoices.remaining_due_amount -= amount`; nếu `remaining_due_amount == 0` → `status = 'paid', paid_at = now()`, ngược lại `status = 'partially_paid'`.
      5. Ghi `audit_events` + `outbox_events`.
  - `GET /api/v1/invoices` (`invoice.read`, lọc theo `status`, `customerId`, khoảng ngày, cursor pagination).
  - `GET /api/v1/invoices/{id}` (staff `invoice.read` hoặc customer chủ sở hữu hóa đơn): trả chi tiết hóa đơn kèm `invoice_lines` và danh sách `invoice_payments` + các `payment_allocations` đã trả trước.
  - `GET /api/v1/me/invoices` (authenticated customer): danh sách hóa đơn của riêng mình.
  - Xác nhận tích hợp trơn tru với `POST /api/v1/rooms/{id}/ready` (`T2.3`): sau khi checkout chuyển phòng sang `cleaning`, buồng phòng/lễ tân gọi `/ready` đưa phòng về `available` để đón khách mới.
- Exported read interface (cho Phase 6): truy vấn `invoice_payments` đã thu theo khoảng thời gian (phục vụ `T6.2` Doanh thu và `T6.3` Khách quay lại).

#### Quy tắc & Invariant bắt buộc
- Không thể thanh toán vượt quá `remaining_due_amount` của hóa đơn (`409`).
- Khách hàng A không thể xem hóa đơn của khách hàng B (`403`/`404`).

#### Tiêu chí nghiệm thu & Kiểm thử
- Integration test (Testcontainers PostgreSQL):
  - Hóa đơn `issued` còn thiếu 500.000đ: trả 200.000đ tiền mặt → `partially_paid` (còn 300.000đ); trả tiếp 300.000đ bằng `wallet` → ví giảm đúng 300.000đ (có ledger `invoice_payment`), hóa đơn chuyển `paid`; cố trả thêm 1đ bị `409`; replay `Idempotency-Key` không trừ 2 lần.
  - Kiểm tra đầy đủ luồng walk-in: tạo booking → staff confirm → check-in → thêm dịch vụ → check-out (`issued`) → thanh toán tại quầy (`paid`) → gọi `/rooms/{id}/ready` về `available`.
  - Negative ownership: customer B gọi `GET /api/v1/invoices/{id_cua_A}` → `403`.
- Chạy `cd BackEnd && go test -race ./internal/modules/stayops/... ./internal/modules/wallet/...`.
