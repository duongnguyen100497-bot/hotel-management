// Dữ liệu mẫu. Ngày tháng tính tương đối theo hôm nay để Dashboard luôn có khách check-in/out sắp tới.
import { addDays, diffDays, todayStr } from '../utils/dateUtils.js';

const today = todayStr();
const d = (n) => addDays(today, n);
const isoAt = (offset, h = 12, m = 0) => {
  const x = new Date(`${d(offset)}T00:00:00`);
  x.setHours(h, m, 0, 0);
  return x.toISOString();
};
const isoAgo = (minutes) => new Date(Date.now() - minutes * 60000).toISOString();

/* ---------------- Tài khoản đăng nhập ---------------- */
export const seedUsers = [
  { id: 'U1', username: 'admin', password: 'admin123', name: 'Nguyễn Quản Trị', email: 'admin@grandnavy.vn', role: 'admin' },
  { id: 'U2', username: 'manager', password: 'manager123', name: 'Trần Minh Quân', email: 'quan.tran@grandnavy.vn', role: 'manager' },
  { id: 'U3', username: 'letan', password: 'letan123', name: 'Trần Thu Hương', email: 'huong.tran@grandnavy.vn', role: 'receptionist' },
  { id: 'U4', username: 'ketoan', password: 'ketoan123', name: 'Hoàng Minh Tuấn', email: 'tuan.hoang@grandnavy.vn', role: 'accountant' },
];

export const defaultHotelInfo = {
  name: 'Grand Navy Hotel',
  logo: null,
  address: '123 Đường Trần Phú, Quận 1, TP. Hồ Chí Minh',
  phone: '028 3822 1234',
  email: 'contact@grandnavy.vn',
  website: 'www.grandnavy.vn',
  stars: 5,
  checkInTime: '14:00',
  checkOutTime: '12:00',
};

/* ---------------- Phòng ---------------- */
const PRICE = { Standard: 800000, Superior: 1100000, Deluxe: 1500000, Family: 1900000, Suite: 2800000 };
const CAPACITY = { Standard: 2, Superior: 2, Deluxe: 3, Family: 4, Suite: 4 };
const DESCRIPTION = {
  Standard: 'Phòng tiêu chuẩn 25m², giường đôi, cửa sổ hướng phố, đầy đủ tiện nghi cơ bản.',
  Superior: 'Phòng cao cấp 30m², giường King, bàn làm việc và khu vực tiếp khách nhỏ.',
  Deluxe: 'Phòng Deluxe 38m², ban công riêng, tầm nhìn thành phố, bồn tắm đứng.',
  Family: 'Phòng gia đình 45m², hai giường đôi, phù hợp nhóm 3-4 người.',
  Suite: 'Suite 65m², phòng khách riêng, bồn tắm nằm, tầm nhìn toàn cảnh.',
};
const LAYOUT = {
  1: ['Standard', 'Standard', 'Standard', 'Standard', 'Superior', 'Superior'],
  2: ['Superior', 'Superior', 'Superior', 'Deluxe', 'Deluxe', 'Deluxe'],
  3: ['Deluxe', 'Deluxe', 'Family', 'Family', 'Family', 'Deluxe'],
  4: ['Suite', 'Suite', 'Suite', 'Deluxe', 'Suite', 'Family'],
};
const STATUS = {
  P102: 'Occupied', P105: 'Occupied', P203: 'Occupied', P204: 'Occupied', P302: 'Occupied', P305: 'Occupied', P401: 'Occupied',
  P103: 'Reserved', P206: 'Reserved',
  P104: 'Cleaning', P304: 'Cleaning',
  P106: 'Maintenance', P405: 'Maintenance',
};

export const seedRooms = Object.entries(LAYOUT).flatMap(([floor, types]) =>
  types.map((type, i) => {
    const code = `P${floor}0${i + 1}`;
    return {
      id: code,
      code,
      name: `Phòng ${code.slice(1)}`,
      type,
      price: PRICE[type],
      floor: Number(floor),
      capacity: CAPACITY[type],
      status: STATUS[code] || 'Available',
      description: DESCRIPTION[type],
      createdAt: isoAt(-90),
    };
  }),
);

