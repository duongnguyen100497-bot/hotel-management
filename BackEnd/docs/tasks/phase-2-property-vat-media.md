# Phase 2 — Khách sạn, VAT, Phòng, Dịch vụ & Ảnh MinIO (Property, Pricing & Media)

> **Mục tiêu Phase:** Xây dựng toàn bộ dữ liệu gốc của khách sạn đơn lẻ: cấu hình khách sạn (bao gồm **TTL giữ phòng lẻ/nhóm** và **giờ hủy miễn phí** do Admin tùy chỉnh), quản lý mức thuế VAT động + hàm tính báo giá (chưa gồm VAT, không phí dịch vụ), danh mục loại phòng/phòng vật lý + tìm kiếm công khai, quản lý trạng thái vận hành phòng (có bảo trì loại trừ khỏi mẫu số công suất), danh mục dịch vụ và module Media kết nối MinIO an toàn.
>
> **Phụ thuộc:** Hoàn thành Phase 1 (`T1.4`, `T1.5`).
>
> **Tiêu chí hoàn thành Phase (Exit Criteria):**
> - Admin cấu hình được thông tin khách sạn, tùy chỉnh `individual_hold_ttl_minutes`, `group_hold_ttl_minutes`, `free_cancel_hours`, và quản lý mức thuế `vat_rates`.
> - Admin/Staff tạo được loại phòng, phòng vật lý, chuyển trạng thái phòng (`available`/`cleaning`/`maintenance`) kèm lịch sử, và quản lý danh mục dịch vụ.
> - API công khai `GET /api/v1/catalog/room-types` trả về danh sách loại phòng kèm ảnh từ MinIO và lọc được theo sức chứa/giá.
> - Luồng upload ảnh MinIO hoàn chỉnh: xin presigned PUT URL → upload ảnh thật lên Docker MinIO → complete xác thực magic bytes/kích thước → gắn ảnh cover cho phòng → xóa ảnh có worker dọn object.

---

### T2.1 — Cấu hình khách sạn (TTL giữ phòng tùy chỉnh), quản lý thuế VAT (Migration 005) & Pricing Quote Helper

- **Commit gợi ý:** `feat(pricing): add hotel settings with configurable hold ttl, vat rates crud and quote calculator`
- **Depends on:** `T1.4`, `T0.5`
- **Modules / Files:** `BackEnd/db/migration/000005_create_settings_vat.{up,down}.sql`, `BackEnd/db/query/{property,pricing}.sql`, `internal/modules/property/`, `internal/modules/pricing/`, `BackEnd/api/openapi.yaml`

#### Mục tiêu nghiệp vụ
Cho phép Admin cấu hình các thông số vận hành cốt lõi của khách sạn (đặc biệt là **TTL giữ phòng lẻ/nhóm** và **số giờ hủy miễn phí**) và quản lý danh mục thuế VAT động để mọi báo giá/booking/hóa đơn đều lấy giá trị từ server.

#### Phạm vi thực hiện
- Migration `000005_create_settings_vat`:
  - `hotel_settings`: `id boolean PK default true CHECK (id)` (đảm bảo single-hotel 1 dòng duy nhất), `name text NN`, `address text NN`, `phone text NN`, `email text NN`, `timezone text NN default 'Asia/Ho_Chi_Minh'`, `currency char(3) NN default 'VND'`, `check_in_time time NN default '14:00'`, `check_out_time time NN default '12:00'`, `individual_hold_ttl_minutes int NN default 15 CHECK (individual_hold_ttl_minutes BETWEEN 1 AND 120)`, `group_hold_ttl_minutes int NN default 30 CHECK (group_hold_ttl_minutes BETWEEN 1 AND 180)`, `free_cancel_hours int NN default 48 CHECK (free_cancel_hours >= 0)`, `logo_media_id uuid ?`, `updated_at/version`. Seed sẵn 1 dòng mặc định (`ON CONFLICT DO NOTHING`).
  - `vat_rates`: `id uuid PK`, `code varchar(32) NN UQ`, `name text NN`, `rate numeric(5,4) NN CHECK (rate >= 0 AND rate <= 1)`, `is_active boolean NN default false`, `created_at/updated_at/version`. Partial unique index `WHERE is_active = true` bảo đảm tại một thời điểm chỉ có tối đa **1 mức VAT đang active**. Seed sẵn 1 mức mặc định (ví dụ `VAT_10`, `0.1000`, `is_active = true`).
