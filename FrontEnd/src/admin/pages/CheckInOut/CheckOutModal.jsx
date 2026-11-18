import { useEffect, useMemo, useState } from 'react';
import { Loader2, LogOut } from 'lucide-react';
import Modal from '../../components/common/Modal.jsx';
import { Input, Select } from '../../components/common/FormField.jsx';
import { PAYMENT_METHODS } from '../../../shared/utils/constants.js';
import { formatCurrency, formatDate, formatDateTime } from '../../../shared/utils/formatters.js';

// Xác nhận check-out: chọn dịch vụ đã dùng, phụ thu → tạo hóa đơn, phòng chuyển sang "Đang dọn dẹp".
export default function CheckOutModal({ booking, customer, services, onClose, onConfirm }) {
  const [qty, setQty] = useState({});
  const [surcharge, setSurcharge] = useState('0');
  const [method, setMethod] = useState(PAYMENT_METHODS[0]);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (booking) {
      setQty({});
      setSurcharge('0');
      setMethod(PAYMENT_METHODS[0]);
    }
  }, [booking]);

  const serviceItems = useMemo(
    () =>
      services
        .filter((s) => Number(qty[s.id]) > 0)
        .map((s) => ({ name: s.name, qty: Number(qty[s.id]), price: s.price })),
    [services, qty],
  );

  if (!booking) return null;

  const serviceCharge = serviceItems.reduce((sum, it) => sum + it.qty * it.price, 0);
  const extra = Math.max(0, Number(surcharge) || 0);
  const total = booking.total + serviceCharge + extra;

  const confirm = async () => {
    setSaving(true);
    try {
      await onConfirm(booking, { serviceItems, surcharge: extra, paymentMethod: method });
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
      title="Xác nhận check-out"
      subtitle={`${booking.customerName} · Phòng ${booking.roomCode}`}
      size="lg"
      footer={
        <>
          <button type="button" className="btn-secondary" onClick={onClose} disabled={saving}>
            Hủy bỏ
          </button>
          <button type="button" className="btn-primary" onClick={confirm} disabled={saving}>
            {saving ? <Loader2 size={16} className="animate-spin" /> : <LogOut size={16} />}
            Xác nhận check-out
          </button>
        </>
      }
    >
      <div className="grid grid-cols-2 gap-3 rounded-xl bg-slate-50 p-4 text-sm sm:grid-cols-4">
        <div>
          <p className="text-xs text-slate-400">Khách hàng</p>
          <p className="font-medium text-slate-800">{customer?.fullName || booking.customerName}</p>
        </div>
        <div>
          <p className="text-xs text-slate-400">Điện thoại</p>
          <p className="font-medium text-slate-800">{customer?.phone || booking.phone}</p>
        </div>
        <div>
          <p className="text-xs text-slate-400">Nhận phòng thực tế</p>
          <p className="font-medium text-slate-800">{formatDateTime(booking.checkInTime) || formatDate(booking.checkIn)}</p>
        </div>
        <div>
          <p className="text-xs text-slate-400">Trả phòng dự kiến</p>
          <p className="font-medium text-slate-800">{formatDate(booking.checkOut)}</p>
        </div>
      </div>

      <h3 className="mb-2 mt-5 text-sm font-semibold text-navy-900">Dịch vụ đã sử dụng</h3>
      <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
        {services.map((s) => (
          <label key={s.id} className="flex items-center justify-between gap-3 rounded-lg border border-slate-200 px-3 py-2 text-sm">
            <span>
              <span className="block font-medium text-slate-700">{s.name}</span>
              <span className="text-xs text-slate-400">{formatCurrency(s.price)}</span>
            </span>
            <input
              type="number"
              min="0"
              value={qty[s.id] ?? ''}
              placeholder="0"
              onChange={(e) => setQty((q) => ({ ...q, [s.id]: e.target.value }))}
              className="input !w-20 text-center"
              aria-label={`Số lượng ${s.name}`}
            />
          </label>
        ))}
      </div>

      <div className="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-2">
        <Input label="Phụ thu (VND)" type="number" min="0" step="10000" value={surcharge} onChange={(e) => setSurcharge(e.target.value)} hint="Hư hỏng, trả phòng muộn, ..." />
        <Select label="Phương thức thanh toán" value={method} onChange={(e) => setMethod(e.target.value)} options={PAYMENT_METHODS} />
      </div>

      <div className="mt-5 space-y-1.5 rounded-xl bg-sky-50 p-4 text-sm">
        <div className="flex justify-between text-slate-600">
          <span>Tiền phòng</span>
          <span>{formatCurrency(booking.total)}</span>
        </div>
        <div className="flex justify-between text-slate-600">
          <span>Tiền dịch vụ</span>
          <span>{formatCurrency(serviceCharge)}</span>
        </div>
        <div className="flex justify-between text-slate-600">
          <span>Phụ thu</span>
          <span>{formatCurrency(extra)}</span>
        </div>
        <div className="flex justify-between border-t border-sky-200 pt-2 text-base font-semibold text-navy-900">
          <span>Tổng cộng</span>
          <span>{formatCurrency(total)}</span>
        </div>
      </div>
      <p className="mt-3 text-xs text-slate-400">
        Sau khi xác nhận, hóa đơn được tạo ở trạng thái chưa thanh toán và phòng {booking.roomCode} chuyển sang "Đang dọn dẹp".
      </p>
    </Modal>
  );
}
