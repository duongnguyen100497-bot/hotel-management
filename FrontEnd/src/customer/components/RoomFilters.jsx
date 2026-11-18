import { AlertCircle } from 'lucide-react';
import { ROOM_AMENITY_LIST } from '../data/amenities.js';
import { PRICE_PRESETS, SORT_OPTIONS, activeFilterCount, validateFilters } from '../utils/roomFilters.js';

const toggle = (list, value) => (list.includes(value) ? list.filter((v) => v !== value) : [...list, value]);

function Group({ title, children }) {
  return (
    <fieldset className="border-t border-slate-200 py-5 first:border-t-0 first:pt-0">
      <legend className="mb-3 text-sm font-bold text-brand">{title}</legend>
      {children}
    </fieldset>
  );
}

function Check({ checked, onChange, children }) {
  return (
    <label className="flex cursor-pointer items-center gap-2.5 py-1.5 text-sm text-slate-700">
      <input type="checkbox" checked={checked} onChange={onChange} className="h-4 w-4 rounded border-slate-300 text-accent focus:ring-accent" />
      {children}
    </label>
  );
}

// Bộ lọc dùng cho cả thanh bên (desktop) và hộp lọc (mobile). `roomTypes`: [{ id, name }] để lọc theo loại.
export default function RoomFilters({ value, onChange, onReset, roomTypes = [] }) {
  const errors = validateFilters(value);
  const patch = (next) => onChange({ ...value, ...next });
  const presetActive = (p) => value.minPrice === p.minPrice && value.maxPrice === p.maxPrice;

  return (
    <div>
      <div className="mb-5 flex items-center justify-between">
        <h2 className="text-lg font-bold text-brand">Bộ lọc</h2>
        {activeFilterCount(value) > 0 && (
          <button type="button" onClick={onReset} className="text-sm font-semibold text-accent hover:underline">
            Xóa bộ lọc
          </button>
        )}
      </div>

      <Group title="Sắp xếp theo">
        <label className="sr-only" htmlFor="room-sort">Sắp xếp theo</label>
        <select id="room-sort" value={value.sort} onChange={(e) => patch({ sort: e.target.value })} className="c-input">
          {SORT_OPTIONS.map((o) => (
            <option key={o.value} value={o.value}>{o.label}</option>
          ))}
        </select>
      </Group>

      <Group title="Khoảng giá mỗi đêm (₫)">
        <div className="grid grid-cols-2 gap-2">
          <div>
            <label htmlFor="price-min" className="sr-only">Giá thấp nhất</label>
            <input id="price-min" type="number" inputMode="numeric" min="0" step="100000" placeholder="Từ" value={value.minPrice} onChange={(e) => patch({ minPrice: e.target.value })} className={`c-input ${errors.price ? 'c-input-error' : ''}`} />
          </div>
          <div>
            <label htmlFor="price-max" className="sr-only">Giá cao nhất</label>
            <input id="price-max" type="number" inputMode="numeric" min="0" step="100000" placeholder="Đến" value={value.maxPrice} onChange={(e) => patch({ maxPrice: e.target.value })} className={`c-input ${errors.price ? 'c-input-error' : ''}`} />
          </div>
        </div>
        {errors.price && (
          <p role="alert" className="mt-2 flex items-center gap-1.5 text-xs text-danger">
            <AlertCircle size={13} /> {errors.price}
          </p>
        )}
        <div className="mt-3 flex flex-wrap gap-2">
          {PRICE_PRESETS.map((p) => (
            <button
              key={p.label}
              type="button"
              aria-pressed={presetActive(p)}
              onClick={() => patch(presetActive(p) ? { minPrice: '', maxPrice: '' } : { minPrice: p.minPrice, maxPrice: p.maxPrice })}
              className={`rounded-full border px-3 py-1.5 text-xs font-semibold transition ${presetActive(p) ? 'border-accent bg-accent text-white' : 'border-slate-200 bg-white text-slate-600 hover:border-accent hover:text-accent'}`}
            >
              {p.label}
            </button>
          ))}
        </div>
      </Group>

      <Group title="Loại phòng">
        {roomTypes.map((t) => (
          <Check key={t.id} checked={value.types.includes(t.id)} onChange={() => patch({ types: toggle(value.types, t.id) })}>
            {t.name}
          </Check>
        ))}
      </Group>

      <Group title="Tiện nghi">
        {ROOM_AMENITY_LIST.map((a) => (
          <Check key={a.key} checked={value.amenities.includes(a.key)} onChange={() => patch({ amenities: toggle(value.amenities, a.key) })}>
            {a.label}
          </Check>
        ))}
      </Group>

      <Group title="Tình trạng">
        <Check checked={value.onlyAvailable} onChange={() => patch({ onlyAvailable: !value.onlyAvailable })}>
          Chỉ hiện phòng còn trống
        </Check>
      </Group>
    </div>
  );
}
