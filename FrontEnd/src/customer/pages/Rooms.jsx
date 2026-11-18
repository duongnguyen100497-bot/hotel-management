import { useCallback, useEffect, useMemo, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { SlidersHorizontal, X } from 'lucide-react';
import SearchBooking from '../components/SearchBooking.jsx';
import RoomCard from '../components/RoomCard.jsx';
import RoomFilters from '../components/RoomFilters.jsx';
import { EmptyBlock, ErrorBlock, RoomListSkeleton } from '../components/States.jsx';
import useFetch from '../../shared/hooks/useFetch.js';
import useDocumentTitle from '../hooks/useDocumentTitle.js';
import customerRoomService from '../services/customerRoomService.js';
import { countNights, queryToSearch, searchToQuery } from '../utils/bookingSearch.js';
import { activeFilterCount, applyFilters, defaultFilters, filtersFromQuery, validateFilters, writeFilters } from '../utils/roomFilters.js';
import { formatDate } from '../../shared/utils/formatters.js';

export default function Rooms() {
  useDocumentTitle('Phòng nghỉ');
  const [params, setParams] = useSearchParams();
  const [sheetOpen, setSheetOpen] = useState(false);

  const search = useMemo(() => queryToSearch(params), [params]);
  const filters = useMemo(() => filtersFromQuery(params), [params]);
  const searchKey = searchToQuery(search);

  // Chỉ tải lại dữ liệu khi tìm kiếm (ngày/khách) đổi; bộ lọc được áp dụng tại chỗ.
  const load = useCallback(() => customerRoomService.searchRoomTypes(search), [searchKey]); // eslint-disable-line react-hooks/exhaustive-deps
  const { data, loading, error, reload } = useFetch(load);

  const setFilters = (next) => setParams((prev) => writeFilters(new URLSearchParams(prev), next), { replace: true });
  const resetFilters = () => setFilters(defaultFilters());
  const onSearch = (values) =>
    setParams((prev) => {
      const next = new URLSearchParams(prev);
      new URLSearchParams(searchToQuery(values)).forEach((v, k) => next.set(k, v));
      return next;
    });

  const priceInvalid = Boolean(validateFilters(filters).price);
  const results = useMemo(() => {
    if (!data) return [];
    // Khoảng giá sai (thấp nhất > cao nhất) thì bỏ qua điều kiện giá, form đã báo lỗi cho khách.
    return applyFilters(data, priceInvalid ? { ...filters, minPrice: '', maxPrice: '' } : filters);
  }, [data, filters, priceInvalid]);
  const activeCount = activeFilterCount(filters);
  const nights = countNights(search.checkIn, search.checkOut);

  useEffect(() => {
    if (!sheetOpen) return undefined;
    const onKey = (e) => e.key === 'Escape' && setSheetOpen(false);
    document.addEventListener('keydown', onKey);
    document.body.style.overflow = 'hidden';
    return () => {
      document.removeEventListener('keydown', onKey);
      document.body.style.overflow = '';
    };
  }, [sheetOpen]);

  const roomTypes = (data || []).map((t) => ({ id: t.id, name: t.name }));

  return (
    <>
      <section className="bg-brand pb-24 pt-12 sm:pt-16" aria-labelledby="rooms-page-title">
        <div className="c-container">
          <h1 id="rooms-page-title" className="text-4xl font-extrabold tracking-tight text-white sm:text-5xl">Phòng nghỉ</h1>
          <p className="mt-3 max-w-xl text-sky-200/85">
            Chọn ngày và số khách để xem hạng phòng còn trống. Khách sạn sẽ tự chọn phòng cụ thể cho bạn khi xác nhận.
          </p>
        </div>
      </section>

      <div className="c-container relative z-10 -mt-14">
        <SearchBooking key={searchKey} initial={search} onSearch={onSearch} submitLabel="Cập nhật" />
      </div>

      <div className="c-container grid gap-10 pb-24 pt-10 lg:grid-cols-[17rem_1fr] lg:gap-12">
        <aside className="hidden lg:block" aria-label="Bộ lọc phòng">
          <div className="sticky top-24 rounded-3xl border border-slate-100 bg-white p-6 shadow-soft">
            <RoomFilters value={filters} onChange={setFilters} onReset={resetFilters} roomTypes={roomTypes} />
          </div>
        </aside>

        <section aria-live="polite" aria-label="Kết quả tìm phòng">
          <div className="mb-6 flex flex-wrap items-center justify-between gap-3">
            <p className="text-sm text-muted">
              {loading ? 'Đang tìm phòng…' : (
                <>
                  <strong className="text-base text-brand">{results.length} hạng phòng</strong> phù hợp, {formatDate(search.checkIn)} – {formatDate(search.checkOut)} ({nights} đêm)
                </>
              )}
            </p>
            <button type="button" onClick={() => setSheetOpen(true)} className="c-btn-outline c-btn-sm lg:!hidden">
              <SlidersHorizontal size={15} /> Bộ lọc{activeCount > 0 ? ` (${activeCount})` : ''}
            </button>
          </div>

          {loading ? (
            <RoomListSkeleton />
          ) : error ? (
            <ErrorBlock message={error} onRetry={reload} />
          ) : results.length === 0 ? (
            <EmptyBlock
              title="Không có phòng phù hợp"
              description={activeCount > 0 ? 'Thử nới lỏng bộ lọc hoặc đổi ngày lưu trú để xem thêm phòng.' : 'Không còn phòng trong khoảng ngày này. Hãy thử chọn ngày khác.'}
              action={activeCount > 0 ? <button type="button" className="c-btn-primary" onClick={resetFilters}>Xóa bộ lọc</button> : null}
            />
          ) : (
            <div className="grid gap-x-8 gap-y-14 sm:grid-cols-2">
              {results.map((room) => (
                <RoomCard key={room.id} room={room} query={searchKey} />
              ))}
            </div>
          )}
        </section>
      </div>

      {sheetOpen && (
        <div className="fixed inset-0 z-50 flex items-end bg-brand-dark/60 lg:hidden" role="dialog" aria-modal="true" aria-label="Bộ lọc phòng">
          <div className="flex max-h-[90vh] w-full flex-col rounded-t-3xl bg-white">
            <div className="flex items-center justify-between border-b border-slate-100 px-5 py-4">
              <span className="font-bold text-brand">Lọc phòng</span>
              <button type="button" onClick={() => setSheetOpen(false)} className="flex h-9 w-9 items-center justify-center rounded-full hover:bg-surface" aria-label="Đóng bộ lọc">
                <X size={20} />
              </button>
            </div>
            <div className="overflow-y-auto px-5 py-5">
              <RoomFilters value={filters} onChange={setFilters} onReset={resetFilters} roomTypes={roomTypes} />
            </div>
            <div className="border-t border-slate-100 p-4">
              <button type="button" className="c-btn-primary c-btn-lg w-full" onClick={() => setSheetOpen(false)}>
                Xem {results.length} hạng phòng
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}
