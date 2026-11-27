-- Idempotent RBAC seed for canonical permissions and default system roles.
INSERT INTO permissions (code, name, description, is_system) VALUES
    ('user.read', 'Xem tài khoản người dùng', 'Xem danh sách và chi tiết tài khoản người dùng', true),
    ('user.manage', 'Quản lý tài khoản người dùng', 'Tạo, cập nhật trạng thái tài khoản người dùng', true),
    ('role.read', 'Xem vai trò', 'Xem danh sách vai trò và tập quyền', true),
    ('role.create', 'Tạo vai trò', 'Tạo vai trò tùy chỉnh mới', true),
    ('role.update', 'Cập nhật vai trò', 'Cập nhật tên, mô tả vai trò không phải hệ thống', true),
    ('role.delete', 'Xóa / Lưu trữ vai trò', 'Xóa hoặc lưu trữ vai trò tùy chỉnh chưa có người dùng', true),
    ('role.update_system', 'Cập nhật vai trò hệ thống', 'Sửa thông tin vai trò hệ thống mặc định', true),
    ('permission.read', 'Xem danh mục quyền', 'Xem toàn bộ danh mục quyền của hệ thống', true),
    ('permission.manage', 'Quản lý danh mục quyền', 'Thêm mới hoặc cập nhật mô tả quyền trong danh mục', true),
    ('role_permission.assign', 'Gán quyền cho vai trò', 'Thay đổi danh sách quyền thuộc một vai trò', true),
    ('user_role.assign', 'Gán vai trò cho nhân sự', 'Gán hoặc gỡ vai trò của nhân viên / quản trị viên', true),
    ('employee.read', 'Xem hồ sơ nhân viên', 'Xem danh sách và chi tiết nhân viên', true),
    ('employee.create', 'Tạo hồ sơ nhân viên', 'Tạo nhân viên mới kèm gửi email kích hoạt', true),
    ('employee.update', 'Cập nhật nhân viên', 'Cập nhật hồ sơ nhân viên và gửi lại email kích hoạt', true),
    ('employee.deactivate', 'Vô hiệu hóa nhân viên', 'Khóa tài khoản và ngừng kích hoạt nhân viên', true),
    ('customer.read', 'Xem hồ sơ khách hàng', 'Tra cứu danh sách và thông tin chi tiết khách hàng', true),
    ('customer.create', 'Tạo hồ sơ khách hàng', 'Tạo mới khách hàng tại quầy (walk-in)', true),
    ('customer.update', 'Cập nhật khách hàng', 'Cập nhật thông tin hồ sơ khách hàng', true),
    ('customer.archive', 'Lưu trữ khách hàng', 'Lưu trữ hồ sơ khách hàng không còn giao dịch active', true),
    ('hotel_settings.read', 'Xem cấu hình khách sạn', 'Xem thông tin cấu hình khách sạn và TTL giữ phòng', true),
    ('hotel_settings.update', 'Cập nhật cấu hình khách sạn', 'Thay đổi cấu hình khách sạn, múi giờ và TTL giữ phòng', true),
    ('vat.manage', 'Quản lý thuế suất VAT', 'Tạo, cập nhật và kích hoạt mức thuế VAT', true),
    ('property.read', 'Xem danh mục phòng & dịch vụ', 'Xem loại phòng, phòng vật lý và trạng thái vận hành', true),
    ('room_type.manage', 'Quản lý loại phòng', 'Tạo, sửa, lưu trữ loại phòng và giá cơ bản', true),
    ('room.create', 'Thêm phòng vật lý', 'Thêm phòng mới vào hệ thống', true),
    ('room.update', 'Cập nhật phòng vật lý', 'Cập nhật thông tin phòng vật lý', true),
    ('room.archive', 'Lưu trữ phòng vật lý', 'Lưu trữ phòng ngừng khai thác', true),
    ('room.state.update', 'Chuyển trạng thái vận hành phòng', 'Chuyển trạng thái phòng giữa available, cleaning, maintenance', true),
    ('service.read', 'Xem danh mục dịch vụ', 'Xem danh sách dịch vụ phụ trợ khách sạn', true),
    ('service.manage', 'Quản lý danh mục dịch vụ', 'Thêm, sửa, ngừng cung cấp dịch vụ khách sạn', true),
    ('media.manage', 'Quản lý hình ảnh / media', 'Tải lên, gắn và gỡ hình ảnh loại phòng / phòng', true),
    ('booking.read', 'Xem đặt phòng', 'Xem danh sách và chi tiết đơn đặt phòng lẻ / nhóm', true),
    ('booking.create', 'Tạo đặt phòng', 'Tạo đơn đặt phòng cho khách tại quầy', true),
    ('booking.confirm', 'Xác nhận đặt phòng thủ công', 'Xác nhận đơn đặt phòng walk-in chưa thanh toán trước', true),
    ('booking.cancel', 'Hủy đặt phòng', 'Hủy đơn đặt phòng lẻ hoặc hủy hàng loạt', true),
    ('wallet.read', 'Xem ví khách hàng', 'Tra cứu số dư và sổ cái ví khách hàng', true),
    ('wallet.deposit', 'Nạp ví thủ công', 'Ghi nhận nạp tiền thủ công vào ví khách hàng', true),
    ('payment.read', 'Xem giao dịch thanh toán', 'Xem payment intent, biên lai ngân hàng và phân bổ', true),
    ('payment.record', 'Ghi nhận thu tiền tại quầy', 'Thu tiền thanh toán hóa đơn tại quầy lễ tân', true),
    ('payment.reconcile', 'Đối soát biên lai ngân hàng', 'Đối soát thủ công các biên lai SePay unmatched', true),
    ('payment.refund', 'Hoàn tiền về ví', 'Thực hiện hoàn tiền về ví khách hàng', true),
    ('stay.check_in', 'Làm thủ tục nhận phòng (Check-in)', 'Thực hiện check-in cho đơn đặt phòng đã xác nhận', true),
    ('stay.check_out', 'Làm thủ tục trả phòng (Check-out)', 'Thực hiện check-out và phát hành hóa đơn', true),
    ('stay.service.manage', 'Ghi nhận dịch vụ trong lưu trú', 'Thêm dịch vụ phát sinh vào kỳ lưu trú đang mở', true),
    ('invoice.read', 'Xem hóa đơn', 'Xem chi tiết hóa đơn và lịch sử thanh toán', true),
    ('report.read', 'Xem báo cáo thống kê', 'Xem báo cáo công suất 19:00, doanh thu ròng, khách quay lại', true),
    ('report.export', 'Xuất dữ liệu báo cáo', 'Xuất báo cáo doanh thu ra định dạng CSV', true),
    ('audit.read', 'Tra cứu nhật ký kiểm toán', 'Xem lịch sử audit_events toàn hệ thống', true)
