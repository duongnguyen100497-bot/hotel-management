import { useState } from 'react';
import { Loader2, LogIn } from 'lucide-react';
import Modal from '../../components/common/Modal.jsx';
import InfoRow from '../../components/common/InfoRow.jsx';
import { diffDays } from '../../../shared/utils/dateUtils.js';
import { formatCurrency, formatDate, formatDateTime } from '../../../shared/utils/formatters.js';

// target: { booking, viewOnly? } – hiển thị thông tin đặt phòng + khách hàng, xác nhận check-in.
export default function CheckInModal({ target, customer, onClose, onConfirm }) {
  const [saving, setSaving] = useState(false);
  if (!target) return null;
  const { booking, viewOnly } = target;

  const confirm = async () => {
    setSaving(true);
    try {
      await onConfirm(booking);
    } catch {
      /* lỗi đã được thông báo ở trang cha */
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal
      open
      onClose={saving ? undefined : onClose}
      title={viewOnly ? `Thông tin đặt phòng ${booking.id}` : 'Xác nhận check-in'}
      subtitle={`Phòng ${booking.roomCode} · ${booking.roomType}`}
      footer={
        <>
          <button type="button" className="btn-secondary" onClick={onClose} disabled={saving}>
            {viewOnly ? 'Đóng' : 'Hủy bỏ'}
          </button>
          {!viewOnly && (
            <button type="button" className="btn-primary" onClick={confirm} disabled={saving}>
              {saving ? <Loader2 size={16} className="animate-spin" /> : <LogIn size={16} />}
              Xác nhận check-in
            </button>
          )}
        </>
      }
    >
      <h3 className="mb-2 text-sm font-semibold text-navy-900">Thông tin đặt phòng</h3>
      <dl className="space-y-2.5">
        <InfoRow label="Lưu trú">
          {formatDate(booking.checkIn)} → {formatDate(booking.checkOut)} ({diffDays(booking.checkIn, booking.checkOut)} đêm)
        </InfoRow>
        <InfoRow label="Số khách">{booking.guests}</InfoRow>
        <InfoRow label="Tiền phòng">{formatCurrency(booking.total)}</InfoRow>
        <InfoRow label="Ghi chú">{booking.note}</InfoRow>
      </dl>

      <h3 className="mb-2 mt-5 text-sm font-semibold text-navy-900">Thông tin khách hàng</h3>
      <dl className="space-y-2.5">
        <InfoRow label="Họ và tên">{customer?.fullName || booking.customerName}</InfoRow>
        <InfoRow label="Số điện thoại">{customer?.phone || booking.phone}</InfoRow>
        <InfoRow label="CCCD / Hộ chiếu">{customer?.idNumber}</InfoRow>
        <InfoRow label="Quốc tịch">{customer?.nationality}</InfoRow>
      </dl>

      {!viewOnly && (
        <div className="mt-5 rounded-xl bg-sky-50 p-3 text-sm text-slate-600">
          Thời gian nhận phòng ghi nhận: <b className="text-navy-800">{formatDateTime(new Date().toISOString())}</b>. Phòng sẽ chuyển
          sang trạng thái <b>Đang sử dụng</b>.
        </div>
      )}
    </Modal>
  );
}
