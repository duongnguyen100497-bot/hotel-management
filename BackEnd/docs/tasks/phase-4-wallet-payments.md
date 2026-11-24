# Phase 4 — Ví khách hàng, Payment Intent, Webhook SePay & Hoàn tiền (Wallet & Payments)

> **Mục tiêu Phase:** Xây dựng hệ thống tài chính an toàn tuyệt đối với sổ cái bất biến (`wallet_ledger_entries`), chống âm số dư bằng `SELECT ... FOR UPDATE` + DB `CHECK`, hỗ trợ thanh toán đơn lẻ và **thanh toán 1 lần cho cả Group Booking** (Ví, SePay, hoặc kết hợp Ví + SePay), xử lý SePay webhook dòng tiền vào (trùng/trễ/lệch tiền), nạp ví thủ công cho demo, **rút tiền ví minh họa** (chỉ trừ ví, không chuyển tiền ngân hàng), đối soát biên lai và hoàn tiền về ví.
>
> **Phụ thuộc:** Hoàn thành Phase 3 (`T3.4`–`T3.6`).
>
> **Tiêu chí hoàn thành Phase (Exit Criteria):**
> - Mọi biến động số dư ví (`deposit`, `withdrawal`, `hold_reserved`, `hold_released`, `payment_captured`, `refund_credited`, `late_transfer_credited`) đều có dòng ledger bất biến, `available_balance >= 0` và `held_balance >= 0` được bảo đảm dưới tải đồng thời.
> - Thanh toán 100% (chỉ ví / chỉ SePay / tách ví + SePay) cho cả booking lẻ lẫn `booking_group` đều tự động confirm booking và ghi `payment_allocations` chi tiết tới từng booking con.
> - SePay webhook giả lập chạy qua script local xác minh đúng: giao dịch hợp lệ, gửi trùng (idempotent), chuyển khoản trễ sau khi intent hết hạn (cộng ví 1 lần duy nhất, không tái sử dụng wallet hold cũ), và sai chữ ký/số tiền.
> - Hủy booking `confirmed` trước hạn miễn phí (`>= free_cancel_hours`) tự động hoàn 100% vào ví; không bao giờ hoàn quá số tiền đã trả.

---

### T4.1 — Schema Wallet & Payments (Migration 010) + API đọc số dư & lịch sử sổ cái ví

- **Commit gợi ý:** `feat(wallet): add wallet and payment schema with balance and ledger read apis`
- **Depends on:** `T3.5`, `T1.6`
- **Modules / Files:** `BackEnd/db/migration/000010_create_wallet_payments.{up,down}.sql`, `BackEnd/db/query/wallet_payments.sql`, `internal/modules/wallet/`, `BackEnd/api/openapi.yaml`

#### Mục tiêu nghiệp vụ
Thiết lập toàn bộ lược đồ ví điện tử nội bộ, payment intent (hỗ trợ cả booking lẻ lẫn group booking), phân bổ thanh toán theo từng phòng con, biên lai ngân hàng SePay và bảng hoàn tiền, kèm API xem ví và sổ cái.

