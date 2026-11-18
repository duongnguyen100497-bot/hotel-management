import { useEffect, useState } from 'react';
import { Pencil, Trash2 } from 'lucide-react';
import Modal from '../../components/common/Modal.jsx';
import InfoRow from '../../components/common/InfoRow.jsx';
import Avatar from '../../components/common/Avatar.jsx';
import { BookingStatusBadge } from '../../components/common/StatusBadge.jsx';
import bookingService from '../../../shared/services/bookingService.js';
import { formatCurrency, formatDate } from '../../../shared/utils/formatters.js';

// Hồ sơ khách hàng + lịch sử lưu trú / đặt phòng.
export default function CustomerDetailModal({ customer, onClose, onEdit, onDelete }) {
  const [history, setHistory] = useState(null);

  useEffect(() => {
    if (!customer) return undefined;
    let alive = true;
    setHistory(null);
    bookingService.getByCustomer(customer.id).then((list) => {
      if (alive) setHistory(list);
    });
    return () => {
      alive = false;
    };
  }, [customer]);

  if (!customer) return null;

  return (
    <Modal
      open
      onClose={onClose}
      title="Hồ sơ khách hàng"
      size="lg"
      footer={
        <>
          <button type="button" className="btn-secondary" onClick={() => onDelete(customer)}>
            <Trash2 size={15} className="text-red-500" /> Xóa
          </button>
          <button type="button" className="btn-primary" onClick={() => onEdit(customer)}>
            <Pencil size={15} /> Chỉnh sửa
          </button>
        </>
      }
    >
      <div className="mb-4 flex items-center gap-3">
        <Avatar name={customer.fullName} size={52} />
        <div>
          <p className="text-lg font-semibold text-navy-900">{customer.fullName}</p>
          <p className="text-sm text-slate-400">{customer.id}</p>
        </div>
      </div>
      <dl className="space-y-3">
        <InfoRow label="Số điện thoại">{customer.phone}</InfoRow>
        <InfoRow label="Email">{customer.email}</InfoRow>
        <InfoRow label="CCCD / Hộ chiếu">{customer.idNumber}</InfoRow>
        <InfoRow label="Quốc tịch">{customer.nationality}</InfoRow>
        <InfoRow label="Địa chỉ">{customer.address}</InfoRow>
      </dl>

      <h3 className="mb-2 mt-6 text-sm font-semibold text-navy-900">Lịch sử lưu trú</h3>
      {history === null ? (
        <p className="text-sm text-slate-400">Đang tải...</p>
      ) : history.length === 0 ? (
        <p className="rounded-xl bg-slate-50 py-6 text-center text-sm text-slate-400">Khách chưa có lịch sử đặt phòng.</p>
      ) : (
        <div className="overflow-x-auto rounded-xl border border-slate-100">
          <table className="w-full min-w-max text-left text-sm">
            <thead>
              <tr className="bg-slate-50 text-xs uppercase tracking-wide text-slate-500">
                <th className="px-4 py-2.5 font-semibold">Mã</th>
                <th className="px-4 py-2.5 font-semibold">Phòng</th>
                <th className="px-4 py-2.5 font-semibold">Thời gian</th>
                <th className="px-4 py-2.5 font-semibold">Tổng tiền</th>
                <th className="px-4 py-2.5 font-semibold">Trạng thái</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {history.map((b) => (
                <tr key={b.id}>
                  <td className="px-4 py-2.5 font-medium text-navy-800">{b.id}</td>
                  <td className="px-4 py-2.5">
                    {b.roomCode} <span className="text-xs text-slate-400">({b.roomType})</span>
                  </td>
                  <td className="px-4 py-2.5">
                    {formatDate(b.checkIn)} → {formatDate(b.checkOut)}
                  </td>
                  <td className="px-4 py-2.5">{formatCurrency(b.total)}</td>
                  <td className="px-4 py-2.5">
                    <BookingStatusBadge status={b.status} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </Modal>
  );
}
