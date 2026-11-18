import { useCallback, useMemo, useState } from 'react';
import { Link, useParams, useSearchParams } from 'react-router-dom';
import { AlertCircle, ArrowLeft, ArrowRight, Banknote, CheckCircle2, CreditCard, Landmark, Loader2, Pencil, XCircle } from 'lucide-react';
import SearchBooking from '../components/SearchBooking.jsx';
import BookingStepper from '../components/BookingStepper.jsx';
import PriceSummary from '../components/PriceSummary.jsx';
import { TextField } from '../components/FormControls.jsx';
import { EmptyBlock, ErrorBlock } from '../components/States.jsx';
import { useCustomerAuth } from '../contexts/CustomerAuthContext.jsx';
import { useSettings } from '../../shared/contexts/SettingsContext.jsx';
import useFetch from '../../shared/hooks/useFetch.js';
import useDocumentTitle from '../hooks/useDocumentTitle.js';
import customerRoomService from '../services/customerRoomService.js';
import customerBookingService, { PAYMENT_METHODS, SPECIAL_REQUEST_MAX, validateGuest } from '../services/customerBookingService.js';
import { CANCEL_RULES } from '../data/bookingPolicy.js';
import { CUSTOMER_ROUTES } from '../routes/paths.js';
import { NATIONALITIES } from '../../shared/utils/constants.js';
import { countNights, queryToSearch, searchToQuery, validateSearch } from '../utils/bookingSearch.js';
import { formatCurrency, formatDate } from '../../shared/utils/formatters.js';

const PAY_OPTIONS = [
  { key: 'hotel', icon: Banknote, hint: 'Thanh toán khi nhận phòng tại quầy lễ tân.' },
  { key: 'transfer', icon: Landmark, hint: 'Khách sạn sẽ liên hệ gửi thông tin chuyển khoản sau khi xác nhận đặt phòng.' },
  { key: 'card', icon: CreditCard, hint: 'Cổng thanh toán thẻ chưa được kết nối nên chưa thể chọn.' },
];

function ReviewBlock({ title, onEdit, children }) {
  return (
    <section className="rounded-2xl border border-slate-100 p-5">
      <div className="flex items-center justify-between">
        <h3 className="font-bold text-brand">{title}</h3>
        <button type="button" onClick={onEdit} className="inline-flex items-center gap-1 text-sm font-semibold text-accent hover:underline"><Pencil size={14} /> Sửa</button>
      </div>
      <div className="mt-3 text-sm text-slate-700">{children}</div>
    </section>
  );
}

const Kv = ({ k, v }) => (
  <div className="flex justify-between gap-4 py-1"><dt className="text-muted">{k}</dt><dd className="text-right font-medium text-ink">{v || '—'}</dd></div>
);