#### Phạm vi thực hiện
- Migration `000010_create_wallet_payments`:
  - `wallets`: `id uuid PK`, `customer_id uuid NN UQ FK -> customers`, `available_balance numeric(14,2) NN default 0 CHECK (available_balance >= 0)`, `held_balance numeric(14,2) NN default 0 CHECK (held_balance >= 0)`, `currency char(3) NN default 'VND'`, `created_at/updated_at/version`.
  - `wallet_ledger_entries` (append-only): `id uuid PK`, `wallet_id uuid NN FK -> wallets`, `entry_type varchar(32) NN CHECK (entry_type IN ('manual_deposit','sepay_deposit','late_transfer_credit','withdrawal_demo','hold_reserved','hold_released','payment_captured','refund_credited','invoice_payment'))`, `amount numeric(14,2) NN CHECK (amount > 0)`, `available_after numeric(14,2) NN CHECK (available_after >= 0)`, `held_after numeric(14,2) NN CHECK (held_after >= 0)`, `reference_type varchar(32) ?`, `reference_id uuid ?`, `idempotency_key varchar(128) NN UQ`, `note text ?`, `created_by uuid ?`, `created_at timestamptz NN default now()`.
  - `wallet_holds`: `id uuid PK`, `wallet_id uuid NN FK -> wallets`, `payment_intent_id uuid NN UQ`, `amount numeric(14,2) NN CHECK (amount > 0)`, `status varchar(24) NN CHECK (status IN ('active','captured','released','expired'))`, `expires_at timestamptz NN`, `created_at/updated_at`.
  - `payment_intents`: `id uuid PK`, `customer_id uuid NN FK -> customers`, `booking_id uuid ? FK -> bookings`, `booking_group_id uuid ? FK -> booking_groups`, `total_amount numeric(14,2) NN CHECK (total_amount > 0)`, `wallet_amount numeric(14,2) NN default 0 CHECK (wallet_amount >= 0)`, `bank_amount numeric(14,2) NN default 0 CHECK (bank_amount >= 0)`, `currency char(3) NN default 'VND'`, `bank_reference varchar(64) ? UQ`, `status varchar(32) NN CHECK (status IN ('created','awaiting_bank_transfer','paid','expired','cancelled','failed'))`, `expires_at timestamptz NN`, `idempotency_key varchar(128) NN`, `created_at/updated_at/version`, `CHECK ((booking_id IS NOT NULL) <> (booking_group_id IS NOT NULL))`, `CHECK (wallet_amount + bank_amount = total_amount)`. Unique `(customer_id, idempotency_key)`.
  - `bank_receipts`: `id uuid PK`, `provider varchar(32) NN default 'sepay'`, `provider_tx_id varchar(128) NN UQ`, `receiving_account varchar(64) NN`, `amount numeric(14,2) NN CHECK (amount > 0)`, `currency char(3) NN default 'VND'`, `parsed_reference varchar(64) ?`, `transfer_content text ?`, `raw_payload_hash bytea NN`, `status varchar(32) NN CHECK (status IN ('matched','late_credited_wallet','unmatched','rejected','reconciled'))`, `matched_payment_intent_id uuid ? FK -> payment_intents`, `credited_customer_id uuid ? FK -> customers`, `received_at timestamptz NN`, `reconciled_by uuid ?`, `reconciled_at timestamptz ?`, `reconcile_note text ?`.
  - `payment_allocations`: `id uuid PK`, `payment_intent_id uuid NN FK -> payment_intents`, `booking_id uuid NN FK -> bookings`, `wallet_ledger_entry_id uuid ? FK -> wallet_ledger_entries`, `bank_receipt_id uuid ? FK -> bank_receipts`, `amount numeric(14,2) NN CHECK (amount > 0)`, `source varchar(24) NN CHECK (source IN ('wallet','sepay'))`, `settled_at timestamptz NN default now()`. (Mỗi phòng con trong group có dòng allocation riêng ghi rõ số tiền đã trả cho phòng đó).
  - `refunds`: `id uuid PK`, `booking_id uuid NN FK -> bookings`, `customer_id uuid NN FK -> customers`, `wallet_ledger_entry_id uuid NN FK -> wallet_ledger_entries`, `amount numeric(14,2) NN CHECK (amount > 0)`, `reason text NN`, `trigger_type varchar(24) NN CHECK (trigger_type IN ('auto_free_cancel','staff_manual'))`, `idempotency_key varchar(128) NN UQ`, `created_by uuid ?`, `created_at timestamptz NN default now()`.
