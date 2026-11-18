import { delay } from './api.js';

const numberFrom = (id) => {
  const m = String(id).match(/(\d+)$/);
  return m ? parseInt(m[1], 10) : 0;
};

/**
 * Tạo một "service" CRUD giả lập (lưu trong bộ nhớ, có độ trễ như API thật).
 * Khi có Backend, chỉ cần thay thân các hàm bằng lời gọi axios với cùng chữ ký.
 *
 * options:
 *  - prefix/pad: sinh mã (vd: BK0001)
 *  - idFrom(payload): tự đặt id (vd: mã phòng)
 *  - unique: [{ field, message }] kiểm tra trùng
 *  - validate(item, store, selfId): ném Error nếu dữ liệu không hợp lệ
 *  - sortBy: hàm so sánh khi getAll
 */
export function createMockService({
  prefix = '',
  pad = 3,
  initial = [],
  unique = [],
  validate,
  idFrom,
  sortBy,
  latency = 250,
}) {
  let store = initial.map((i) => ({ ...i }));
  let counter = store.reduce((max, i) => Math.max(max, numberFrom(i.id)), 0);

  const clone = (x) => ({ ...x });
  const wait = () => delay(latency);
  const peekId = () => `${prefix}${String(counter + 1).padStart(pad, '0')}`;
  const nextId = () => {
    counter += 1;
    return `${prefix}${String(counter).padStart(pad, '0')}`;
  };
  const sorted = () => (sortBy ? [...store].sort(sortBy) : store);

  const checkUnique = (item, selfId) => {
    unique.forEach(({ field, message }) => {
      const value = String(item[field] ?? '').trim().toLowerCase();
      if (!value) return;
      const dup = store.some(
        (o) => o.id !== selfId && String(o[field] ?? '').trim().toLowerCase() === value,
      );
      if (dup) throw new Error(message);
    });
  };

  return {
    async getAll() {
      await wait();
      return sorted().map(clone);
    },

    async getById(id) {
      await wait();
      const item = store.find((x) => x.id === id);
      if (!item) throw new Error('Không tìm thấy dữ liệu');
      return clone(item);
    },

    async create(payload) {
      await wait();
      // Chỉ tăng bộ đếm sau khi dữ liệu hợp lệ để mã không bị nhảy cách khi tạo thất bại.
      const item = {
        ...payload,
        id: idFrom ? idFrom(payload) : peekId(),
        createdAt: payload.createdAt || new Date().toISOString(),
      };
      checkUnique(item, null);
      if (validate) validate(item, store, null);
      if (!idFrom) counter += 1;
      store = [item, ...store];
      return clone(item);
    },

    async update(id, payload) {
      await wait();
      const current = store.find((x) => x.id === id);
      if (!current) throw new Error('Không tìm thấy dữ liệu');
      const next = { ...current, ...payload, id };
      checkUnique(next, id);
      if (validate) validate(next, store, id);
      store = store.map((x) => (x.id === id ? next : x));
      return clone(next);
    },

    async remove(id) {
      await wait();
      if (!store.some((x) => x.id === id)) throw new Error('Không tìm thấy dữ liệu');
      store = store.filter((x) => x.id !== id);
    },

    // Thêm đồng bộ (không độ trễ) – dùng cho ghi log hoạt động.
    insertSync(payload) {
      const item = {
        ...payload,
        id: nextId(),
        createdAt: payload.createdAt || new Date().toISOString(),
      };
      store = [item, ...store];
      return clone(item);
    },

    // Truy cập nội bộ cho các service khác (tính toán chéo, không độ trễ).
    internal: {
      get: () => store,
      set: (next) => {
        store = next;
        // Bộ đếm mã không bao giờ lùi: tránh trùng mã khi nạp lại dữ liệu đã lưu (xem onlineDataStore).
        counter = next.reduce((max, i) => Math.max(max, numberFrom(i.id)), counter);
      },
    },
  };
}
