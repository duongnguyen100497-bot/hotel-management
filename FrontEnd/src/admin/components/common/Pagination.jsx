import { ChevronLeft, ChevronRight } from 'lucide-react';

const windowPages = (page, totalPages) => {
  const size = 5;
  let start = Math.max(1, page - Math.floor(size / 2));
  const end = Math.min(totalPages, start + size - 1);
  start = Math.max(1, end - size + 1);
  return Array.from({ length: end - start + 1 }, (_, i) => start + i);
};

export default function Pagination({ page, totalPages, total, pageSize, onPageChange }) {
  if (total === 0) return null;
  const from = (page - 1) * pageSize + 1;
  const to = Math.min(page * pageSize, total);

  return (
    <div className="no-print flex flex-col items-center justify-between gap-3 border-t border-slate-100 px-4 py-3 text-sm text-slate-500 sm:flex-row">
      <span>
        Hiển thị <b className="text-slate-700">{from}–{to}</b> / {total} bản ghi
      </span>
      <div className="flex items-center gap-1">
        <button
          type="button"
          className="btn-icon disabled:opacity-40"
          disabled={page <= 1}
          onClick={() => onPageChange(page - 1)}
          aria-label="Trang trước"
        >
          <ChevronLeft size={18} />
        </button>
        {windowPages(page, totalPages).map((p) => (
          <button
            key={p}
            type="button"
            onClick={() => onPageChange(p)}
            className={`h-8 min-w-8 rounded-lg px-2 text-sm font-medium transition-colors ${
              p === page ? 'bg-navy-800 text-white' : 'text-slate-600 hover:bg-sky-50'
            }`}
          >
            {p}
          </button>
        ))}
        <button
          type="button"
          className="btn-icon disabled:opacity-40"
          disabled={page >= totalPages}
          onClick={() => onPageChange(page + 1)}
          aria-label="Trang sau"
        >
          <ChevronRight size={18} />
        </button>
      </div>
    </div>
  );
}
