import { useEffect, useState } from 'react';
import { Pencil, Trash2 } from 'lucide-react';
import Modal from '../../components/common/Modal.jsx';
import InfoRow from '../../components/common/InfoRow.jsx';
import { BookingStatusBadge, RoomStatusBadge } from '../../components/common/StatusBadge.jsx';
import bookingService from '../../../shared/services/bookingService.js';
import { ROOM_STATUS, ROOM_STATUS_LIST } from '../../../shared/utils/constants.js';
import { formatCurrency, formatDate } from '../../../shared/utils/formatters.js';

export default function RoomDetailModal({ room, onClose, onEdit, onDelete, onChangeStatus }) {
  const [bookings, setBookings] = useState(null);

  useEffect(() => {
    if (!room) return undefined;
    let alive = true;
    setBookings(null);
    bookingService.getByRoom(room.code).then((list) => {
      if (alive) setBookings(list);
    });
    return () => {
      alive = false;
    };
  }, [room]);

  if (!room) return null;
  const relevant = (bookings || []).filter((b) => ['Checked-in', 'Confirmed', 'Pending'].includes(b.status));

  return (
    <Modal
      open={Boolean(room)}
      onClose={onClose}
      title={`${room.name} (${room.code})`}
      subtitle={`${room.type} · Tầng ${room.floor}`}
      size="lg"
      footer={
        <>
          <button type="button" className="btn-secondary" onClick={() => onDelete(room)}>
            <Trash2 size={15} className="text-red-500" /> Xóa
          </button>
          <button type="button" className="btn-primary" onClick={() => onEdit(room)}>
            <Pencil size={15} /> Chỉnh sửa
          </button>
        </>
      }
    >
      <dl className="space-y-3">
        <InfoRow label="Trạng thái">
          <RoomStatusBadge status={room.status} />
        </InfoRow>
        <InfoRow label="Loại phòng">{room.type}</InfoRow>
        <InfoRow label="Giá phòng">{formatCurrency(room.price)} / đêm</InfoRow>
        <InfoRow label="Sức chứa">{room.capacity} người</InfoRow>
        <InfoRow label="Mô tả">{room.description}</InfoRow>
      </dl>

      <div className="mt-5 rounded-xl bg-slate-50 p-4">
        <p className="mb-2 text-sm font-semibold text-navy-900">Cập nhật nhanh trạng thái</p>
        <div className="flex flex-wrap gap-2">
          {ROOM_STATUS_LIST.map((s) => (
            <button
              key={s}
              type="button"
              disabled={s === room.status}
              onClick={() => onChangeStatus(room, s)}
              className={`rounded-full border px-3 py-1 text-xs font-medium transition-colors disabled:cursor-default ${
                s === room.status
                  ? 'border-navy-700 bg-navy-800 text-white'
                  : 'border-slate-200 bg-white text-slate-600 hover:border-navy-300 hover:bg-sky-50'
              }`}
            >
              {ROOM_STATUS[s].label}
            </button>
          ))}
        </div>
      </div>

      <div className="mt-5">
        <p className="mb-2 text-sm font-semibold text-navy-900">Đặt phòng hiện tại / sắp tới</p>
        {bookings === null ? (
          <p className="text-sm text-slate-400">Đang tải...</p>
        ) : relevant.length === 0 ? (
          <p className="text-sm text-slate-400">Không có đặt phòng nào đang hoạt động.</p>
        ) : (
          <ul className="divide-y divide-slate-100 rounded-xl border border-slate-100">
            {relevant.map((b) => (
              <li key={b.id} className="flex flex-wrap items-center justify-between gap-2 px-4 py-3 text-sm">
                <div>
                  <p className="font-medium text-slate-800">{b.customerName}</p>
                  <p className="text-xs text-slate-400">
                    {b.id} · {formatDate(b.checkIn)} → {formatDate(b.checkOut)}
                  </p>
                </div>
                <BookingStatusBadge status={b.status} />
              </li>
            ))}
          </ul>
        )}
      </div>
    </Modal>
  );
}
