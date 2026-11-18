# Hotel Management System – Frontend

ReactJS + Vite + Tailwind CSS + React Router DOM + Axios + Lucide React + Recharts.
Dữ liệu là Mock Data (lưu trong bộ nhớ), chưa cần Backend.

## Chạy dự án

```bash
npm install
npm run dev      # http://localhost:5173
npm run build    # build production
```

Yêu cầu Node.js >= 18.

## Địa chỉ truy cập

| Khu vực                | URL                                   |
| ---------------------- | ------------------------------------- |
| Website khách hàng     | `/` (và `/rooms`, `/login`, ...)      |
| Trang quản trị (Admin) | `/admin/login` → `/admin/dashboard`   |

Admin nằm dưới tiền tố `/admin` để không trùng URL với website khách hàng (cấu hình ở `src/shared/appModes.js`, `src/admin/routes/paths.js`).

## Tài khoản demo (Admin)

| Vai trò        | Tên đăng nhập | Mật khẩu    |
| -------------- | ------------- | ----------- |
| Quản trị viên  | admin         | admin123    |
| Quản lý        | manager       | manager123  |
| Lễ tân         | letan         | letan123    |
| Kế toán        | ketoan        | ketoan123   |

## Website khách hàng

Là một chế độ riêng của dự án (`src/customer`), dùng chung dữ liệu với Admin (phòng, giá, đặt phòng, khách hàng) nhưng không dùng code giao diện của Admin.

| Trang                  | URL                         | Ghi chú                                                        |
| ---------------------- | --------------------------- | -------------------------------------------------------------- |
| Trang chủ              | `/`                         | Hero, tìm phòng, phòng nổi bật, tiện ích, ưu đãi, đánh giá     |
| Danh sách phòng        | `/rooms`                    | Lọc giá / loại / tiện nghi / còn trống, sắp xếp; lưu trên URL  |
| Chi tiết phòng         | `/rooms/:id`                | `:id` là hạng phòng (`deluxe`...); thư viện ảnh, tóm tắt giá   |
| Đặt phòng              | `/booking/:roomId`          | 5 bước; **bắt buộc đăng nhập**                                 |
| Đăng nhập / Đăng ký    | `/login`, `/register`       |                                                                |
| Tài khoản              | `/account`                  | Tổng quan, hồ sơ, đổi mật khẩu; cần đăng nhập                  |
| Đặt phòng của tôi      | `/my-bookings`              | Chi tiết, hóa đơn, hủy theo chính sách, hướng dẫn nhận phòng   |
| Liên hệ và hỗ trợ      | `/contact`                  | Thông tin, bản đồ minh họa, form hỗ trợ, FAQ                   |

**Tài khoản khách dùng thử:** `demo@grandnavy.vn` / `Demo@123` (gắn với khách KH001 có sẵn lịch sử đặt phòng).

Cách hoạt động cần biết:

- Khách chọn **hạng phòng**; hệ thống tự gán phòng cụ thể còn trống (P101, P203...). Đặt N phòng tạo N booking `Pending`, cùng `groupId`, hiện ngay ở Admin (nhãn `[Đặt trực tuyến]` trong ghi chú).
- Chống đặt trùng: kiểm tra lại lúc gửi, `bookingService` chặn trùng lịch theo phòng, thiếu phòng thì hoàn tác các phòng đã giữ.
- **Không giả lập thanh toán**: chỉ có "Thanh toán tại khách sạn" và "Chuyển khoản" (trạng thái luôn *Chưa thanh toán*); thẻ trực tuyến bị khóa vì chưa có cổng thanh toán.
- Khách đăng ký và đặt phòng trực tuyến được lưu ở `localStorage` (`shared/services/onlineDataStore.js`) và nạp lại khi khởi động. Sửa đổi của Admin lên các bản ghi này chỉ được lưu từ lần khách thao tác kế tiếp.
- Mật khẩu khách được băm SHA-256 + salt ở trình duyệt, **chỉ là cơ chế demo**; khi có Backend phải xác thực ở máy chủ.
- Phiên khách và phiên Admin tách biệt (`hms_customer_session` / `hms_session`); khách không vào được `/admin`.
- Ảnh phòng và ảnh trang chủ là **ảnh Unsplash** nhúng trực tiếp (`customer/data/photos.js`, cần Internet). Ảnh lỗi tự quay về hình minh họa SVG (`customer/components/scenes.jsx`). Dùng ảnh của khách sạn: đặt file vào `public/images/` rồi đổi đường dẫn trong `photos.js`.

**Giá trị mặc định cần khách sạn xác nhận** (`customer/data/bookingPolicy.js`): VAT 10%, phí dịch vụ 5%, hủy miễn phí trước giờ nhận phòng 48 giờ. Đánh giá, ưu đãi và thư viện ảnh ở trang chủ là nội dung mẫu (`customer/data/homeContent.js`). Khuyến mãi theo hạng phòng (`promoPercent`) đang tắt (0).