- Nối hook tạo `wallets` tự động (`available_balance = 0, held_balance = 0`) khi tạo `customers` mới (ở `T1.6`) + lazy ensure-wallet helper cho customer cũ.
- REST endpoints:
  - `GET /api/v1/wallet` (customer xem ví của mình; staff có quyền `wallet.read` xem theo `?customerId=...`).
  - `GET /api/v1/wallet/ledger` (cursor pagination, lọc theo `entryType`, khoảng thời gian; chỉ owner hoặc `wallet.read`).

#### Quy tắc & Invariant bắt buộc
- Không bao giờ xóa vật lý `wallet_ledger_entries`, `bank_receipts`, `payment_allocations`, `refunds`.
- `payment_intents` chỉ được gắn với **đúng 1 mục tiêu**: hoặc `booking_id`, hoặc `booking_group_id`.

#### Tiêu chí nghiệm thu & Kiểm thử
- Integration test (Testcontainers PostgreSQL): verify mọi constraint (`available_balance >= 0`, `wallet_amount + bank_amount = total_amount`, mutual exclusivity giữa `booking_id` & `booking_group_id`, unique `provider_tx_id` & `idempotency_key`); test `GET /api/v1/wallet` & `/ledger` kèm negative ownership (`403`).
- Chạy `cd BackEnd && go test -race ./internal/modules/wallet/...`.

---

### T4.2 — Nạp ví thủ công cho Admin (`POST /api/v1/wallet/deposits`) & Rút tiền ví minh họa (`POST /api/v1/wallet/withdrawals`)

- **Commit gợi ý:** `feat(wallet): add manual admin deposit and simulated customer withdrawal apis`
- **Depends on:** `T4.1`
- **Modules / Files:** `internal/modules/wallet/{domain,application,delivery/http}`, `BackEnd/api/openapi.yaml`

#### Mục tiêu nghiệp vụ
Cung cấp 2 thao tác trực tiếp trên ví phục vụ demo đồ án: Admin/Kế toán nạp tiền vào ví khách hàng (`manual_deposit`) và khách hàng rút tiền minh họa (`withdrawal_demo` — trừ số dư ví kèm ledger bất biến, không gọi chuyển tiền ngân hàng ra ngoài).

#### Phạm vi thực hiện
- REST endpoints:
  - `POST /api/v1/wallet/deposits` (`wallet.deposit`, bắt buộc `Idempotency-Key`):
    - Đầu vào: `customerId`, `amount (> 0)`, `note`.
    - Trong 1 transaction: kiểm tra `idempotency_key` (replay trả kết quả cũ), `SELECT ... FROM wallets WHERE customer_id = $1 FOR UPDATE`, cộng `available_balance += amount`, chèn `wallet_ledger_entries` (`entry_type='manual_deposit'`, `available_after`, `held_after`), ghi `audit_events`.
  - `POST /api/v1/wallet/withdrawals` (authenticated customer self-service, bắt buộc `Idempotency-Key`):
    - Đầu vào: `amount (> 0)`, `note` (tùy chọn, ví dụ mô tả tài khoản minh họa — **không** lưu dữ liệu nhạy cảm hay thực hiện payout ngân hàng).
    - Trong 1 transaction: kiểm tra `idempotency_key`, lock `wallets` (`FOR UPDATE`), kiểm tra **`available_balance >= amount`** (nếu không đủ trả `409 Conflict` mã `insufficient_wallet_balance`), trừ `available_balance -= amount`, chèn `wallet_ledger_entries` (`entry_type='withdrawal_demo'`), ghi `audit_events` và trả về số dư mới + mã giao dịch ledger.

#### Quy tắc & Invariant bắt buộc
- **Phạm vi đồ án đã chốt:** Rút tiền ví là **minh họa** — chỉ trừ `available_balance` và ghi sổ cái nội bộ, **không** tích hợp lệnh chuyển tiền ngân hàng hay SePay payout.
- Dưới truy cập đồng thời (ví dụ 5 request rút cùng lúc khi số dư chỉ đủ cho 1 request), `SELECT ... FOR UPDATE` + DB `CHECK (available_balance >= 0)` đảm bảo số dư không bao giờ âm.