- REST endpoints:
  - `GET /api/v1/hotel/settings` (`hotel_settings.read` hoặc public cho các field hiển thị cơ bản như tên/giờ check-in/out).
  - `PUT /api/v1/hotel/settings` (`hotel_settings.update`, yêu cầu `version` optimistic lock): Cho phép Admin cập nhật thông tin khách sạn và **tùy chỉnh `individual_hold_ttl_minutes`, `group_hold_ttl_minutes`, `free_cancel_hours`**; ghi `audit_events` (before/after).
  - `GET /api/v1/vat-rates` (`vat.manage` / `property.read`), `POST /api/v1/vat-rates` (`vat.manage`), `PATCH /api/v1/vat-rates/{id}` (`vat.manage` — kích hoạt mức mới sẽ tắt mức cũ trong cùng 1 transaction), `POST /api/v1/vat-rates/{id}/deactivate` (`vat.manage`).
- Exported interface `pricing.QuoteCalculator`:
  - Nhận `subtotal` (decimal) + `activeVAT` → tính chính xác bằng decimal làm tròn chuẩn VND: `vat_amount = subtotal * vat_rate`, `total = subtotal + vat_amount` (**giá niêm yết chưa gồm VAT, không có phí dịch vụ riêng**), trả về `PricingSnapshot{VatRateID, VatRate, Subtotal, VatAmount, TotalAmount, Currency}`.

#### Quy tắc & Invariant bắt buộc
- Không xóa cứng `vat_rates` (chỉ deactivate); giá trị VAT cũ đã chụp snapshot ở booking/hóa đơn không bao giờ bị ảnh hưởng khi Admin đổi VAT mới.
- Thay đổi `individual_hold_ttl_minutes` / `group_hold_ttl_minutes` chỉ áp dụng cho hold tạo **sau thời điểm cập nhật**, không làm thay đổi TTL của các hold đang tồn tại.

#### Tiêu chí nghiệm thu & Kiểm thử
- Unit test `pricing.QuoteCalculator` với VAT `0%`, `8%`, `10%`, kiểm tra độ chính xác decimal không dùng float.
- Integration test: cập nhật `individual_hold_ttl_minutes` = 20, `group_hold_ttl_minutes` = 45 thành công và ghi audit; giá trị ngoài biên (`0` hoặc `-5`) trả `422`; kích hoạt mức VAT thứ 2 tự động tắt mức thứ 1 trong cùng transaction; conflict `version` trả `409`.
- Chạy `cd BackEnd && go test -race ./internal/modules/property/... ./internal/modules/pricing/...`.

---

### T2.2 — Loại phòng, Phòng vật lý (Migration 006), Staff CRUD & Tìm kiếm Catalogue công khai

- **Commit gợi ý:** `feat(property): add room types, physical rooms crud and public catalog search`
- **Depends on:** `T2.1`
- **Modules / Files:** `BackEnd/db/migration/000006_create_rooms.{up,down}.sql`, `BackEnd/db/query/rooms.sql`, `internal/modules/property/`, `BackEnd/api/openapi.yaml`

#### Mục tiêu nghiệp vụ
Quản lý danh mục loại phòng (`room_types`) và từng phòng vật lý (`rooms`) với khóa lạc quan chống ghi đè, đồng thời mở API tìm kiếm công khai cho khách xem loại phòng và giá.

#### Phạm vi thực hiện
- Migration `000006_create_rooms`:
  - `room_types`: `id uuid PK`, `code varchar(64) NN UQ`, `name text NN`, `description text ?`, `base_nightly_rate numeric(14,2) NN CHECK (base_nightly_rate > 0)`, `currency char(3) NN default 'VND'`, `capacity smallint NN CHECK (capacity > 0)`, `area_sqm numeric(6,2) ?`, `bed_type text ?`, `amenities jsonb NN default '[]'`, `is_bookable boolean NN default true`, `archived_at timestamptz ?`, `created_at/updated_at/version`.
  - `rooms`: `id uuid PK`, `code varchar(32) NN UQ`, `name text NN`, `room_type_id uuid NN FK -> room_types`, `floor smallint NN`, `capacity smallint NN CHECK (capacity > 0)`, `operational_status varchar(24) NN default 'available' CHECK (operational_status IN ('available','occupied','cleaning','maintenance'))`, `description text ?`, `archived_at timestamptz ?`, `created_at/updated_at/version`.
  - `room_state_history`: `id uuid PK`, `room_id uuid NN FK -> rooms`, `from_status varchar(24) NN`, `to_status varchar(24) NN`, `reason text ?`, `actor_user_id uuid ?`, `occurred_at timestamptz NN default now()`. Index `(room_id, occurred_at DESC)`.
