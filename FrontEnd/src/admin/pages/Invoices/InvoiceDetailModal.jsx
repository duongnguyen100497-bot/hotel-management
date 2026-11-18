import { useEffect, useState } from 'react';
import { CheckCircle2, Loader2, Printer } from 'lucide-react';
import Modal from '../../components/common/Modal.jsx';
import { PaymentStatusBadge } from '../../components/common/StatusBadge.jsx';
import { useSettings } from '../../../shared/contexts/SettingsContext.jsx';
import { invoiceTotal } from '../../../shared/services/invoiceService.js';
import { PAYMENT_METHODS } from '../../../shared/utils/constants.js';
import { diffDays } from '../../../shared/utils/dateUtils.js';
import { printModalContent } from '../../utils/print.js';
import { formatCurrency, formatDate, formatDateTime } from '../../../shared/utils/formatters.js';

// Chi tiết hóa đơn: xem, in và xác nhận thanh toán.
export default function InvoiceDetailModal({ invoice, onClose, onPay }) {
  const { hotel } = useSettings();
  const [method, setMethod] = useState(PAYMENT_METHODS[0]);
  const [paying, setPaying] = useState(false);

  useEffect(() => {
    if (invoice) setMethod(invoice.paymentMethod || PAYMENT_METHODS[0]);
  }, [invoice]);

  if (!invoice) return null;

  const unpaid = invoice.paymentStatus === 'Chưa thanh toán';
  const items = invoice.serviceItems || [];
  const nights = Math.max(1, diffDays(invoice.checkIn, invoice.checkOut));
  const total = invoiceTotal(invoice);

  const pay = async () => {
    setPaying(true);
    try {
      await onPay(invoice, method);
    } finally {
      setPaying(false);
    }
  };

  return (
    <Modal
      open
      onClose={paying ? undefined : onClose}
      title={`Hóa đơn ${invoice.id}`}
      subtitle={`Lập lúc ${formatDateTime(invoice.createdAt)}`}
      size="lg"
      footer={
        <>
          {unpaid && (
            <div className="mr-auto flex items-center gap-2">
              <select className="input !w-44" value={method} onChange={(e) => setMethod(e.target.value)} disabled={paying} aria-label="Phương thức thanh toán">
                {PAYMENT_METHODS.map((m) => (
                  <option key={m} value={m}>
                    {m}
                  </option>
                ))}
              </select>
              <button type="button" className="btn-primary" onClick={pay} disabled={paying}>
                {paying ? <Loader2 size={16} className="animate-spin" /> : <CheckCircle2 size={16} />}
                Xác nhận thanh toán
              </button>
            </div>
          )}
          <button type="button" className="btn-secondary" onClick={onClose} disabled={paying}>
            Đóng
          </button>
          <button type="button" className="btn-secondary" onClick={printModalContent}>
            <Printer size={16} /> In hóa đơn
          </button>
        </>
      }
    >
      <div className="text-sm">
        <div className="flex flex-col gap-3 border-b border-slate-200 pb-4 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <p className="text-lg font-bold text-navy-900">{hotel.name}</p>
            <p className="text-slate-500">{hotel.address}</p>
            <p className="text-slate-500">
              {hotel.phone} · {hotel.email}
            </p>
          </div>
          <div className="sm:text-right">
            <p className="text-xl font-bold uppercase tracking-wide text-navy-800">Hóa đơn</p>
            <p className="font-semibold text-slate-700">{invoice.id}</p>
            <div className="mt-1">
              <PaymentStatusBadge status={invoice.paymentStatus} />
            </div>
          </div>
        </div>

        <div className="grid grid-cols-1 gap-x-6 gap-y-1.5 py-4 sm:grid-cols-2">
          <p>
            <span className="text-slate-400">Khách hàng: </span>
            <b className="text-slate-800">{invoice.customerName}</b>
          </p>
          <p>
            <span className="text-slate-400">Mã đặt phòng: </span>
            <b className="text-slate-800">{invoice.bookingId}</b>
          </p>
          <p>
            <span className="text-slate-400">Phòng: </span>
            <b className="text-slate-800">{invoice.roomCode}</b>
          </p>
          <p>
            <span className="text-slate-400">Lưu trú: </span>
            <b className="text-slate-800">
              {formatDate(invoice.checkIn)} – {formatDate(invoice.checkOut)} ({nights} đêm)
            </b>
          </p>
          <p>
            <span className="text-slate-400">Phương thức thanh toán: </span>
            <b className="text-slate-800">{invoice.paymentMethod}</b>
          </p>
          {invoice.paidAt && (
            <p>
              <span className="text-slate-400">Thanh toán lúc: </span>
              <b className="text-slate-800">{formatDateTime(invoice.paidAt)}</b>
            </p>
          )}
        </div>

        <div className="overflow-x-auto">
          <table className="w-full min-w-[480px] text-left">
            <thead>
              <tr className="border-y border-slate-200 bg-slate-50 text-xs uppercase tracking-wide text-slate-500">
                <th className="px-3 py-2 font-semibold">Hạng mục</th>
                <th className="px-3 py-2 text-right font-semibold">SL</th>
                <th className="px-3 py-2 text-right font-semibold">Đơn giá</th>
                <th className="px-3 py-2 text-right font-semibold">Thành tiền</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              <tr>
                <td className="px-3 py-2">Tiền phòng {invoice.roomCode}</td>
                <td className="px-3 py-2 text-right">{nights} đêm</td>
                <td className="px-3 py-2 text-right">{formatCurrency(invoice.roomCharge / nights)}</td>
                <td className="px-3 py-2 text-right font-medium">{formatCurrency(invoice.roomCharge)}</td>
              </tr>
              {items.map((it, i) => (
                <tr key={`${it.name}-${i}`}>
                  <td className="px-3 py-2">{it.name}</td>
                  <td className="px-3 py-2 text-right">{it.qty}</td>
                  <td className="px-3 py-2 text-right">{formatCurrency(it.price)}</td>
                  <td className="px-3 py-2 text-right font-medium">{formatCurrency(it.qty * it.price)}</td>
                </tr>
              ))}
              {items.length === 0 && (
                <tr>
                  <td colSpan={4} className="px-3 py-2 text-slate-400">
                    Không sử dụng dịch vụ.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>

        <div className="ml-auto mt-4 w-full space-y-1.5 sm:w-72">
          <div className="flex justify-between text-slate-600">
            <span>Tiền phòng</span>
            <span>{formatCurrency(invoice.roomCharge)}</span>
          </div>
          <div className="flex justify-between text-slate-600">
            <span>Tiền dịch vụ</span>
            <span>{formatCurrency(invoice.serviceCharge)}</span>
          </div>
          <div className="flex justify-between text-slate-600">
            <span>Phụ thu</span>
            <span>{formatCurrency(invoice.surcharge)}</span>
          </div>
          <div className="flex justify-between border-t border-slate-200 pt-2 text-base font-bold text-navy-900">
            <span>Tổng cộng</span>
            <span>{formatCurrency(total)}</span>
          </div>
        </div>

        <p className="mt-6 text-center text-xs text-slate-400">Cảm ơn quý khách đã lựa chọn {hotel.name}.</p>
      </div>
    </Modal>
  );
}
