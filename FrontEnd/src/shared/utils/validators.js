// Các hàm validate dùng cho form. Mỗi hàm trả về chuỗi lỗi, hoặc '' nếu hợp lệ.
// Dùng kèm firstError(...) để lấy lỗi đầu tiên của một trường.

const isEmpty = (v) => v === undefined || v === null || String(v).trim() === '';

export const required = (v, label = 'Trường này') => (isEmpty(v) ? `${label} không được để trống` : '');

export const minLength = (v, n, label = 'Giá trị') =>
  !isEmpty(v) && String(v).trim().length < n ? `${label} tối thiểu ${n} ký tự` : '';

// Không bắt buộc nhập: chỉ kiểm tra định dạng khi có giá trị.
export const isEmail = (v) =>
  isEmpty(v) || /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(String(v).trim()) ? '' : 'Email không hợp lệ';

export const isPhone = (v) =>
  isEmpty(v) || /^(0|\+84)\d{9,10}$/.test(String(v).replace(/[\s.-]/g, ''))
    ? ''
    : 'Số điện thoại không hợp lệ (VD: 0912345678)';

export const positiveNumber = (v, label = 'Giá trị') =>
  isEmpty(v) || (Number.isFinite(Number(v)) && Number(v) > 0) ? '' : `${label} phải là số lớn hơn 0`;

export const nonNegativeNumber = (v, label = 'Giá trị') =>
  isEmpty(v) || (Number.isFinite(Number(v)) && Number(v) >= 0) ? '' : `${label} không được âm`;

export const firstError = (...errors) => errors.find(Boolean) || '';