#### Tiêu chí nghiệm thu & Kiểm thử
- Integration test (Testcontainers PostgreSQL):
  - Admin nạp 1.000.000đ → `available_balance` = 1.000.000đ; gửi trùng `Idempotency-Key` không cộng 2 lần.
  - Khách rút 400.000đ → còn 600.000đ, có ledger `withdrawal_demo`.
  - Concurrency test: số dư 500.000đ, bắn đồng thời 5 goroutine rút 300.000đ với 5 `Idempotency-Key` khác nhau → **đúng 1 goroutine thành công**, 4 goroutine nhận `409`, số dư cuối = 200.000đ.
  - Customer gọi `POST /api/v1/wallet/deposits` bị `403`.
- Chạy `cd BackEnd && go test -race ./internal/modules/wallet/...`.

---

### T4.3 — Payment Intent (Lẻ & Nhóm: Ví / SePay / Kết hợp) + Hủy & Job hết hạn nhả Wallet Hold

- **Commit gợi ý:** `feat(payment): implement individual and group payment intents with wallet hold and expiry worker`
- **Depends on:** `T4.2`, `T3.5`
- **Modules / Files:** `internal/modules/wallet/{domain,application,delivery/http,persistence/postgres}`, `BackEnd/api/openapi.yaml`

#### Mục tiêu nghiệp vụ
Cho phép khách hàng khởi tạo yêu cầu thanh toán 100% giá trị cho 1 booking lẻ hoặc 1 `booking_group` bằng 3 chế độ (chỉ ví, chỉ chuyển khoản SePay, hoặc kết hợp ví + SePay), tự động chốt booking khi trả đủ bằng ví, và tự động giải phóng tiền giữ trong ví khi intent bị hủy hoặc hết hạn.

#### Phạm vi thực hiện
- REST endpoints:
  - `POST /api/v1/payment-intents` (authenticated customer owner hoặc staff `payment.record`, bắt buộc `Idempotency-Key`):
    - Đầu vào: `bookingId` **hoặc** `bookingGroupId`, `walletAmount (>= 0)`. Server tự tính `totalAmount` từ booking (hoặc tổng các booking con `pending` của group) và `bankAmount = totalAmount - walletAmount` (`bankAmount >= 0`).
    - Validation: Booking/group phải đang ở trạng thái `pending` và chưa hết hạn hold; không cho tạo intent thứ 2 khi đã có 1 intent đang `awaiting_bank_transfer` (trả `409` yêu cầu hủy intent cũ trước).
    - **Chế độ 1 — Wallet-only (`bankAmount == 0`):** Trong **1 PostgreSQL transaction**: lock `wallets` (`FOR UPDATE`), lock các `bookings` theo thứ tự UUID, kiểm tra `available_balance >= totalAmount`, trừ trực tiếp `available_balance` (ghi ledger `hold_reserved` → `payment_captured` hoặc trực tiếp `payment_captured`), tạo `payment_intents` (`status='paid'`), tạo `payment_allocations` cho từng booking con đúng bằng `booking.total_amount`, gọi `booking.ConfirmPaidBookings` chuyển booking/group → `confirmed` + giải phóng Redis hold sau commit + enqueue email xác nhận vào `outbox_events`.
    - **Chế độ 2 — Bank-only (`walletAmount == 0, bankAmount > 0`):** Sinh mã tham chiếu duy nhất `bank_reference` (ví dụ `HMS<10 ký tự ngẫu nhiên viết hoa/số>`), tạo `payment_intents` (`status='awaiting_bank_transfer'`, `expires_at = min(now + intentTTL, booking.expires_at)`), trả về thông tin chuyển khoản (`receivingAccount`, `bankReference`, `bankAmount`, `expiresAt`).
    - **Chế độ 3 — Split (`walletAmount > 0, bankAmount > 0`):** Lock `wallets` (`FOR UPDATE`), kiểm tra `available_balance >= walletAmount`, chuyển `available_balance -= walletAmount`, `held_balance += walletAmount`, tạo `wallet_holds` (`status='active'`), ghi ledger `hold_reserved`, tạo `payment_intents` (`status='awaiting_bank_transfer'`, `bank_reference`), trả thông tin chuyển khoản phần `bankAmount` còn lại.
  - `POST /api/v1/payment-intents/{id}/cancel` (owner hoặc staff): Lock intent + wallet (`FOR UPDATE`), chỉ cho hủy khi `awaiting_bank_transfer`, chuyển `status='cancelled'`, nếu có `wallet_holds` đang `active` → hoàn `held_balance -= amount`, `available_balance += amount`, cập nhật `wallet_holds.status='released'`, ghi ledger `hold_released`.
