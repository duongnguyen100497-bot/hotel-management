import { AlertTriangle, Loader2 } from 'lucide-react';
import Modal from './Modal.jsx';

export default function ConfirmDialog({
  open,
  title = 'Xác nhận',
  message,
  confirmText = 'Xác nhận',
  cancelText = 'Giữ lại',
  danger = false,
  loading = false,
  onConfirm,
  onCancel,
}) {
  return (
    <Modal
      open={open}
      onClose={loading ? undefined : onCancel}
      title={title}
      size="sm"
      footer={
        <>
          <button type="button" className="c-btn-outline" onClick={onCancel} disabled={loading}>{cancelText}</button>
          <button type="button" className={danger ? 'c-btn-primary !bg-danger hover:!bg-red-700' : 'c-btn-primary'} onClick={onConfirm} disabled={loading}>
            {loading && <Loader2 size={16} className="animate-spin" />}
            {confirmText}
          </button>
        </>
      }
    >
      <div className="flex gap-3">
        <span className={`flex h-10 w-10 shrink-0 items-center justify-center rounded-full ${danger ? 'bg-red-50 text-danger' : 'bg-accent-soft text-accent'}`}>
          <AlertTriangle size={20} />
        </span>
        <div className="text-sm leading-relaxed text-slate-600">{message}</div>
      </div>
    </Modal>
  );
}
