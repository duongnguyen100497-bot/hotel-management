import { Ban, Pencil } from 'lucide-react';
import Modal from '../../components/common/Modal.jsx';
import InfoRow from '../../components/common/InfoRow.jsx';
import { BookingStatusBadge } from '../../components/common/StatusBadge.jsx';
import { diffDays } from '../../../shared/utils/dateUtils.js';
import { formatCurrency, formatDate, formatDateTime } from '../../../shared/utils/formatters.js';

export default function BookingDetailModal({ booking, onClose, onEdit, onCancel }) {
  if (!booking) return null;
  const editable = ['Pending', 'Confirmed', 'Checked-in'].includes(booking.status);
  const cancellable = ['Pending', 'Confirmed'].includes(booking.status);

  return (
    <Modal
      open
      onClose={onClose}
      title={`Đặt phòng ${booking.id}`}
      subtitle={booking.customerName}
      size="lg"
      footer={
        <>
          {cancellable && (
            <button type="button" className="btn-secondary" onClick={() => onCancel(booking)}>
              <Ban size={15} className="text-red-500" /> Hủy đặt phòng
            </button>
          )}
          {editable && (
            <button type="button" className="btn-primary" onClick={() => onEdit(booking)}>
              <Pencil size={15} /> Chỉnh sửa
            </button>
          )}
          {!editable && (
            <button type="button" className="btn-secondary" onClick={onClose}>
              Đóng
            </button>
          )}
        </>
      }
    >
      <dl className="space-y-3">
        <InfoRow label="Trạng thái">
          <BookingStatusBadge status={booking.status} />
        </InfoRow>
        <InfoRow label="Khách hàng">
          {booking.customerName} ({booking.customerId})
        </InfoRow>
        <InfoRow label="Số điện thoại">{booking.phone}</InfoRow>
        <InfoRow label="Phòng">
          {booking.roomCode} – {booking.roomType}
        </InfoRow>
        <InfoRow label="Nhận phòng">{formatDate(booking.checkIn)}</InfoRow>
        <InfoRow label="Trả phòng">{formatDate(booking.checkOut)}</InfoRow>
        <InfoRow label="Số đêm">{diffDays(booking.checkIn, booking.checkOut)} đêm</InfoRow>
        <InfoRow label="Số khách">{booking.guests}</InfoRow>
        <InfoRow label="Tổng tiền phòng">
          <span className="text-base font-semibold text-navy-800">{formatCurrency(booking.total)}</span>
        </InfoRow>
        {booking.checkInTime && <InfoRow label="Check-in thực tế">{formatDateTime(booking.checkInTime)}</InfoRow>}
        {booking.checkOutTime && <InfoRow label="Check-out thực tế">{formatDateTime(booking.checkOutTime)}</InfoRow>}
        <InfoRow label="Ghi chú">{booking.note}</InfoRow>
        <InfoRow label="Ngày tạo">{formatDateTime(booking.createdAt)}</InfoRow>
      </dl>
    </Modal>
  );
}