- REST endpoints:
  - Public: `GET /api/v1/catalog/room-types` (lọc theo `capacity`, `minPrice`, `maxPrice`, allowlisted sort, cursor pagination; tham số ngày `checkIn`/`checkOut` sẽ nối thêm số phòng trống thực tế ở `T3.1`), `GET /api/v1/catalog/room-types/{code}`.
  - Staff Room Types: `GET /api/v1/room-types` (`property.read`), `POST /api/v1/room-types`, `PATCH /api/v1/room-types/{id}` (`room_type.manage`, kiểm tra `version`/`If-Match`).
  - Staff Rooms: `GET /api/v1/rooms` (`property.read`), `POST /api/v1/rooms` (`room.create`), `PATCH /api/v1/rooms/{id}` (`room.update`, kiểm tra `version`/`If-Match`), `POST /api/v1/rooms/{id}/archive` (`room.archive` — không cho archive nếu phòng đang `occupied`).

#### Quy tắc & Invariant bắt buộc
- CRUD `PATCH /api/v1/rooms/{id}` **không được phép** sửa trực tiếp `operational_status` (phải đi qua API chuyển trạng thái có kiểm soát ở `T2.3` hoặc Stay Ops ở Phase 5).
- Giá niêm yết `base_nightly_rate` là giá 1 đêm chưa gồm VAT; sửa giá loại phòng không bao giờ thay đổi snapshot của booking cũ.

#### Tiêu chí nghiệm thu & Kiểm thử
- Integration test: CRUD room_type & room, optimistic lock `version` conflict trả `409`, mã phòng trùng trả `409`, public catalog chỉ trả `is_bookable = true` và `archived_at IS NULL`.
- Negative RBAC test: khách hàng hoặc lễ tân không có `room.create` gọi `POST /api/v1/rooms` trả `403`.
- Chạy `cd BackEnd && go test -race ./internal/modules/property/...`.

---

### T2.3 — Chuyển trạng thái vận hành phòng & Bảo trì (Maintenance State Transitions)

- **Commit gợi ý:** `feat(property): add controlled room operational state and maintenance transitions`
- **Depends on:** `T2.2`
- **Modules / Files:** `internal/modules/property/{domain,application,delivery/http}`, `BackEnd/api/openapi.yaml`

#### Mục tiêu nghiệp vụ
Cung cấp luồng chuyển trạng thái vận hành phòng có kiểm soát (`available` ↔ `maintenance`, `cleaning` → `available`) kèm ghi nhận lịch sử bất biến để phục vụ vận hành và làm căn cứ loại trừ phòng bảo trì khỏi mẫu số báo cáo công suất.

#### Phạm vi thực hiện
- State machine hợp lệ cho thao tác trực tiếp của Property:
  - `available → maintenance` (yêu cầu `reason`).
  - `maintenance → available` (hoàn tất bảo trì).
  - `cleaning → available` (hoàn tất dọn phòng — dùng chung với endpoint `POST /api/v1/rooms/{id}/ready`).
  - Các chuyển đổi `available → occupied` và `occupied → cleaning` chỉ được gọi qua exported application interface từ module `stayops` (Phase 5), không mở cho API thủ công.
- REST endpoints:
  - `POST /api/v1/rooms/{id}/maintenance` (`room.state.update`): Đưa phòng vào bảo trì (`enter`) hoặc kết thúc bảo trì (`exit`), ghi `room_state_history` và `audit_events` trong cùng 1 transaction.
  - `POST /api/v1/rooms/{id}/ready` (`room.state.update`): Chuyển `cleaning` (hoặc `maintenance`) về `available`.
  - `GET /api/v1/rooms/{id}/state-history` (`property.read`): Xem lịch sử trạng thái phòng.