/* ---------------- Khách hàng ---------------- */
const cust = (n, fullName, phone, email, idNumber, nationality, address) => ({
  id: `KH${String(n).padStart(3, '0')}`,
  fullName,
  phone,
  email,
  idNumber,
  nationality,
  address,
  createdAt: isoAt(-60 + n),
});

export const seedCustomers = [
  cust(1, 'Nguyễn Văn An', '0901234567', 'an.nguyen@email.com', '079123456789', 'Việt Nam', 'Quận 3, TP. Hồ Chí Minh'),
  cust(2, 'Trần Thị Bích', '0912345678', 'bich.tran@email.com', '001234567890', 'Việt Nam', 'Cầu Giấy, Hà Nội'),
  cust(3, 'Lê Minh Châu', '0923456789', 'chau.le@email.com', '048123456789', 'Việt Nam', 'Hải Châu, Đà Nẵng'),
  cust(4, 'Phạm Quốc Dũng', '0934567890', 'dung.pham@email.com', '092123456789', 'Việt Nam', 'Ninh Kiều, Cần Thơ'),
  cust(5, 'John Smith', '0987654321', 'john.smith@mail.com', 'US12345678', 'Hoa Kỳ', 'New York, USA'),
  cust(6, 'Yuki Tanaka', '0976543210', 'yuki.tanaka@mail.jp', 'JP87654321', 'Nhật Bản', 'Osaka, Japan'),
  cust(7, 'Hoàng Thị Lan', '0945678901', 'lan.hoang@email.com', '038123456789', 'Việt Nam', 'Vinh, Nghệ An'),
  cust(8, 'Kim Min-jun', '0965432109', 'minjun.kim@mail.kr', 'KR11223344', 'Hàn Quốc', 'Seoul, Korea'),
  cust(9, 'Vũ Đức Hải', '0956789012', 'hai.vu@email.com', '031123456789', 'Việt Nam', 'Lê Chân, Hải Phòng'),
  cust(10, 'Emma Wilson', '0954321098', 'emma.wilson@mail.uk', 'GB55667788', 'Anh', 'London, UK'),
  cust(11, 'Đặng Thu Hà', '0967890123', 'ha.dang@email.com', '079987654321', 'Việt Nam', 'Quận 7, TP. Hồ Chí Minh'),
  cust(12, 'Li Wei', '0943210987', 'li.wei@mail.cn', 'CN99887766', 'Trung Quốc', 'Shanghai, China'),
];

/* ---------------- Đặt phòng ---------------- */
let bookingCounter = 0;
const mkBooking = (custId, code, inOff, outOff, status, guests = 2, extra = {}) => {
  const room = seedRooms.find((r) => r.code === code);
  const c = seedCustomers.find((x) => x.id === custId);
  const nights = diffDays(d(inOff), d(outOff));
  bookingCounter += 1;
  return {
    id: `BK${String(bookingCounter).padStart(4, '0')}`,
    customerId: custId,
    customerName: c.fullName,
    phone: c.phone,
    roomType: room.type,
    roomCode: code,
    checkIn: d(inOff),
    checkOut: d(outOff),
    guests,
    total: nights * room.price,
    status,
    note: '',
    createdAt: isoAt(Math.min(inOff, 0) - 3, 10, 30),
    ...extra,
  };
};
const stay = (inOff) => ({ checkInTime: isoAt(inOff, 14, 10) });
const stayOut = (inOff, outOff) => ({ checkInTime: isoAt(inOff, 14, 10), checkOutTime: isoAt(outOff, 11, 30) });