export default function Booking() {
  const { roomId } = useParams();
  const { account, customer } = useCustomerAuth();
  const { hotel } = useSettings();
  const [params, setParams] = useSearchParams();
  const search = useMemo(() => queryToSearch(params), [params]);
  const searchKey = searchToQuery(search);

  const load = useCallback(() => customerRoomService.getRoomType(roomId, search), [roomId, searchKey]); // eslint-disable-line react-hooks/exhaustive-deps
  const { data, loading, error, reload } = useFetch(load);
  const room = data?.room;
  useDocumentTitle(room ? `Đặt ${room.name}` : 'Đặt phòng');

  const [step, setStep] = useState(1);
  const [guest, setGuest] = useState(() => ({
    fullName: customer?.fullName || '',
    email: account?.email || '',
    phone: customer?.phone || '',
    nationality: customer?.nationality || 'Việt Nam',
    request: '',
  }));
  const [guestErrors, setGuestErrors] = useState({});
  const [agree, setAgree] = useState(false);
  const [agreeError, setAgreeError] = useState('');
  const [payment, setPayment] = useState('hotel');
  const [submitting, setSubmitting] = useState(false);
  const [outcome, setOutcome] = useState(null); // { ok, result? , message? }

  const onSearch = (values) =>
    setParams((prev) => {
      const next = new URLSearchParams(prev);
      new URLSearchParams(searchToQuery(values)).forEach((v, k) => next.set(k, v));
      return next;
    }, { replace: true });

  const back = `${CUSTOMER_ROUTES.ROOMS}?${searchKey}`;

  if (loading && !room) {
    return <div className="c-container py-14" role="status" aria-label="Đang tải"><div className="h-10 w-1/2 animate-pulse rounded bg-slate-200" /><div className="mt-8 h-96 animate-pulse rounded-3xl bg-slate-100" /></div>;
  }
  if (error) {
    return (
      <div className="c-container py-14">
        {error.includes('Không tìm thấy')
          ? <EmptyBlock title="Không tìm thấy hạng phòng" description="Hãy chọn lại hạng phòng từ danh sách." action={<Link to={CUSTOMER_ROUTES.ROOMS} className="c-btn-primary">Xem phòng nghỉ</Link>} />
          : <ErrorBlock message={error} onRetry={reload} />}
      </div>
    );
  }
  if (!room) return null;

  const searchErrors = validateSearch(search);
  const searchOk = Object.keys(searchErrors).length === 0;
  const canContinueRoom = searchOk && room.available;
  const nights = countNights(search.checkIn, search.checkOut);

  const setField = (key) => (e) => {
    setGuest((g) => ({ ...g, [key]: e.target.value }));
    setGuestErrors((er) => ({ ...er, [key]: undefined }));
  };

  const goGuestNext = () => {
    const found = validateGuest(guest);
    setGuestErrors(found);
    if (Object.keys(found).length === 0) setStep(3);
  };

  const goReviewNext = () => {
    if (!agree) {
      setAgreeError('Vui lòng xác nhận bạn đã đọc chính sách hủy phòng');
      return;
    }
    setAgreeError('');
    setStep(4);
  };

  const confirm = async () => {
    if (submitting) return; // chống bấm đúp
    setSubmitting(true);
    try {
      const result = await customerBookingService.createBooking({ customer, guest, roomType: room, search, paymentMethod: payment });
      setOutcome({ ok: true, result });
      setStep(5);
    } catch (e) {
      setOutcome({ ok: false, message: e.message || 'Không thể hoàn tất đặt phòng.' });
      setStep(5);
    } finally {
      setSubmitting(false);
    }
  };

  const heading = (
    <div className="mb-8">
      <h1 className="text-3xl font-extrabold tracking-tight text-brand sm:text-4xl">Đặt phòng</h1>
      <p className="mt-2 text-muted">Hoàn tất trong vài bước, bạn có thể xem lại mọi thông tin trước khi xác nhận.</p>
    </div>
  );

  /* ---------------- Bước 5: kết quả ---------------- */
  if (step === 5 && outcome) {
    return (
      <div className="c-container max-w-3xl py-12">
        {outcome.ok ? (
          <div className="rounded-[2rem] border border-slate-100 bg-white p-8 text-center shadow-lift sm:p-12">
            <span className="mx-auto flex h-20 w-20 items-center justify-center rounded-full bg-green-50 text-success"><CheckCircle2 size={44} /></span>
            <h1 className="mt-6 text-3xl font-extrabold text-brand">Đã gửi yêu cầu đặt phòng</h1>
            <p className="mx-auto mt-3 max-w-md text-muted">Phòng của bạn đã được giữ. Trạng thái hiện tại là <strong className="text-ink">Chờ xác nhận</strong>; lễ tân sẽ xác nhận với bạn qua email hoặc điện thoại.</p>

            <div className="mx-auto mt-8 max-w-md rounded-2xl bg-surface p-5 text-left">
              <p className="text-sm text-muted">Mã đặt phòng</p>
              <p className="mt-1 flex flex-wrap gap-2">
                {outcome.result.bookings.map((b) => (
                  <span key={b.id} className="rounded-lg bg-white px-3 py-1.5 font-mono text-base font-bold text-brand shadow-soft">{b.id}</span>
                ))}
              </p>
              <dl className="mt-4 divide-y divide-slate-200 text-sm">
                <Kv k="Hạng phòng" v={`${room.name} × ${search.rooms}`} />
                <Kv k="Nhận phòng" v={`${formatDate(search.checkIn)} từ ${hotel.checkInTime}`} />
                <Kv k="Trả phòng" v={`${formatDate(search.checkOut)} trước ${hotel.checkOutTime}`} />
                <Kv k="Thanh toán" v={PAYMENT_METHODS[payment].label} />
                <Kv k="Tổng thanh toán" v={formatCurrency(outcome.result.pricing.total)} />
              </dl>
              <p className="mt-3 text-xs text-muted">Chưa có khoản thanh toán nào được thực hiện. Trạng thái thanh toán: chưa thanh toán.</p>
            </div>

            <div className="mt-8 flex flex-wrap justify-center gap-3">
              <Link to={CUSTOMER_ROUTES.MY_BOOKINGS} className="c-btn-primary c-btn-lg">Xem đặt phòng của tôi</Link>
              <Link to={CUSTOMER_ROUTES.HOME} className="c-btn-outline c-btn-lg">Về trang chủ</Link>
            </div>
          </div>
        ) : (
          <div className="rounded-[2rem] border border-red-100 bg-white p-8 text-center shadow-lift sm:p-12" role="alert">
            <span className="mx-auto flex h-20 w-20 items-center justify-center rounded-full bg-red-50 text-danger"><XCircle size={44} /></span>
            <h1 className="mt-6 text-3xl font-extrabold text-brand">Chưa thể đặt phòng</h1>
            <p className="mx-auto mt-3 max-w-md text-muted">{outcome.message}</p>
            <p className="mt-2 text-sm text-muted">Không có phòng nào bị giữ và không có khoản thanh toán nào được thực hiện.</p>
            <div className="mt-8 flex flex-wrap justify-center gap-3">
              <button type="button" className="c-btn-primary c-btn-lg" onClick={() => { setOutcome(null); setStep(1); reload(); }}>Chọn lại ngày hoặc phòng</button>
              <Link to={back} className="c-btn-outline c-btn-lg">Xem hạng phòng khác</Link>
            </div>
          </div>
        )}
      </div>
    );
  }

  return (
    <div className="c-container py-10 pb-20">
      {heading}
      <div className="mb-10"><BookingStepper current={step} onGo={(n) => !submitting && setStep(n)} /></div>

      <div className="grid gap-10 lg:grid-cols-[1fr_24rem] lg:gap-12">
        <div>
          {/* ---- Bước 1 ---- */}
          {step === 1 && (
            <section aria-labelledby="step1">
              <h2 id="step1" className="text-2xl font-bold text-brand">Kiểm tra phòng và ngày ở</h2>
              <p className="mt-1 text-muted">Bạn đang đặt <strong className="text-ink">{room.name}</strong>. Có thể đổi ngày hoặc số khách tại đây.</p>
              <div className="mt-6"><SearchBooking key={searchKey} initial={search} onSearch={onSearch} submitLabel="Cập nhật" className="!shadow-soft" /></div>

              {searchOk && (
                <p role="status" className={`mt-5 flex items-start gap-2 rounded-2xl px-4 py-3 text-sm font-medium ${room.available ? 'bg-green-50 text-success' : 'bg-red-50 text-danger'}`}>
                  {room.available ? <CheckCircle2 size={18} className="mt-0.5 shrink-0" /> : <AlertCircle size={18} className="mt-0.5 shrink-0" />}
                  {room.available
                    ? `Còn phòng cho ${nights} đêm bạn chọn.`
                    : !room.fitsGuests
                      ? `Hạng phòng này chứa tối đa ${room.capacity} khách mỗi phòng. Hãy tăng số phòng hoặc chọn hạng khác.`
                      : 'Hạng phòng này không còn đủ phòng trong khoảng ngày bạn chọn.'}
                </p>
              )}
              <div className="mt-8 flex flex-wrap justify-between gap-3">
                <Link to={back} className="c-btn-ghost"><ArrowLeft size={16} /> Chọn hạng phòng khác</Link>
                <button type="button" className="c-btn-primary c-btn-lg" disabled={!canContinueRoom} onClick={() => setStep(2)}>Tiếp tục <ArrowRight size={18} /></button>
              </div>
            </section>
          )}

          {/* ---- Bước 2 ---- */}
          {step === 2 && (
            <section aria-labelledby="step2">
              <h2 id="step2" className="text-2xl font-bold text-brand">Thông tin khách hàng</h2>
              <p className="mt-1 text-muted">Thông tin người sẽ nhận phòng. Chúng tôi dùng email và số điện thoại để xác nhận đặt phòng.</p>
              <form noValidate onSubmit={(e) => { e.preventDefault(); goGuestNext(); }} className="mt-6 grid gap-5 sm:grid-cols-2">
                <TextField className="sm:col-span-2" label="Họ và tên" autoComplete="name" value={guest.fullName} onChange={setField('fullName')} error={guestErrors.fullName} required />
                <TextField label="Email" type="email" autoComplete="email" value={guest.email} onChange={setField('email')} error={guestErrors.email} required />
                <TextField label="Số điện thoại" type="tel" autoComplete="tel" value={guest.phone} onChange={setField('phone')} error={guestErrors.phone} placeholder="0912345678" required />
                <TextField as="select" className="sm:col-span-2" label="Quốc tịch" value={guest.nationality} onChange={setField('nationality')} error={guestErrors.nationality} required>
                  {NATIONALITIES.map((n) => <option key={n} value={n}>{n}</option>)}
                </TextField>
                <TextField as="textarea" rows={4} className="sm:col-span-2" label="Yêu cầu đặc biệt" value={guest.request} onChange={setField('request')} error={guestErrors.request} maxLength={SPECIAL_REQUEST_MAX + 50} hint={`Ví dụ: tầng cao, giường phụ, đến muộn. Đã nhập ${guest.request.length}/${SPECIAL_REQUEST_MAX} ký tự. Yêu cầu được ghi nhận, không đảm bảo luôn đáp ứng được.`} />
                <div className="mt-2 flex flex-wrap justify-between gap-3 sm:col-span-2">
                  <button type="button" className="c-btn-ghost" onClick={() => setStep(1)}><ArrowLeft size={16} /> Quay lại</button>
                  <button type="submit" className="c-btn-primary c-btn-lg">Tiếp tục <ArrowRight size={18} /></button>
                </div>
              </form>
            </section>
          )}

          {/* ---- Bước 3 ---- */}
          {step === 3 && (
            <section aria-labelledby="step3">
              <h2 id="step3" className="text-2xl font-bold text-brand">Kiểm tra thông tin đặt phòng</h2>
              <div className="mt-6 space-y-4">
                <ReviewBlock title="Phòng và thời gian" onEdit={() => setStep(1)}>
                  <dl className="divide-y divide-slate-100">
                    <Kv k="Hạng phòng" v={`${room.name} × ${search.rooms}`} />
                    <Kv k="Nhận phòng" v={`${formatDate(search.checkIn)} từ ${hotel.checkInTime}`} />
                    <Kv k="Trả phòng" v={`${formatDate(search.checkOut)} trước ${hotel.checkOutTime}`} />
                    <Kv k="Số đêm" v={`${nights} đêm`} />
                    <Kv k="Số khách" v={`${search.adults} người lớn${search.children ? `, ${search.children} trẻ em` : ''}`} />
                  </dl>
                </ReviewBlock>
                <ReviewBlock title="Thông tin khách hàng" onEdit={() => setStep(2)}>
                  <dl className="divide-y divide-slate-100">
                    <Kv k="Họ và tên" v={guest.fullName} />
                    <Kv k="Email" v={guest.email} />
                    <Kv k="Số điện thoại" v={guest.phone} />
                    <Kv k="Quốc tịch" v={guest.nationality} />
                    <Kv k="Yêu cầu đặc biệt" v={guest.request} />
                  </dl>
                </ReviewBlock>
                <div className="rounded-2xl bg-surface p-5 text-sm text-slate-600">
                  <p className="font-bold text-brand">Chính sách hủy phòng</p>
                  <ul className="mt-2 list-disc space-y-1 pl-5">{CANCEL_RULES.map((r) => <li key={r}>{r}</li>)}</ul>
                  <label className="mt-4 flex cursor-pointer items-start gap-2.5 font-medium text-slate-800">
                    <input type="checkbox" checked={agree} onChange={(e) => { setAgree(e.target.checked); setAgreeError(''); }} aria-invalid={Boolean(agreeError)} className="mt-0.5 h-4 w-4 rounded border-slate-300 text-accent focus:ring-accent" />
                    Tôi đã đọc và đồng ý với chính sách hủy phòng.
                  </label>
                  {agreeError && <p role="alert" className="mt-2 text-danger">{agreeError}</p>}
                </div>
              </div>
              <div className="mt-8 flex flex-wrap justify-between gap-3">
                <button type="button" className="c-btn-ghost" onClick={() => setStep(2)}><ArrowLeft size={16} /> Quay lại</button>
                <button type="button" className="c-btn-primary c-btn-lg" onClick={goReviewNext}>Tiếp tục <ArrowRight size={18} /></button>
              </div>
            </section>
          )}

          {/* ---- Bước 4 ---- */}
          {step === 4 && (
            <section aria-labelledby="step4">
              <h2 id="step4" className="text-2xl font-bold text-brand">Phương thức thanh toán</h2>
              <p className="mt-1 text-muted">Bạn chưa phải trả tiền ngay lúc này. Chọn cách bạn dự định thanh toán.</p>
              <fieldset className="mt-6 space-y-3">
                <legend className="sr-only">Phương thức thanh toán</legend>
                {PAY_OPTIONS.map(({ key, icon: Icon, hint }) => {
                  const m = PAYMENT_METHODS[key];
                  const checked = payment === key;
                  return (
                    <label key={key} className={`flex items-start gap-4 rounded-2xl border p-5 transition ${!m.available ? 'cursor-not-allowed border-slate-200 bg-slate-50 opacity-70' : checked ? 'cursor-pointer border-accent bg-accent-soft ring-2 ring-accent/20' : 'cursor-pointer border-slate-200 hover:border-accent'}`}>
                      <input type="radio" name="payment" value={key} checked={checked} disabled={!m.available} onChange={() => setPayment(key)} className="mt-1 h-4 w-4 text-accent focus:ring-accent" />
                      <span className={`flex h-11 w-11 shrink-0 items-center justify-center rounded-xl ${checked ? 'bg-accent text-white' : 'bg-surface text-accent'}`}><Icon size={22} /></span>
                      <span>
                        <span className="block font-bold text-brand">{m.label}{!m.available && <span className="ml-2 rounded-full bg-slate-200 px-2 py-0.5 text-xs font-semibold text-slate-600">Chưa khả dụng</span>}</span>
                        <span className="mt-1 block text-sm text-muted">{hint}</span>
                      </span>
                    </label>
                  );
                })}
              </fieldset>
              <div className="mt-8 flex flex-wrap justify-between gap-3">
                <button type="button" className="c-btn-ghost" onClick={() => setStep(3)} disabled={submitting}><ArrowLeft size={16} /> Quay lại</button>
                <button type="button" className="c-btn-primary c-btn-lg" onClick={confirm} disabled={submitting}>
                  {submitting ? <><Loader2 size={18} className="animate-spin" /> Đang gửi…</> : <>Xác nhận đặt phòng</>}
                </button>
              </div>
            </section>
          )}
        </div>

        <div className="lg:sticky lg:top-24 lg:self-start"><PriceSummary room={room} search={search} /></div>
      </div>
    </div>
  );
}
