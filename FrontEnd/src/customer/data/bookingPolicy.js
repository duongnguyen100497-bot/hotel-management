// Chính sách đặt phòng hiển thị cho khách và dùng để tính giá.
// LƯU Ý: các con số dưới đây là giá trị MẶC ĐỊNH để minh họa (chưa có trong Cài đặt của Admin).
// Khi khách sạn xác nhận chính sách thật, chỉ cần sửa tại file này.
export const BOOKING_POLICY = {
  vatRate: 0.1, // Thuế VAT 10% trên tiền phòng
  serviceFeeRate: 0.05, // Phí dịch vụ 5% trên tiền phòng
  freeCancelHours: 48, // Hủy miễn phí đến trước giờ nhận phòng 48 giờ
  maxRoomsPerBooking: 5,
};

export const CHECK_RULES = [
  'Nhận phòng từ {checkIn}, trả phòng trước {checkOut}.',
  'Cần xuất trình CCCD/hộ chiếu khi nhận phòng.',
  'Nhận phòng sớm hoặc trả phòng muộn tùy theo tình trạng phòng, vui lòng liên hệ lễ tân.',
  'Khách sạn hỗ trợ gửi hành lý miễn phí.',
];

export const CANCEL_RULES = [
  `Hủy miễn phí khi thực hiện trước giờ nhận phòng ${BOOKING_POLICY.freeCancelHours} giờ.`,
  `Hủy trong vòng ${BOOKING_POLICY.freeCancelHours} giờ trước giờ nhận phòng cần liên hệ khách sạn để được hỗ trợ.`,
  'Đặt phòng đã nhận phòng không thể hủy trực tuyến.',
];