Chưa có: trang "Đánh giá của tôi", gửi email thật (quên mật khẩu, xác nhận đặt phòng), thanh toán thẻ trực tuyến, form hỗ trợ gửi tới nhân viên (hiện chỉ lưu trong trình duyệt).

## Chức năng (Admin)

- **Đăng nhập**: ghi nhớ đăng nhập, quên mật khẩu, tài khoản demo.
- **Dashboard**: 8 chỉ số, biểu đồ doanh thu / công suất / trạng thái phòng, danh sách đặt phòng, check-in, check-out, hoạt động gần đây.
- **Phòng**: bảng + Room Grid theo tầng, tìm kiếm, lọc tầng / loại / trạng thái, thêm / sửa / xóa, đổi trạng thái nhanh.
- **Đặt phòng**: tạo / sửa / hủy, kiểm tra trùng lịch, lọc theo ngày và trạng thái.
- **Khách hàng**: CRUD, lịch sử đặt phòng, chặn xóa khi còn đặt phòng hiệu lực.
- **Nhận / Trả phòng**: check-in, check-out kèm dịch vụ và phụ thu, tự tạo hóa đơn, phòng chuyển sang "Đang dọn dẹp".
- **Dịch vụ**: CRUD, bật / tắt hoạt động, lọc theo loại.
- **Hóa đơn**: xem chi tiết, in hóa đơn, xác nhận thanh toán, thống kê đã thu / chưa thu.
- **Báo cáo**: doanh thu, công suất phòng, khách hàng, dịch vụ; lọc theo ngày / tháng / năm; xuất Excel (CSV) và PDF (in).
- **Nhân viên**: CRUD, lọc theo chức vụ / trạng thái, không cho xóa tài khoản đang đăng nhập.
- **Cài đặt**: thông tin khách sạn và logo, hồ sơ tài khoản, đổi mật khẩu, phân quyền giao diện theo vai trò.

## Cấu trúc: hai chế độ trong một dự án

Dự án có hai chế độ độc lập, chạy cùng một lệnh `npm run dev`. Chọn chế độ theo URL (`src/App.jsx`): đường dẫn bắt đầu bằng `/admin` là Admin, còn lại là website khách hàng. Mỗi chế độ được tải riêng, chỉ tải code của chế độ đang dùng.

```
src/
  main.jsx, App.jsx           (khởi động + chọn chế độ theo URL)
  index.css                   (Tailwind + lớp giao diện Admin)
  customer/                   Chế độ WEBSITE KHÁCH HÀNG
    CustomerApp.jsx, layout/  (Header + Footer riêng, không có Sidebar)
    pages/ components/ routes/ contexts/ (phiên đăng nhập khách)
    services/ (phòng theo hạng, đặt phòng, tài khoản khách)
    data/ (nội dung, catalog phòng, ảnh, chính sách)  styles/customer.css
  admin/                      Chế độ QUẢN TRỊ
    AdminApp.jsx, components/{common,layout,dashboard}
    pages/ routes/ contexts/ (phiên đăng nhập nhân viên)
    services/ hooks/ utils/ (phân quyền, xuất báo cáo...)
  shared/                     Dùng chung, không phụ thuộc chế độ nào
    services/ (dữ liệu mẫu: phòng, đặt phòng, khách hàng, hóa đơn)
    contexts/ (Cài đặt khách sạn, Toast)  hooks/ utils/  appModes.js
```

Quy tắc ranh giới (đã kiểm tra tự động, 0 vi phạm):

- `customer/` và `admin/` **không import code của nhau**; `shared/` không import ngược lên hai chế độ.
- Website khách có Modal, hộp xác nhận, ảnh đại diện và CSS (`c-*`) riêng, không dùng giao diện của Admin. Website khách không có liên kết nào dẫn tới Admin.
- Hai chế độ gặp nhau ở **dữ liệu**: đặt phòng của khách ghi vào `shared/services`, nên hiện ngay trong Admin.
- Phiên đăng nhập tách biệt: `hms_customer_session` (khách) và `hms_session` (Admin).

Ghi chú:

- `shared/services/*` là nơi duy nhất truy cập dữ liệu. Khi có Backend, thay thân hàm bằng lời gọi `axios` (`shared/services/api.js`) với cùng chữ ký.
- `admin/hooks/useCrud`, `useForm` và `shared/hooks/useFetch` tách logic dữ liệu / form khỏi giao diện.
- Phân quyền Admin: `admin/utils/permissions.js` (mặc định) + màn hình Cài đặt → Phân quyền (lưu ở localStorage).

## Tiến độ

- [x] Giai đoạn 1: Khởi tạo, thư viện, cấu trúc thư mục
- [x] Giai đoạn 2: Layout (Sidebar, Header, Main)
- [x] Giai đoạn 3: Dashboard
- [x] Giai đoạn 4: Các module quản lý
- [x] Giai đoạn 5: Điều hướng, tìm kiếm, lọc, form, tương tác
- [x] Giai đoạn 6: Rà soát responsive, sửa lỗi
- [x] Website khách hàng: layout + trang chủ; danh sách + chi tiết phòng; đặt phòng; đăng nhập / tài khoản / đặt phòng của tôi; liên hệ