- Exported interface `property.RoomStateService` cho Phase 5 (`MarkOccupied`, `MarkCleaning`) và Phase 6 (`CountOperationalRoomsAt` / truy vấn phòng đang bảo trì).

#### Quy tắc & Invariant bắt buộc
- Cấm chuyển phòng đang ở trạng thái `occupied` sang `maintenance` hoặc `available` (trả `409`).
- Mọi thay đổi trạng thái phòng bắt buộc ghi 1 dòng vào `room_state_history` (phục vụ truy vết & backfill snapshot công suất ở `T6.1`) và 1 dòng `audit_events`.

#### Tiêu chí nghiệm thu & Kiểm thử
- Unit test state machine (mọi transition hợp lệ và không hợp lệ).
- Integration test: chuyển `available → maintenance → available` ghi đủ 2 dòng `room_state_history` + audit; thử đưa phòng `occupied` vào `maintenance` trả `409` và không đổi DB.
- Chạy `cd BackEnd && go test -race ./internal/modules/property/...`.

---

### T2.4 — Danh mục dịch vụ khách sạn (Migration 007 — Service Catalog)

- **Commit gợi ý:** `feat(property): add service catalog crud and deactivation protection`
- **Depends on:** `T2.1`
- **Modules / Files:** `BackEnd/db/migration/000007_create_service_catalog.{up,down}.sql`, `BackEnd/db/query/service_catalog.sql`, `internal/modules/property/` (hoặc `internal/modules/servicecatalog/`), `BackEnd/api/openapi.yaml`

#### Mục tiêu nghiệp vụ
Quản lý danh mục dịch vụ phụ trợ của khách sạn (ăn uống, giặt ủi, minibar, đưa đón…) để lễ tân ghi nhận tiêu thụ trong quá trình lưu trú và xuất hóa đơn check-out ở Phase 5.

#### Phạm vi thực hiện
- Migration `000007_create_service_catalog`:
  - `service_catalog`: `id uuid PK`, `code varchar(64) NN UQ`, `name text NN`, `category varchar(64) NN`, `unit_price numeric(14,2) NN CHECK (unit_price >= 0)`, `currency char(3) NN default 'VND'`, `description text ?`, `is_active boolean NN default true`, `created_at/updated_at/version`.
- REST endpoints:
  - `GET /api/v1/services` (`service.read`, lọc theo `category`, `isActive`, cursor pagination).
  - `POST /api/v1/services` (`service.manage`), `PATCH /api/v1/services/{id}` (`service.manage` với `version` optimistic lock).
  - `POST /api/v1/services/{id}/deactivate` (`service.manage`): Tắt `is_active = false` thay vì xóa cứng, ghi `audit_events`.
- Exported read interface `GetActiveService(ctx, id)` cung cấp giá/tên chuẩn cho `stayops` chụp snapshot ở `T5.2`.

#### Quy tắc & Invariant bắt buộc
- Không có API xóa cứng dịch vụ; dịch vụ đã deactivate không thể thêm mới vào stay nhưng các dòng hóa đơn/tiêu thụ cũ vẫn giữ nguyên snapshot.
- Giá dịch vụ là giá **chưa gồm VAT**.

#### Tiêu chí nghiệm thu & Kiểm thử
- Integration test: tạo, cập nhật, lọc theo category/active, deactivate dịch vụ, kiểm tra `409` trùng code hoặc sai `version`, negative RBAC (`403`).
- Chạy `cd BackEnd && go test -race ./internal/modules/...`.

---

### T2.5 — Module Media & Tích hợp MinIO cho ảnh phòng (Migration 008)

- **Commit gợi ý:** `feat(media): add minio presigned upload, magic-byte validation, room linking and cleanup worker`
- **Depends on:** `T2.2`, `T0.5`
- **Modules / Files:** `BackEnd/db/migration/000008_create_media.{up,down}.sql`, `BackEnd/db/query/media.sql`, `internal/platform/minio/`, `internal/modules/media/`, `internal/modules/property/` (expose room images), `BackEnd/api/openapi.yaml`