- **Payment Intent Expiry Worker & Hook tích hợp Phase 3:**
  - Worker quét `payment_intents` đang `awaiting_bank_transfer` với `expires_at <= now()` (`FOR UPDATE SKIP LOCKED`), chuyển `status='expired'` và giải phóng `wallet_holds` về `available_balance` + ghi ledger `hold_released`.
  - Gắn vào hook `OnBookingCancelled` (`T3.6`) & `ExpiryScanner` (`T3.3`): khi booking lẻ hoặc 1 phòng con của nhóm bị hủy/hết hạn mà còn intent `awaiting_bank_transfer`, tự động hủy intent đó và trả tiền giữ về `available_balance` trong cùng transaction.

#### Quy tắc & Invariant bắt buộc
- Không bao giờ cộng hoặc trừ trùng tiền khi retry với cùng `Idempotency-Key`.
- Tiền đang giữ (`held_balance`) không thể dùng cho thanh toán khác hoặc rút ví.
- Thanh toán 1 `booking_group` luôn sinh đủ N dòng `payment_allocations` tương ứng từng phòng con để đảm bảo truy vết hoàn tiền lẻ chính xác.

#### Tiêu chí nghiệm thu & Kiểm thử
- Integration test (Testcontainers PostgreSQL):
  - Wallet-only cho booking lẻ & group 3 phòng: trừ đúng ví, tạo đủ 3 dòng `payment_allocations`, cả 3 phòng & group chuyển `confirmed`.
  - Split payment: `available_balance` giảm, `held_balance` tăng; hủy intent hoặc chờ worker hết hạn → `held_balance` về 0, `available_balance` hồi đủ, có đủ 2 dòng ledger `hold_reserved` và `hold_released`.
  - Hủy 1 phòng con khi group có split intent đang chờ → group intent tự hủy và tiền ví được giải phóng nguyên vẹn.
- Chạy `cd BackEnd && go test -race ./internal/modules/wallet/... ./internal/modules/booking/...`.

---

### T4.4 — Webhook SePay dòng tiền vào (`POST /api/v1/webhooks/sepay`), Xử lý chuyển trễ & Script giả lập local

- **Commit gợi ý:** `feat(payment): implement sepay incoming webhook settlement, late transfer credit and local simulator`
- **Depends on:** `T4.3`, `T0.5`
- **Modules / Files:** `internal/modules/wallet/{application,delivery/http}`, `BackEnd/scripts/simulate-sepay.sh`, `BackEnd/api/openapi.yaml`

#### Mục tiêu nghiệp vụ
Tiếp nhận và xác thực webhook chuyển khoản đến từ SePay, chốt thanh toán tự động (bao gồm capture wallet hold nếu thanh toán tách và confirm booking/group), xử lý an toàn giao dịch trùng lặp / chuyển khoản trễ (cộng vào ví khách hàng đúng 1 lần), kèm script giả lập để dễ test & demo đồ án.

