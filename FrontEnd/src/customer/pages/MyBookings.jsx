import { useCallback, useMemo, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { CalendarX, Clock, FileText, Info, KeyRound, MapPin, Phone } from 'lucide-react';
import AccountNav from '../components/AccountNav.jsx';
import { ROOM_CATALOG } from '../data/roomCatalog.js';
import Photo from '../components/Photo.jsx';
import StatusBadge from '../components/StatusBadge.jsx';
import { EmptyBlock, ErrorBlock } from '../components/States.jsx';
import Modal from '../components/Modal.jsx';
import ConfirmDialog from '../components/ConfirmDialog.jsx';
import { useCustomerAuth } from '../contexts/CustomerAuthContext.jsx';
import { useSettings } from '../../shared/contexts/SettingsContext.jsx';
import { useToast } from '../../shared/contexts/ToastContext.jsx';
import useFetch from '../../shared/hooks/useFetch.js';
import useDocumentTitle from '../hooks/useDocumentTitle.js';
import customerBookingService from '../services/customerBookingService.js';
import invoiceService, { invoiceTotal } from '../../shared/services/invoiceService.js';
import { CUSTOMER_ROUTES } from '../routes/paths.js';
import { STATUS_TABS, bookingGrandTotal, cancelEligibility } from '../utils/bookingRules.js';
import { countNights } from '../utils/bookingSearch.js';
import { formatCurrency, formatDate, formatDateTime } from '../../shared/utils/formatters.js';

const Kv = ({ k, v }) => (
  <div className="flex justify-between gap-4 py-2 text-sm"><dt className="text-muted">{k}</dt><dd className="text-right font-medium text-ink">{v || '—'}</dd></div>
);

function BookingCard({ b, checkInTime, onDetail, onInvoice, onGuide, onCancel }) {
  const nights = countNights(b.checkIn, b.checkOut);
  const rule = cancelEligibility(b, checkInTime);
  const canGuide = b.status === 'Pending' || b.status === 'Confirmed' || b.status === 'Checked-in';
  return (
    <article className="overflow-hidden rounded-3xl border border-slate-100 bg-white shadow-soft sm:flex">
      <Photo src={ROOM_CATALOG[b.roomType]?.images?.[0]} variant={b.roomType.toLowerCase()} alt={`Phòng ${b.roomType}`} className="aspect-[16/9] w-full shrink-0 sm:aspect-auto sm:w-56" />
      <div className="flex flex-1 flex-col p-5">
        <div className="flex flex-wrap items-start justify-between gap-2">
          <div>
            <p className="font-mono text-sm font-semibold text-accent">{b.id}</p>
            <h3 className="mt-0.5 text-lg font-bold text-brand">Phòng {b.roomType}{b.roomCode ? <span className="font-normal text-muted"> ({b.roomCode})</span> : null}</h3>
          </div>
          <StatusBadge status={b.status} />
        </div>
        <dl className="mt-3 grid gap-x-6 gap-y-1 text-sm sm:grid-cols-3">
          <div><dt className="text-xs text-muted">Nhận phòng</dt><dd className="font-medium">{formatDate(b.checkIn)}</dd></div>
          <div><dt className="text-xs text-muted">Trả phòng</dt><dd className="font-medium">{formatDate(b.checkOut)}</dd></div>
          <div><dt className="text-xs text-muted">Số đêm / Tổng tiền</dt><dd className="font-medium">{nights} đêm, {formatCurrency(bookingGrandTotal(b))}</dd></div>
        </dl>
        <div className="mt-auto flex flex-wrap gap-2 pt-5">
          <button type="button" className="c-btn-outline c-btn-sm" onClick={() => onDetail(b)}><Info size={14} /> Chi tiết</button>
          <button type="button" className="c-btn-outline c-btn-sm" onClick={() => onInvoice(b)}><FileText size={14} /> Hóa đơn</button>
          {canGuide && <button type="button" className="c-btn-outline c-btn-sm" onClick={() => onGuide(b)}><KeyRound size={14} /> Hướng dẫn nhận phòng</button>}
          {(b.status === 'Pending' || b.status === 'Confirmed') && (
            <button type="button" className="c-btn-ghost c-btn-sm !text-danger hover:!bg-red-50" onClick={() => onCancel(b)} aria-describedby={rule.ok ? undefined : `cancel-note-${b.id}`}><CalendarX size={14} /> Hủy đặt phòng</button>
          )}
        </div>
        {(b.status === 'Pending' || b.status === 'Confirmed') && !rule.ok && <p id={`cancel-note-${b.id}`} className="mt-2 text-xs text-muted">{rule.reason}</p>}
      </div>
    </article>
  );
}

function InvoiceBody({ booking }) {
  const load = useCallback(async () => (await invoiceService.getAll()).find((i) => i.bookingId === booking.id) || null, [booking.id]);
  const { data: invoice, loading, error } = useFetch(load);
  if (loading) return <p role="status" className="py-6 text-center text-sm text-muted">Đang tải hóa đơn…</p>;
  if (error) return <p role="alert" className="py-6 text-center text-sm text-danger">{error}</p>;

  if (invoice) {
    return (
      <div>
        <p className="font-mono text-sm font-semibold text-accent">{invoice.id}</p>
        <dl className="mt-2 divide-y divide-slate-100">
          <Kv k="Tiền phòng" v={formatCurrency(invoice.roomCharge)} />
          {invoice.serviceItems.map((s) => <Kv key={s.name} k={`${s.name} × ${s.qty}`} v={formatCurrency(s.qty * s.price)} />)}
          {invoice.surcharge > 0 && <Kv k="Phụ thu" v={formatCurrency(invoice.surcharge)} />}
          <Kv k="Tổng cộng" v={<strong className="text-base text-brand">{formatCurrency(invoiceTotal(invoice))}</strong>} />
          <Kv k="Hình thức thanh toán" v={invoice.paymentMethod} />
          <Kv k="Trạng thái" v={invoice.paymentStatus} />
        </dl>
      </div>
    );
  }
  const hasFees = booking.grandTotal != null;
  return (
    <div>
      <p className="rounded-xl bg-amber-50 px-3 py-2 text-sm text-amber-800">Hóa đơn chính thức được xuất khi trả phòng. Dưới đây là số tiền tạm tính cho đặt phòng này.</p>
      <dl className="mt-3 divide-y divide-slate-100">
        <Kv k="Tiền phòng" v={formatCurrency(booking.total)} />
        {hasFees && <Kv k="Phí dịch vụ" v={formatCurrency(booking.serviceFee)} />}
        {hasFees && <Kv k="Thuế VAT" v={formatCurrency(booking.vat)} />}
        <Kv k="Tạm tính" v={<strong className="text-base text-brand">{formatCurrency(bookingGrandTotal(booking))}</strong>} />
        <Kv k="Thanh toán" v={booking.paymentStatus || 'Chưa thanh toán'} />
      </dl>
    </div>
  );
}

export default function MyBookings() {
  useDocumentTitle('Đặt phòng của tôi');
  const { customer } = useCustomerAuth();
  const { hotel } = useSettings();
  const toast = useToast();
  const [params, setParams] = useSearchParams();
  const rawTab = params.get('tab') === 'history' ? 'done' : params.get('tab');
  const tab = STATUS_TABS.some((t) => t.key === rawTab) ? rawTab : 'all';

  const load = useCallback(() => customerBookingService.listMine(customer.id), [customer.id]);
  const { data, loading, error, reload, refresh } = useFetch(load);

  const [detail, setDetail] = useState(null);
  const [invoice, setInvoice] = useState(null);
  const [guide, setGuide] = useState(null);
  const [toCancel, setToCancel] = useState(null);
  const [cancelling, setCancelling] = useState(false);

  const bookings = data || [];
  const counts = useMemo(() => Object.fromEntries(STATUS_TABS.map((t) => [t.key, bookings.filter(t.match).length])), [bookings]);
  const shown = bookings.filter(STATUS_TABS.find((t) => t.key === tab).match);

  // Kiểm tra chính sách ngay khi bấm: quá hạn thì báo lý do, không mở hộp xác nhận.
  const requestCancel = (b) => {
    const rule = cancelEligibility(b, hotel.checkInTime);
    if (rule.ok) setToCancel(b);
    else toast.error(rule.reason);
  };

  const doCancel = async () => {
    setCancelling(true);
    try {
      await customerBookingService.cancelMine(customer.id, toCancel.id, hotel.checkInTime);
      toast.success(`Đã hủy đặt phòng ${toCancel.id}`);
      setToCancel(null);
      refresh();
    } catch (e) {
      toast.error(e.message);
      setToCancel(null);
    } finally {
      setCancelling(false);
    }
  };

  return (
    <div className="c-container grid gap-8 py-10 pb-20 lg:grid-cols-[17rem_1fr] lg:gap-12">
      <aside className="lg:sticky lg:top-24 lg:self-start"><AccountNav active={tab === 'done' ? 'history' : 'bookings'} /></aside>

      <section aria-labelledby="mb-title" className="min-w-0">
        <h1 id="mb-title" className="text-3xl font-extrabold tracking-tight text-brand">{tab === 'done' ? 'Lịch sử đặt phòng' : 'Đặt phòng của tôi'}</h1>

        <div role="tablist" aria-label="Lọc theo trạng thái" className="mt-6 flex gap-2 overflow-x-auto pb-1">
          {STATUS_TABS.map((t) => (
            <button key={t.key} role="tab" type="button" aria-selected={tab === t.key} onClick={() => setParams(t.key === 'all' ? {} : { tab: t.key }, { replace: true })}
              className={`shrink-0 rounded-full px-4 py-2 text-sm font-semibold transition ${tab === t.key ? 'bg-brand text-white' : 'bg-surface text-slate-600 hover:bg-accent-soft hover:text-accent'}`}>
              {t.label}{!loading && <span className={`ml-1.5 ${tab === t.key ? 'text-sky-200' : 'text-muted'}`}>{counts[t.key]}</span>}
            </button>
          ))}
        </div>

        <div className="mt-6 space-y-5">
          {loading ? (
            <div role="status" aria-label="Đang tải">{[0, 1].map((i) => <div key={i} className="mb-5 h-44 animate-pulse rounded-3xl bg-slate-100" />)}</div>
          ) : error ? (
            <ErrorBlock message={error} onRetry={reload} />
          ) : shown.length === 0 ? (
            <EmptyBlock
              title={bookings.length === 0 ? 'Bạn chưa có đặt phòng nào' : 'Không có đặt phòng trong mục này'}
              description={bookings.length === 0 ? 'Chọn ngày và hạng phòng để bắt đầu kỳ nghỉ của bạn.' : 'Thử chọn mục khác ở trên.'}
              action={<Link to={CUSTOMER_ROUTES.ROOMS} className="c-btn-primary">Tìm phòng</Link>}
            />
          ) : (
            shown.map((b) => <BookingCard key={b.id} b={b} checkInTime={hotel.checkInTime} onDetail={setDetail} onInvoice={setInvoice} onGuide={setGuide} onCancel={requestCancel} />)
          )}
        </div>
      </section>

      <Modal open={Boolean(detail)} onClose={() => setDetail(null)} title={detail ? `Đặt phòng ${detail.id}` : ''} size="md" footer={<button type="button" className="c-btn-outline" onClick={() => setDetail(null)}>Đóng</button>}>
        {detail && (
          <dl className="divide-y divide-slate-100">
            <Kv k="Trạng thái" v={<StatusBadge status={detail.status} />} />
            <Kv k="Hạng phòng" v={`Phòng ${detail.roomType}${detail.roomCode ? ` (${detail.roomCode})` : ''}`} />
            <Kv k="Nhận phòng" v={`${formatDate(detail.checkIn)} từ ${hotel.checkInTime}`} />
            <Kv k="Trả phòng" v={`${formatDate(detail.checkOut)} trước ${hotel.checkOutTime}`} />
            <Kv k="Số đêm" v={`${countNights(detail.checkIn, detail.checkOut)} đêm`} />
            <Kv k="Số khách" v={detail.adults != null ? `${detail.adults} người lớn${detail.children ? `, ${detail.children} trẻ em` : ''}` : `${detail.guests} khách`} />
            <Kv k="Người nhận phòng" v={detail.customerName} />
            <Kv k="Điện thoại" v={detail.phone} />
            {detail.contactEmail && <Kv k="Email" v={detail.contactEmail} />}
            <Kv k="Ghi chú" v={(detail.note || '').replace('[Đặt trực tuyến]', '').trim()} />
            <Kv k="Thanh toán" v={detail.paymentMethod ? `${detail.paymentMethod}, ${detail.paymentStatus}` : '—'} />
            <Kv k="Tổng tiền" v={<strong className="text-brand">{formatCurrency(bookingGrandTotal(detail))}</strong>} />
            <Kv k="Ngày đặt" v={formatDateTime(detail.createdAt)} />
          </dl>
        )}
      </Modal>

      <Modal open={Boolean(invoice)} onClose={() => setInvoice(null)} title={invoice ? `Hóa đơn, ${invoice.id}` : ''} size="md" footer={<button type="button" className="c-btn-outline" onClick={() => setInvoice(null)}>Đóng</button>}>
        {invoice && <InvoiceBody booking={invoice} />}
      </Modal>

      <Modal open={Boolean(guide)} onClose={() => setGuide(null)} title="Hướng dẫn nhận phòng" size="md" footer={<button type="button" className="c-btn-outline" onClick={() => setGuide(null)}>Đóng</button>}>
        {guide && (
          <ol className="space-y-4 text-sm text-slate-700">
            <li className="flex gap-3"><Clock size={18} className="mt-0.5 shrink-0 text-accent" /><span>Nhận phòng ngày <strong>{formatDate(guide.checkIn)}</strong> từ <strong>{hotel.checkInTime}</strong>. Trả phòng trước <strong>{hotel.checkOutTime}</strong> ngày {formatDate(guide.checkOut)}.</span></li>
            <li className="flex gap-3"><KeyRound size={18} className="mt-0.5 shrink-0 text-accent" /><span>Đến quầy lễ tân, đọc mã đặt phòng <strong className="font-mono">{guide.id}</strong> và xuất trình CCCD/hộ chiếu của người nhận phòng ({guide.customerName}).</span></li>
            <li className="flex gap-3"><MapPin size={18} className="mt-0.5 shrink-0 text-accent" /><span>{hotel.name}, {hotel.address}.</span></li>
            <li className="flex gap-3"><Phone size={18} className="mt-0.5 shrink-0 text-accent" /><span>Đến muộn hoặc cần hỗ trợ, gọi {hotel.phone}. Lễ tân phục vụ 24/7.</span></li>
            {guide.status === 'Pending' && <li className="rounded-xl bg-amber-50 px-3 py-2 text-amber-800">Đặt phòng đang chờ khách sạn xác nhận. Bạn sẽ được liên hệ trước ngày nhận phòng.</li>}
          </ol>
        )}
      </Modal>

      <ConfirmDialog open={Boolean(toCancel)} danger loading={cancelling} title="Hủy đặt phòng" confirmText="Hủy đặt phòng" cancelText="Giữ đặt phòng"
        message={toCancel ? `Bạn chắc chắn muốn hủy đặt phòng ${toCancel.id} (Phòng ${toCancel.roomType}, ${formatDate(toCancel.checkIn)})? Thao tác này không thể hoàn tác.` : ''}
        onConfirm={doCancel} onCancel={() => setToCancel(null)} />
    </div>
  );
}