#### Mục tiêu nghiệp vụ
Thay thế hoàn toàn ảnh tĩnh/Unsplash bằng quy trình lưu trữ ảnh phòng thực tế trên MinIO (presigned upload trực tiếp, kiểm tra định dạng thật phía server, gắn ảnh vào phòng/loại phòng, chọn ảnh bìa và dọn rác object mồ côi).

#### Phạm vi thực hiện
- Migration `000008_create_media`:
  - `media_assets`: `id uuid PK`, `object_key text NN UQ`, `sha256 bytea ?`, `mime_type varchar(64) ?`, `byte_size bigint ?`, `width int ?`, `height int ?`, `status varchar(24) NN CHECK (status IN ('pending','ready','deletion_pending','deleted'))`, `uploaded_by uuid ?`, `created_at/updated_at`.
  - `room_media`: `room_id uuid ? FK -> rooms`, `room_type_id uuid ? FK -> room_types`, `media_asset_id uuid NN FK -> media_assets`, `sort_order smallint NN default 0`, `is_cover boolean NN default false`, `CHECK ((room_id IS NOT NULL) <> (room_type_id IS NOT NULL))` (gắn cho phòng hoặc loại phòng).
  - Thêm FK `hotel_settings.logo_media_id -> media_assets(id)`.
- `internal/platform/minio`: Adapter bọc `minio-go/v7` (GeneratePresignedPut, StatObject, GetObject, RemoveObject, GeneratePresignedGet / PublicURL).
- REST endpoints:
  - `POST /api/v1/media/uploads` (`media.manage`): Validate tên/content-type khai báo, sinh `object_key` ngẫu nhiên (`uploads/<uuid>` — không dùng tên file client gửi để chống path traversal), tạo dòng `media_assets` trạng thái `pending`, trả về `mediaId` + presigned PUT URL ngắn hạn (5–10 phút).
  - `POST /api/v1/media/{id}/complete` (`media.manage`): Gọi MinIO đọc object thực tế, kiểm tra dung lượng `<= UPLOAD_MAX_BYTES` (mặc định 5 MiB), phát hiện **magic bytes** (`http.DetectContentType` + decode ảnh với `image/jpeg`, `image/png`, `golang.org/x/image/webp`; **từ chối tuyệt đối SVG/HTML/executable**), lấy `width`/`height` và `sha256`, chuyển `status = 'ready'`, tùy chọn gắn ngay vào `room_id`/`room_type_id`/`is_cover` trong 1 transaction.
  - `POST /api/v1/rooms/{id}/media` / `PATCH /api/v1/rooms/{id}/media/{mediaId}` (`media.manage`): Liên kết ảnh đã `ready`, đổi thứ tự / đặt ảnh cover.
  - `DELETE /api/v1/media/{id}` (`media.manage`): Gỡ liên kết DB, chuyển `status = 'deletion_pending'`, enqueue sự kiện xóa MinIO vào `outbox_events` để worker xóa object thật trên MinIO (retry nếu lỗi) rồi đánh dấu `deleted`.
- Reconciliation job nhẹ: định kỳ dọn các `media_assets` ở trạng thái `pending` quá 1 giờ (xóa object trên MinIO nếu có + đánh dấu `deleted`).

#### Quy tắc & Invariant bắt buộc
- Không bao giờ trả MinIO credentials hay cho client chỉ định `object_key`.
- Chỉ dựa vào magic bytes + decode bitmap phía server, tuyệt đối không tin header `Content-Type` do client gửi.
- Không làm malware/antivirus scan (đã cắt bỏ theo phạm vi đồ án).

#### Tiêu chí nghiệm thu & Kiểm thử
- Integration test (Testcontainers MinIO + PostgreSQL):
  - Upload ảnh PNG/JPEG hợp lệ qua presigned URL → gọi `complete` thành công (`status='ready'`, có đúng `width`/`height`/`sha256`) → gắn vào phòng → `GET /api/v1/catalog/room-types` trả về URL ảnh.
  - Upload file text/SVG giả mạo đuôi `.png` hoặc file vượt quá giới hạn dung lượng → `complete` trả `422` và không chuyển `ready`.
  - Xóa ảnh → gỡ link DB + outbox worker xóa sạch object khỏi MinIO.
- Chạy `cd BackEnd && go test -race ./internal/modules/media/...`.