#### Phạm vi thực hiện
- REST endpoint `POST /api/v1/webhooks/sepay` (xác thực riêng bằng provider auth, không dùng JWT):
  1. Kiểm tra header `Authorization: Apikey <SEPAY_WEBHOOK_SECRET>` bằng `subtle.ConstantTimeCompare` (sai → trả `401`, không ghi hiệu ứng tài chính).
  2. Parse payload chuẩn SePay (`id`, `gateway`, `transactionDate`, `accountNumber`, `transferType`, `transferAmount`, `content`, `code`). Chỉ chấp nhận `transferType == "in"` và `accountNumber == config.SepayReceivingAccount` (sai tài khoản/out → lưu `bank_receipts` trạng thái `rejected` và trả `200`).
  3. **Dedupe tuyệt đối:** Chèn `bank_receipts` với `provider_tx_id = strconv.FormatInt(payload.ID, 10)` (`UNIQUE`). Nếu trùng `provider_tx_id` → **trả `200` idempotent ngay lập tức, 0 thay đổi số dư/booking**.
  4. Trích xuất `parsed_reference` (từ `code` hoặc regex tìm `HMS[A-Z0-9]{10}` trong `content`) và tìm `payment_intents` tương ứng (`FOR UPDATE`):
     - **Trường hợp A — Khớp intent đang `awaiting_bank_transfer`, chưa hết hạn, và `transferAmount == intent.bank_amount` (khớp chính xác số tiền):**
       - Trong **1 PostgreSQL transaction**: lock `wallets` (nếu có `wallet_hold`) và lock tất cả `bookings` liên quan theo thứ tự UUID.
       - Nếu có `wallet_hold` đang `active`: trừ `held_balance -= wallet_amount`, đánh dấu `wallet_holds.status='captured'`, ghi ledger `payment_captured`.
       - Cập nhật `bank_receipts.status='matched'`, `payment_intents.status='paid'`.
       - Tạo `payment_allocations` phân bổ cho từng booking con (tổng allocation của mỗi booking con đúng bằng `booking.total_amount`, chia theo tỷ lệ/ưu tiên phần wallet và phần sepay).
       - Chuyển toàn bộ booking & group sang `confirmed`, ghi history + audit + enqueue email xác nhận vào `outbox_events`. (Post-commit: xóa Redis hold keys).
     - **Trường hợp B — Khớp `bank_reference` nhưng intent đã `expired`/`cancelled`, hoặc booking đã `expired`/`cancelled` (chuyển khoản trễ):**
       - **Không** tái sử dụng wallet hold cũ, **không** xác nhận lại booking đã hết hạn.
       - Cộng toàn bộ `transferAmount` vào `wallets.available_balance` của `intent.customer_id`, ghi ledger `late_transfer_credit` (dùng `idempotency_key = "sepay_late:" + provider_tx_id`), cập nhật `bank_receipts.status = 'late_credited_wallet'`, `credited_customer_id = intent.customer_id`, enqueue email thông báo tiền chuyển trễ đã được cộng vào ví khách hàng.
     - **Trường hợp C — Không tìm thấy `bank_reference` hoặc lệch số tiền (`transferAmount != intent.bank_amount`):**
       - Giữ nguyên intent/booking, lưu `bank_receipts.status = 'unmatched'` chờ nhân viên đối soát ở `T4.5`, ghi `audit_events`.
  5. Trả `200 OK` chỉ sau khi DB transaction đã commit bền vững.
- Tạo script `BackEnd/scripts/simulate-sepay.sh`: hỗ trợ chạy nhanh các kịch bản demo local (`--scenario valid|duplicate|late|mismatch|invalid-key --ref <BANK_REF> --amount <VND>`).

#### Quy tắc & Invariant bắt buộc
- Một giao dịch SePay (`provider_tx_id`) chỉ có tối đa **1 hiệu ứng tài chính duy nhất** dù SePay retry 10 lần.
- Không bao giờ log raw `Authorization` header hoặc secret.

