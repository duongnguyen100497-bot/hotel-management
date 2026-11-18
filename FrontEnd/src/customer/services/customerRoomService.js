import roomService from '../../shared/services/roomService.js';
import bookingService from '../../shared/services/bookingService.js';
import { ROOM_CATALOG } from '../data/roomCatalog.js';
import { ACTIVE_BOOKING_STATUSES } from '../../shared/utils/constants.js';
import { defaultSearch, totalGuests } from '../utils/bookingSearch.js';

// Lớp dữ liệu phòng cho website khách hàng – làm việc theo LOẠI PHÒNG.
// Khách chọn một hạng phòng; hệ thống tự gán phòng cụ thể còn trống (P101, P203...) khi đặt.
// Nguồn sự thật là roomService / bookingService của Admin: tên, loại, giá, sức chứa, trạng thái luôn lấy từ đó.
// Khi có Backend thật, chỉ cần thay phần gọi service bằng API tương ứng.

const FALLBACK_SPEC = {
  description: '', promoPercent: 0, area: null, bedType: '—', bedCount: 1, view: '',
  amenities: ['wifi', 'ac'], images: [], highlight: '',
};

export const typeSlug = (type) => String(type).toLowerCase();

// Phòng còn trống trong khoảng [checkIn, checkOut) nếu không bảo trì và không trùng booking đang hoạt động.
export function isRoomAvailable(room, bookings, checkIn, checkOut) {
  if (room.status === 'Maintenance') return false;
  return !bookings.some(
    (b) =>
      b.roomCode === room.code &&
      ACTIVE_BOOKING_STATUSES.includes(b.status) &&
      b.checkIn < checkOut &&
      checkIn < b.checkOut,
  );
}

// Gộp danh sách phòng của Admin thành các hạng phòng (giữ thứ tự giá tăng dần).
export function groupRoomTypes(rooms) {
  const byType = new Map();
  rooms.forEach((r) => {
    if (!byType.has(r.type)) byType.set(r.type, []);
    byType.get(r.type).push(r);
  });

  return [...byType.entries()]
    .map(([type, list]) => {
      const spec = ROOM_CATALOG[type] || FALLBACK_SPEC;
      const bookable = list.filter((r) => r.status !== 'Maintenance');
      const base = bookable.length ? bookable : list;
      return {
        id: typeSlug(type),
        type,
        name: `Phòng ${type}`,
        price: Math.min(...base.map((r) => r.price)),
        capacity: Math.max(...base.map((r) => r.capacity)),
        totalRooms: bookable.length,
        floors: [...new Set(list.map((r) => r.floor))].sort((a, b) => a - b),
        ...spec,
        // Mô tả riêng của website; nếu thiếu thì dùng mô tả từ Admin.
        description: spec.description || list[0]?.description || '',
      };
    })
    .sort((a, b) => a.price - b.price);
}

// Thêm thông tin còn phòng cho khoảng ngày + số khách tìm kiếm.
function withAvailability(types, rooms, bookings, search) {
  const { checkIn, checkOut, rooms: wanted } = search;
  const guests = totalGuests(search);
  return types.map((t) => {
    const free = rooms.filter((r) => r.type === t.type && isRoomAvailable(r, bookings, checkIn, checkOut));
    const fitsGuests = guests <= t.capacity * wanted;
    return {
      ...t,
      availableCount: free.length,
      fitsGuests,
      // Đủ phòng trống và đủ chỗ cho tất cả khách.
      available: free.length >= wanted && fitsGuests,
    };
  });
}

async function loadAll() {
  const [rooms, bookings] = await Promise.all([roomService.getAll(), bookingService.getAll()]);
  return { rooms, bookings };
}

const customerRoomService = {
  // Danh sách hạng phòng + tình trạng còn phòng theo tìm kiếm (trang Danh sách phòng).
  async searchRoomTypes(search = defaultSearch()) {
    const { rooms, bookings } = await loadAll();
    return withAvailability(groupRoomTypes(rooms), rooms, bookings, search);
  },

  // Một hạng phòng theo id (slug, vd "deluxe"). Cũng chấp nhận id/mã của một phòng cụ thể (vd "P203").
  async getRoomType(id, search = defaultSearch()) {
    const { rooms, bookings } = await loadAll();
    const wanted = String(id || '').toLowerCase();
    const bySlug = groupRoomTypes(rooms).find((t) => t.id === wanted);
    const byRoom = rooms.find((r) => r.id.toLowerCase() === wanted || r.code.toLowerCase() === wanted);
    const type = bySlug || (byRoom ? groupRoomTypes(rooms).find((t) => t.type === byRoom.type) : null);
    if (!type) throw new Error('Không tìm thấy hạng phòng này');

    const all = withAvailability(groupRoomTypes(rooms), rooms, bookings, search);
    return { room: all.find((t) => t.id === type.id), others: all.filter((t) => t.id !== type.id) };
  },

  // Dữ liệu cho trang chủ.
  async getOverview() {
    const { rooms, bookings } = await loadAll();
    const search = defaultSearch();
    const types = withAvailability(groupRoomTypes(rooms), rooms, bookings, search);
    const bookable = rooms.filter((r) => r.status !== 'Maintenance');
    return {
      stats: {
        totalRooms: rooms.length,
        typeCount: types.length,
        minPrice: bookable.length ? Math.min(...bookable.map((r) => r.price)) : 0,
      },
      types,
    };
  },

  // Chọn `count` phòng cụ thể còn trống cho khoảng ngày (ưu tiên mã nhỏ), bỏ qua các mã trong `exclude`.
  // Trả về [] nếu không đủ.
  async pickRooms(type, checkIn, checkOut, count = 1, exclude = []) {
    const { rooms, bookings } = await loadAll();
    const free = rooms
      .filter((r) => r.type === type && !exclude.includes(r.code) && isRoomAvailable(r, bookings, checkIn, checkOut))
      .sort((a, b) => a.code.localeCompare(b.code));
    return free.length >= count ? free.slice(0, count) : [];
  },
};

export default customerRoomService;