export const seedBookings = [
  // Đang lưu trú
  mkBooking('KH001', 'P102', -2, 1, 'Checked-in', 2, stay(-2)),
  mkBooking('KH002', 'P105', -1, 2, 'Checked-in', 2, stay(-1)),
  mkBooking('KH003', 'P203', -3, 0, 'Checked-in', 3, stay(-3)),
  mkBooking('KH004', 'P204', -1, 1, 'Checked-in', 2, stay(-1)),
  mkBooking('KH005', 'P302', -2, 0, 'Checked-in', 2, stay(-2)),
  mkBooking('KH006', 'P305', -4, 1, 'Checked-in', 2, stay(-4)),
  mkBooking('KH007', 'P401', -1, 3, 'Checked-in', 3, stay(-1)),
  // Sắp nhận phòng
  mkBooking('KH008', 'P103', 0, 2, 'Confirmed', 2, { createdAt: isoAgo(35) }),
  mkBooking('KH009', 'P206', 0, 3, 'Confirmed', 2, { createdAt: isoAgo(90) }),
  mkBooking('KH010', 'P301', 1, 3, 'Confirmed', 3, { createdAt: isoAgo(150) }),
  mkBooking('KH011', 'P202', 2, 4, 'Confirmed', 2, { createdAt: isoAgo(240) }),
  mkBooking('KH012', 'P402', 5, 7, 'Confirmed', 2, { createdAt: isoAgo(20) }),
  mkBooking('KH001', 'P101', 6, 8, 'Pending', 2, { createdAt: isoAgo(10) }),
  // Đã trả phòng
  mkBooking('KH001', 'P101', -10, -8, 'Checked-out', 2, stayOut(-10, -8)),
  mkBooking('KH004', 'P102', -7, -5, 'Checked-out', 2, stayOut(-7, -5)),
  mkBooking('KH002', 'P201', -2, 0, 'Checked-out', 2, stayOut(-2, 0)),
  mkBooking('KH003', 'P303', -3, -1, 'Checked-out', 3, stayOut(-3, -1)),
  mkBooking('KH006', 'P304', -2, 0, 'Checked-out', 2, stayOut(-2, 0)),
  mkBooking('KH009', 'P104', -1, 0, 'Checked-out', 2, stayOut(-1, 0)),
  // Đã hủy
  mkBooking('KH005', 'P205', 3, 5, 'Cancelled', 2),
];

/* ---------------- Dịch vụ ---------------- */
const svc = (n, name, price, type, description, active = true) => ({
  id: `DV${String(n).padStart(3, '0')}`,
  name,
  price,
  type,
  description,
  active,
  createdAt: isoAt(-80),
});

export const seedServices = [
  svc(1, 'Giặt ủi thường', 50000, 'Giặt ủi', 'Giặt ủi tính theo kg, trả đồ trong 24 giờ.'),
  svc(2, 'Giặt ủi nhanh', 90000, 'Giặt ủi', 'Giặt ủi lấy trong 4 giờ.'),
  svc(3, 'Buffet sáng', 180000, 'Nhà hàng', 'Buffet sáng tại nhà hàng tầng trệt, 6:00 - 10:00.'),
  svc(4, 'Set ăn tối', 350000, 'Nhà hàng', 'Set menu 3 món phục vụ tại phòng hoặc nhà hàng.'),
  svc(5, 'Massage 60 phút', 450000, 'Spa', 'Massage thư giãn toàn thân 60 phút.'),
  svc(6, 'Combo Minibar', 250000, 'Minibar', 'Bộ đồ uống và snack trong minibar.'),
  svc(7, 'Đưa đón sân bay', 300000, 'Đưa đón sân bay', 'Xe 4 chỗ đưa hoặc đón sân bay (một chiều).'),
  svc(8, 'Thuê xe máy', 150000, 'Khác', 'Thuê xe máy theo ngày (tạm ngưng).', false),
];

/* ---------------- Hóa đơn ---------------- */
const checkedOut = seedBookings.filter((b) => b.status === 'Checked-out');
const invoiceCfg = [
  { items: [['Buffet sáng', 2, 180000]], surcharge: 0, method: 'Tiền mặt', status: 'Đã thanh toán', day: -8 },
  { items: [['Giặt ủi thường', 3, 50000], ['Combo Minibar', 1, 250000]], surcharge: 0, method: 'Chuyển khoản', status: 'Đã thanh toán', day: -5 },
  { items: [['Massage 60 phút', 2, 450000]], surcharge: 100000, method: 'Thẻ tín dụng', status: 'Đã thanh toán', day: 0 },
  { items: [['Đưa đón sân bay', 1, 300000], ['Set ăn tối', 2, 350000]], surcharge: 0, method: 'Chuyển khoản', status: 'Đã thanh toán', day: -1 },
  { items: [['Combo Minibar', 2, 250000]], surcharge: 200000, method: 'Tiền mặt', status: 'Chưa thanh toán', day: 0 },
  { items: [['Giặt ủi nhanh', 2, 90000]], surcharge: 0, method: 'Tiền mặt', status: 'Chưa thanh toán', day: 0 },
];