#### Tiêu chí nghiệm thu & Kiểm thử
- Bộ test tài chính/webhook bắt buộc (Testcontainers PostgreSQL):
  - Sai `Apikey` → `401`, 0 dòng `bank_receipts`.
  - Bank-only & Split (wallet + SePay) cho cả booking lẻ và group 3 phòng: webhook khớp tiền → `held_balance` về 0, booking/group chuyển `confirmed`, đủ `payment_allocations`.
  - Gửi lặp lại cùng `provider_tx_id` lần 2 → `200`, 0 ledger mới, số dư & allocation giữ nguyên.
  - Intent hết hạn (wallet hold đã nhả) rồi mới nhận webhook trễ → booking vẫn `expired`, `transferAmount` cộng vào `available_balance` đúng 1 lần (`late_transfer_credit`), gửi lặp lần 2 không cộng thêm.
  - Chuyển sai số tiền hoặc sai mã tham chiếu → `bank_receipts.status = 'unmatched'`, 0 thay đổi ví/booking.
- Chạy `cd BackEnd && go test -race ./internal/modules/wallet/...`.

---

### T4.5 — Đối soát biên lai ngân hàng chưa khớp (`unmatched`) cho nhân viên

- **Commit gợi ý:** `feat(payment): add staff unmatched bank receipt review and manual reconciliation apis`
- **Depends on:** `T4.4`
- **Modules / Files:** `internal/modules/wallet/{application,delivery/http}`, `BackEnd/api/openapi.yaml`

#### Mục tiêu nghiệp vụ
Cho phép kế toán/admin xem danh sách các giao dịch ngân hàng chưa khớp (`unmatched` — ví dụ khách chuyển sai mã hoặc chuyển thiếu/dư tiền) và xử lý đối soát thủ công một cách idempotent, có ghi vết audit (cộng vào ví của một khách hàng cụ thể hoặc đánh dấu đã xử lý ngoài hệ thống).

#### Phạm vi thực hiện
- REST endpoints:
  - `GET /api/v1/admin/bank-receipts` (`payment.read`, lọc theo `status` mặc định `unmatched`, khoảng thời gian, cursor pagination; che thông tin nhạy cảm nếu có).
  - `POST /api/v1/admin/bank-receipts/{id}/reconcile` (`payment.reconcile`, bắt buộc `Idempotency-Key`):
    - Hành động hỗ trợ: `credit_customer_wallet` (chọn `customerId` + `note`) hoặc `mark_resolved` (kèm `note`).
    - Trong 1 transaction: lock `bank_receipts` (`FOR UPDATE`), kiểm tra `status == 'unmatched'` (nếu đã `matched`/`reconciled`/`late_credited_wallet` trả `409`); nếu `credit_customer_wallet` → lock `wallets` (`FOR UPDATE`), cộng `bank_receipts.amount` vào `available_balance`, ghi ledger `sepay_deposit` (idempotency theo receipt ID); cập nhật `bank_receipts.status = 'reconciled'`, `reconciled_by`, `reconciled_at`, `reconcile_note`, ghi `audit_events` + `outbox_events`.

#### Quy tắc & Invariant bắt buộc
- Một `bank_receipt` chỉ được đối soát đúng **1 lần**, không bao giờ cộng ví 2 lần kể cả khi 2 nhân viên bấm cùng lúc.

#### Tiêu chí nghiệm thu & Kiểm thử
- Integration test: tạo 1 receipt `unmatched`, nhân viên có `payment.reconcile` đối soát cộng ví khách hàng thành công; gọi lại lần 2 bị `409`; nhân viên lễ tân không có `payment.reconcile` bị `403`.
- Chạy `cd BackEnd && go test -race ./internal/modules/wallet/...`.

---

### T4.6 — Hoàn tiền về ví (`POST /api/v1/refunds`) & Nối tự động hoàn tiền khi hủy miễn phí

- **Commit gợi ý:** `feat(payment): add wallet refund api and automatic free-cancellation refund hook`
- **Depends on:** `T4.4`, `T3.6`
- **Modules / Files:** `internal/modules/wallet/{domain,application,delivery/http}`, `internal/modules/booking/` (wire cancel hook), `BackEnd/api/openapi.yaml`

