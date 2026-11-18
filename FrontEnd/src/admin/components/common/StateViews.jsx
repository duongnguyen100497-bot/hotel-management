import { AlertCircle, Inbox, Loader2, RefreshCw } from 'lucide-react';

export function LoadingState({ label = 'Đang tải dữ liệu...' }) {
  return (
    <div className="flex flex-col items-center justify-center gap-3 py-16 text-slate-500">
      <Loader2 size={28} className="animate-spin text-navy-500" />
      <p className="text-sm">{label}</p>
    </div>
  );
}

export function EmptyState({ icon: Icon = Inbox, title = 'Không có dữ liệu', description, action }) {
  return (
    <div className="flex flex-col items-center justify-center gap-2 px-4 py-14 text-center">
      <span className="flex h-14 w-14 items-center justify-center rounded-full bg-sky-50 text-navy-400">
        <Icon size={26} />
      </span>
      <h3 className="mt-1 text-base font-semibold text-navy-900">{title}</h3>
      {description && <p className="max-w-sm text-sm text-slate-500">{description}</p>}
      {action && <div className="mt-3">{action}</div>}
    </div>
  );
}

export function ErrorState({ message = 'Đã có lỗi xảy ra', onRetry }) {
  return (
    <div className="flex flex-col items-center justify-center gap-2 px-4 py-14 text-center">
      <span className="flex h-14 w-14 items-center justify-center rounded-full bg-red-50 text-red-500">
        <AlertCircle size={26} />
      </span>
      <h3 className="mt-1 text-base font-semibold text-navy-900">Không thể tải dữ liệu</h3>
      <p className="max-w-sm text-sm text-slate-500">{message}</p>
      {onRetry && (
        <button type="button" onClick={onRetry} className="btn-secondary mt-3">
          <RefreshCw size={15} /> Thử lại
        </button>
      )}
    </div>
  );
}

// Bọc bảng/danh sách: tự chọn loading / error / empty / nội dung.
export function DataState({ loading, error, onRetry, isEmpty, emptyProps, children }) {
  if (loading) return <LoadingState />;
  if (error) return <ErrorState message={error} onRetry={onRetry} />;
  if (isEmpty) return <EmptyState {...emptyProps} />;
  return children;
}
