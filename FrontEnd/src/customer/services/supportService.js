import { delay } from '../../shared/services/api.js';
import { firstError, isEmail, isPhone, minLength, required } from '../../shared/utils/validators.js';

// Yêu cầu hỗ trợ từ trang Liên hệ.
// Bản chưa có Backend: yêu cầu chỉ được lưu trong trình duyệt này (localStorage), CHƯA chuyển tới nhân viên.
// Khi có Backend, thay phần lưu bằng lời gọi API tới hệ thống ticket / email của khách sạn.
const KEY = 'hms_support_requests';

export const SUPPORT_TOPICS = ['Đặt phòng và thanh toán', 'Thay đổi hoặc hủy đặt phòng', 'Dịch vụ và tiện ích', 'Hóa đơn', 'Góp ý khác'];
export const MESSAGE_MIN = 10;
export const MESSAGE_MAX = 1000;

export function validateSupport(v) {
  const errors = {};
  const set = (k, msg) => msg && (errors[k] = msg);
  set('fullName', firstError(required(v.fullName, 'Họ và tên'), minLength(v.fullName, 2, 'Họ và tên')));
  set('email', firstError(required(v.email, 'Email'), isEmail(v.email)));
  set('phone', isPhone(v.phone)); // không bắt buộc
  set('topic', required(v.topic, 'Chủ đề'));
  const msg = (v.message || '').trim();
  if (!msg) errors.message = 'Nội dung không được để trống';
  else if (msg.length < MESSAGE_MIN) errors.message = `Nội dung tối thiểu ${MESSAGE_MIN} ký tự`;
  else if (msg.length > MESSAGE_MAX) errors.message = `Nội dung tối đa ${MESSAGE_MAX} ký tự`;
  return errors;
}

const supportService = {
  async submit(values) {
    await delay(500);
    const errors = validateSupport(values);
    if (Object.keys(errors).length) throw Object.assign(new Error('Thông tin chưa hợp lệ'), { fields: errors });
    let list = [];
    try {
      list = JSON.parse(window.localStorage.getItem(KEY) || '[]');
    } catch {
      list = [];
    }
    const request = {
      id: `YC${String(list.length + 1).padStart(4, '0')}`,
      fullName: values.fullName.trim(),
      email: values.email.trim().toLowerCase(),
      phone: (values.phone || '').trim(),
      topic: values.topic,
      message: values.message.trim(),
      createdAt: new Date().toISOString(),
    };
    try {
      window.localStorage.setItem(KEY, JSON.stringify([...list, request]));
    } catch {
      /* localStorage bị chặn: vẫn trả về mã để người dùng có phản hồi */
    }
    return request;
  },
};

export default supportService;