#### Mục tiêu nghiệp vụ
Cho phép nhân viên có quyền hoàn tiền vào ví khách hàng (chống hoàn vượt số tiền đã trả cho từng booking) và nối trực tiếp cơ chế **tự động hoàn 100% về ví** khi hủy booking `confirmed` trước hạn `free_cancel_hours` (mặc định 48h).

#### Phạm vi thực hiện
- REST endpoint `POST /api/v1/refunds` (`payment.refund`, bắt buộc `Idempotency-Key`):
  - Đầu vào: `bookingId`, `amount (> 0)`, `reason`.
  - Trong 1 PostgreSQL transaction:
    1. Kiểm tra `idempotency_key` (replay trả bản ghi refund cũ).
    2. Lock `bookings` (`FOR UPDATE`) và `wallets` (`FOR UPDATE`).
    3. Tính `settled_amount` của `bookingId` = tổng `payment_allocations.amount` của booking đó (+ `invoice_payments` nếu có sau Phase 5), và `prior_refunded` = tổng `refunds.amount` đã hoàn cho booking đó.
    4. Kiểm tra **`amount <= settled_amount - prior_refunded`** (vượt quá → trả `409 Conflict` mã `refund_exceeds_settled`).
    5. Cộng `wallets.available_balance += amount`, chèn `wallet_ledger_entries` (`entry_type='refund_credited'`), chèn `refunds` (`trigger_type='staff_manual'`), ghi `audit_events` + enqueue email thông báo hoàn tiền vào `outbox_events`.
- **Nối Auto-Refund Hook vào `T3.6` (Single Cancel & Batch Cancel):**
  - Khi hủy booking `confirmed` thỏa điều kiện `now <= check_in_date - free_cancel_hours` (theo timezone khách sạn): trong **cùng transaction hủy booking**, tự động tính `refundable = settled_amount - prior_refunded`; nếu `refundable > 0` thì gọi hàm refund nội bộ (`trigger_type='auto_free_cancel'`, `idempotency_key="auto_cancel:"+booking.ID`) hoàn 100% vào ví khách hàng.
  - Nếu hủy sát giờ (`< free_cancel_hours`): hủy booking thành công nhưng **không** tự động hoàn tiền (nhân viên vẫn có thể hoàn thủ công qua `POST /api/v1/refunds` nếu được duyệt).

#### Quy tắc & Invariant bắt buộc
- Hoàn tiền chỉ cộng vào ví khách hàng, **không** tạo lệnh chuyển khoản ngân hàng ra ngoài.
- Với group booking 3 phòng đã thanh toán chung 1 lần: hủy lẻ 1 phòng đủ điều kiện miễn phí chỉ hoàn đúng phần `payment_allocations` của riêng phòng con đó, không ảnh hưởng 2 phòng còn lại.

#### Tiêu chí nghiệm thu & Kiểm thử
- Integration test (Testcontainers PostgreSQL):
  - Hủy booking lẻ `confirmed` trước check-in 72h (≥ 48h) → booking chuyển `cancelled`, ví khách được cộng đúng 100% số tiền đã trả trong cùng transaction.
  - Hủy booking lẻ `confirmed` trước check-in 12h (< 48h) → booking `cancelled`, số dư ví không đổi; nhân viên gọi `POST /api/v1/refunds` hoàn 50% thành công, gọi tiếp 60% bị từ chối `409` (chống over-refund).
  - Group booking 3 phòng thanh toán 1 lần (3.000.000đ, mỗi phòng 1.000.000đ) → hủy lẻ phòng 1 trước 72h → hoàn đúng 1.000.000đ về ví, phòng 2 & 3 giữ nguyên `confirmed`.
- Chạy `cd BackEnd && go test -race ./internal/modules/wallet/... ./internal/modules/booking/...`.
