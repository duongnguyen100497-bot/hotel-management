import { addDays, diffDays, todayStr } from '../../shared/utils/dateUtils.js';

// Quy tắc và tiện ích cho form tìm phòng (dùng ở trang chủ, danh sách phòng, đặt phòng).
export const SEARCH_LIMITS = { maxAdults: 10, maxChildren: 6, maxRooms: 5, maxNights: 30 };

const DATE_RE = /^\d{4}-\d{2}-\d{2}$/;
const isValidDate = (value) => DATE_RE.test(value || '') && !Number.isNaN(new Date(`${value}T00:00:00`).getTime());

export const countNights = (checkIn, checkOut) =>
  isValidDate(checkIn) && isValidDate(checkOut) ? Math.max(0, diffDays(checkIn, checkOut)) : 0;

export const defaultSearch = () => {
  const today = todayStr();
  return { checkIn: today, checkOut: addDays(today, 1), adults: 2, children: 0, rooms: 1 };
};

export const totalGuests = (v) => Number(v.adults || 0) + Number(v.children || 0);

// Trả về { field: 'thông báo lỗi' }, object rỗng nghĩa là hợp lệ.
export function validateSearch(v) {
  const errors = {};
  const today = todayStr();

  if (!isValidDate(v.checkIn)) errors.checkIn = 'Vui lòng chọn ngày nhận phòng';
  else if (v.checkIn < today) errors.checkIn = 'Ngày nhận phòng không được ở quá khứ';

  if (!isValidDate(v.checkOut)) errors.checkOut = 'Vui lòng chọn ngày trả phòng';
  else if (!errors.checkIn && v.checkOut <= v.checkIn) errors.checkOut = 'Ngày trả phòng phải sau ngày nhận phòng';
  else if (!errors.checkIn && countNights(v.checkIn, v.checkOut) > SEARCH_LIMITS.maxNights) {
    errors.checkOut = `Mỗi lần đặt tối đa ${SEARCH_LIMITS.maxNights} đêm`;
  }

  const adults = Number(v.adults);
  const rooms = Number(v.rooms);
  if (!Number.isInteger(adults) || adults < 1) errors.guests = 'Cần ít nhất 1 người lớn';
  else if (!Number.isInteger(rooms) || rooms < 1) errors.guests = 'Cần ít nhất 1 phòng';
  else if (rooms > adults) errors.guests = 'Số phòng không được nhiều hơn số người lớn';

  return errors;
}

export function searchToQuery(v) {
  const params = new URLSearchParams({
    checkIn: v.checkIn,
    checkOut: v.checkOut,
    adults: String(v.adults),
    children: String(v.children),
    rooms: String(v.rooms),
  });
  return params.toString();
}

const clampInt = (raw, min, max, fallback) => {
  const n = Number.parseInt(raw, 10);
  if (Number.isNaN(n)) return fallback;
  return Math.min(max, Math.max(min, n));
};

// Đọc lại tham số tìm kiếm từ URL, giá trị sai/thiếu được thay bằng mặc định.
export function queryToSearch(searchParams) {
  const base = defaultSearch();
  const checkIn = isValidDate(searchParams.get('checkIn')) ? searchParams.get('checkIn') : base.checkIn;
  const checkOut = isValidDate(searchParams.get('checkOut')) ? searchParams.get('checkOut') : addDays(checkIn, 1);
  const adults = clampInt(searchParams.get('adults'), 1, SEARCH_LIMITS.maxAdults, base.adults);
  return {
    checkIn,
    checkOut,
    adults,
    children: clampInt(searchParams.get('children'), 0, SEARCH_LIMITS.maxChildren, base.children),
    rooms: clampInt(searchParams.get('rooms'), 1, Math.min(adults, SEARCH_LIMITS.maxRooms), base.rooms),
  };
}