export const seedInvoices = checkedOut.map((b, i) => {
  const cfg = invoiceCfg[i];
  const serviceItems = cfg.items.map(([name, qty, price]) => ({ name, qty, price }));
  const serviceCharge = serviceItems.reduce((s, it) => s + it.qty * it.price, 0);
  return {
    id: `HD${String(i + 1).padStart(4, '0')}`,
    bookingId: b.id,
    customerName: b.customerName,
    roomCode: b.roomCode,
    checkIn: b.checkIn,
    checkOut: b.checkOut,
    serviceItems,
    roomCharge: b.total,
    serviceCharge,
    surcharge: cfg.surcharge,
    paymentMethod: cfg.method,
    paymentStatus: cfg.status,
    createdAt: isoAt(cfg.day, 12, 0),
  };
});

/* ---------------- Nhân viên ---------------- */
const emp = (n, fullName, phone, email, position, username, status = 'Đang làm việc') => ({
  id: `NV${String(n).padStart(3, '0')}`,
  fullName,
  phone,
  email,
  position,
  username,
  status,
  createdAt: isoAt(-200 + n),
});

export const seedEmployees = [
  emp(1, 'Nguyễn Quản Trị', '0900000001', 'admin@grandnavy.vn', 'Quản lý', 'admin'),
  emp(2, 'Trần Thu Hương', '0900000002', 'huong.tran@grandnavy.vn', 'Lễ tân', 'letan01'),
  emp(3, 'Lê Văn Phúc', '0900000003', 'phuc.le@grandnavy.vn', 'Lễ tân', 'letan02'),
  emp(4, 'Phạm Thị Mai', '0900000004', 'mai.pham@grandnavy.vn', 'Buồng phòng', 'buongphong01'),
  emp(5, 'Hoàng Minh Tuấn', '0900000005', 'tuan.hoang@grandnavy.vn', 'Kế toán', 'ketoan01'),
  emp(6, 'Đỗ Văn Hùng', '0900000006', 'hung.do@grandnavy.vn', 'Bảo vệ', 'baove01'),
  emp(7, 'Ngô Thanh Bình', '0900000007', 'binh.ngo@grandnavy.vn', 'Bếp', 'bep01'),
  emp(8, 'Vũ Thị Lan', '0900000008', 'lan.vu@grandnavy.vn', 'Buồng phòng', 'buongphong02', 'Nghỉ phép'),
];

/* ---------------- Hoạt động gần đây ---------------- */
export const seedActivities = [
  { id: 'AC001', type: 'checkin', message: 'Check-in phòng P401 – Hoàng Thị Lan', time: isoAgo(25) },
  { id: 'AC002', type: 'booking', message: 'Đặt phòng mới BK0012 – Li Wei (P402)', time: isoAgo(20) },
  { id: 'AC003', type: 'payment', message: 'Thanh toán hóa đơn HD0003 – Trần Thị Bích', time: isoAgo(70) },
  { id: 'AC004', type: 'checkout', message: 'Check-out phòng P304 – Yuki Tanaka', time: isoAgo(130) },
  { id: 'AC005', type: 'room', message: 'Phòng P106 chuyển sang trạng thái bảo trì', time: isoAgo(260) },
  { id: 'AC006', type: 'booking', message: 'Đặt phòng mới BK0010 – Emma Wilson (P301)', time: isoAgo(150) },
  { id: 'AC007', type: 'checkin', message: 'Check-in phòng P105 – Trần Thị Bích', time: isoAgo(600) },
  { id: 'AC008', type: 'booking', message: 'Hủy đặt phòng BK0020 – John Smith', time: isoAgo(900) },
].map((a) => ({ ...a, createdAt: a.time }));
