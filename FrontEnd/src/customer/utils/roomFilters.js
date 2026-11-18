// Bộ lọc & sắp xếp cho danh sách hạng phòng. Toàn bộ bộ lọc nằm trên URL để chia sẻ/quay lại được.
export const SORT_OPTIONS = [
  { value: 'price-asc', label: 'Giá thấp đến cao' },
  { value: 'price-desc', label: 'Giá cao đến thấp' },
  { value: 'capacity-desc', label: 'Sức chứa lớn nhất' },
];

export const PRICE_PRESETS = [
  { label: 'Dưới 1 triệu', minPrice: '', maxPrice: '1000000' },
  { label: '1 – 2 triệu', minPrice: '1000000', maxPrice: '2000000' },
  { label: 'Trên 2 triệu', minPrice: '2000000', maxPrice: '' },
];

export const defaultFilters = () => ({ types: [], minPrice: '', maxPrice: '', amenities: [], sort: 'price-asc', onlyAvailable: false });

const list = (raw) => (raw || '').split(',').map((s) => s.trim()).filter(Boolean);
const money = (raw) => {
  const n = Number.parseInt(raw, 10);
  return Number.isNaN(n) || n < 0 ? '' : String(n);
};

export function filtersFromQuery(params) {
  const sort = params.get('sort');
  return {
    types: list(params.get('types')),
    minPrice: money(params.get('minPrice')),
    maxPrice: money(params.get('maxPrice')),
    amenities: list(params.get('amenities')),
    sort: SORT_OPTIONS.some((o) => o.value === sort) ? sort : 'price-asc',
    onlyAvailable: params.get('available') === '1',
  };
}

// Ghi bộ lọc vào URLSearchParams (xóa khóa khi giá trị rỗng/mặc định) và trả về chính nó.
export function writeFilters(params, f) {
  const set = (key, value) => (value ? params.set(key, value) : params.delete(key));
  set('types', f.types.join(','));
  set('minPrice', f.minPrice);
  set('maxPrice', f.maxPrice);
  set('amenities', f.amenities.join(','));
  set('sort', f.sort !== 'price-asc' ? f.sort : '');
  set('available', f.onlyAvailable ? '1' : '');
  return params;
}

export function validateFilters(f) {
  if (f.minPrice !== '' && f.maxPrice !== '' && Number(f.minPrice) > Number(f.maxPrice)) {
    return { price: 'Giá thấp nhất không được lớn hơn giá cao nhất' };
  }
  return {};
}

export function activeFilterCount(f) {
  return f.types.length + f.amenities.length + (f.minPrice || f.maxPrice ? 1 : 0) + (f.onlyAvailable ? 1 : 0);
}

export function applyFilters(rooms, f) {
  const min = f.minPrice === '' ? 0 : Number(f.minPrice);
  const max = f.maxPrice === '' ? Infinity : Number(f.maxPrice);
  const out = rooms.filter(
    (r) =>
      (f.types.length === 0 || f.types.includes(r.id)) &&
      r.price >= min &&
      r.price <= max &&
      f.amenities.every((a) => r.amenities.includes(a)) &&
      (!f.onlyAvailable || r.available),
  );
  const sorters = {
    'price-asc': (a, b) => a.price - b.price,
    'price-desc': (a, b) => b.price - a.price,
    'capacity-desc': (a, b) => b.capacity - a.capacity || a.price - b.price,
  };
  return out.sort(sorters[f.sort] || sorters['price-asc']);
}