ON CONFLICT (code) DO NOTHING;

INSERT INTO roles (code, name, description, is_system, is_root) VALUES
    ('system-administrator', 'Quản trị hệ thống (Root Admin)', 'Toàn quyền hệ thống với cơ chế root bypass bất biến', true, true),
    ('hotel-manager', 'Quản lý khách sạn', 'Quản lý nhân sự, cấu hình khách sạn, phòng, dịch vụ, đặt phòng, báo cáo', true, false),
    ('front-desk', 'Lễ tân', 'Đặt phòng tại quầy, quản lý khách hàng, check-in/out, ghi nhận dịch vụ và thu tiền hóa đơn', true, false),
    ('accounting', 'Kế toán', 'Quản lý VAT, ví khách hàng, đối soát biên lai ngân hàng, hoàn tiền, hóa đơn và báo cáo', true, false)
ON CONFLICT (code) DO NOTHING;

-- Assign all permissions to system-administrator as baseline (in addition to code-level root bypass).
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.code = 'system-administrator'
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- Assign baseline permissions to hotel-manager.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.code IN (
    'user.read', 'role.read', 'permission.read',
    'employee.read', 'employee.create', 'employee.update', 'employee.deactivate',
    'customer.read', 'customer.create', 'customer.update', 'customer.archive',
    'hotel_settings.read', 'hotel_settings.update',
    'property.read', 'room_type.manage', 'room.create', 'room.update', 'room.archive', 'room.state.update',
    'service.read', 'service.manage', 'media.manage',
    'booking.read', 'booking.create', 'booking.confirm', 'booking.cancel',
    'wallet.read', 'payment.read', 'payment.refund',
    'stay.check_in', 'stay.check_out', 'stay.service.manage', 'invoice.read',
    'report.read', 'report.export', 'audit.read'
)
WHERE r.code = 'hotel-manager'
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- Assign baseline permissions to front-desk.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.code IN (
    'customer.read', 'customer.create', 'customer.update',
    'hotel_settings.read', 'property.read', 'room.state.update', 'service.read',
    'booking.read', 'booking.create', 'booking.confirm', 'booking.cancel',
    'wallet.read', 'payment.read', 'payment.record',
    'stay.check_in', 'stay.check_out', 'stay.service.manage', 'invoice.read'
)
WHERE r.code = 'front-desk'
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- Assign baseline permissions to accounting.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.code IN (
    'customer.read', 'hotel_settings.read', 'vat.manage', 'property.read', 'service.read',
    'booking.read',
    'wallet.read', 'wallet.deposit',
    'payment.read', 'payment.record', 'payment.reconcile', 'payment.refund',
    'invoice.read', 'report.read', 'report.export', 'audit.read'
)
WHERE r.code = 'accounting'
ON CONFLICT (role_id, permission_id) DO NOTHING;
