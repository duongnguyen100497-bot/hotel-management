import { AlertCircle, Loader2, RefreshCw, SearchX } from 'lucide-react';

export function EmptyBlock({ icon: Icon = SearchX, title, description, action, className = '' }) {
  return (
    <div className={`flex flex-col items-center rounded-3xl border border-dashed border-slate-300 bg-white px-6 py-16 text-center ${className}`}>
      <span className="flex h-16 w-16 items-center justify-center rounded-full bg-accent-soft text-accent">
        <Icon size={28} />
      </span>
      <h3 className="mt-5 text-xl font-bold text-brand">{title}</h3>
      {description && <p className="mt-2 max-w-md text-muted">{description}</p>}
      {action && <div className="mt-6">{action}</div>}
    </div>
  );
}

export function ErrorBlock({ message, onRetry, className = '' }) {
  return (
    <div role="alert" className={`flex flex-col items-center rounded-3xl border border-red-100 bg-red-50/50 px-6 py-14 text-center ${className}`}>
      <span className="flex h-16 w-16 items-center justify-center rounded-full bg-red-100 text-danger">
        <AlertCircle size={28} />
      </span>
      <h3 className="mt-5 text-xl font-bold text-brand">Không tải được dữ liệu</h3>
      <p className="mt-2 max-w-md text-muted">{message || 'Đã có lỗi xảy ra. Vui lòng thử lại.'}</p>
      {onRetry && (
        <button type="button" onClick={onRetry} className="c-btn-outline mt-6">
          <RefreshCw size={16} /> Thử lại
        </button>
      )}
    </div>
  );
}

// Khung chờ giữ đúng hình dạng thẻ phòng để trang không "nhảy" khi dữ liệu về.
export function RoomCardSkeleton() {
  return (
    <div className="animate-pulse" aria-hidden="true">
      <div className="c-arch aspect-[1/1.08] w-full bg-slate-200" />
      <div className="mt-5 h-6 w-2/3 rounded bg-slate-200" />
      <div className="mt-3 h-4 w-full rounded bg-slate-100" />
      <div className="mt-2 h-4 w-4/5 rounded bg-slate-100" />
      <div className="mt-6 flex justify-between">
        <div className="h-8 w-32 rounded bg-slate-200" />
        <div className="h-10 w-28 rounded-xl bg-slate-200" />
      </div>
    </div>
  );
}

export function RoomListSkeleton({ count = 4 }) {
  return (
    <div role="status" aria-label="Đang tải danh sách phòng" className="grid gap-x-8 gap-y-12 sm:grid-cols-2">
      {Array.from({ length: count }, (_, i) => (
        <RoomCardSkeleton key={i} />
      ))}
    </div>
  );
}

export function LoadingBlock({ label = 'Đang tải...' }) {
  return (
    <div role="status" className="flex flex-col items-center justify-center gap-3 py-16 text-muted">
      <Loader2 size={28} className="animate-spin text-accent" />
      <p className="text-sm">{label}</p>
    </div>
  );
}

// Bọc nội dung tải từ dịch vụ: tự chọn loading / error / empty / nội dung.
export function DataBlock({ loading, error, onRetry, isEmpty, emptyProps, children }) {
  if (loading) return <LoadingBlock />;
  if (error) return <ErrorBlock message={error} onRetry={onRetry} />;
  if (isEmpty) return <EmptyBlock {...emptyProps} />;
  return children;
}
