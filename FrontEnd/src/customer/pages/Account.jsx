import { useCallback, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { CalendarCheck, Loader2, Moon, Trophy } from 'lucide-react';
import AccountNav from '../components/AccountNav.jsx';
import StatusBadge from '../components/StatusBadge.jsx';
import { FormAlert, PasswordMeter, TextField } from '../components/FormControls.jsx';
import { ErrorBlock } from '../components/States.jsx';
import { useCustomerAuth } from '../contexts/CustomerAuthContext.jsx';
import { useToast } from '../../shared/contexts/ToastContext.jsx';
import useFetch from '../../shared/hooks/useFetch.js';
import useDocumentTitle from '../hooks/useDocumentTitle.js';
import customerBookingService from '../services/customerBookingService.js';
import { CUSTOMER_ROUTES } from '../routes/paths.js';
import { NATIONALITIES } from '../../shared/utils/constants.js';
import { countNights } from '../utils/bookingSearch.js';
import { passwordStrength } from '../utils/password.js';
import { formatCurrency, formatDate } from '../../shared/utils/formatters.js';
import { bookingGrandTotal } from '../utils/bookingRules.js';

const TABS = ['overview', 'profile', 'password'];
const TITLES = { overview: 'Tổng quan tài khoản', profile: 'Thông tin cá nhân', password: 'Đổi mật khẩu' };

function Stat({ icon: Icon, value, label }) {
  return (
    <div className="rounded-3xl border border-slate-100 bg-white p-5 shadow-soft">
      <span className="flex h-11 w-11 items-center justify-center rounded-2xl bg-accent-soft text-accent"><Icon size={22} /></span>
      <p className="mt-4 text-3xl font-extrabold text-brand">{value}</p>
      <p className="mt-0.5 text-sm text-muted">{label}</p>
    </div>
  );
}

function Overview() {
  const { customer, account } = useCustomerAuth();
  const load = useCallback(() => customerBookingService.listMine(customer.id), [customer.id]);
  const { data, loading, error, reload } = useFetch(load);
  if (loading) return <div role="status" aria-label="Đang tải" className="grid gap-4 sm:grid-cols-3">{[0, 1, 2].map((i) => <div key={i} className="h-36 animate-pulse rounded-3xl bg-slate-100" />)}</div>;
  if (error) return <ErrorBlock message={error} onRetry={reload} />;

  const bookings = data || [];
  const counted = bookings.filter((b) => b.status !== 'Cancelled');
  const upcoming = bookings.filter((b) => b.status === 'Pending' || b.status === 'Confirmed').sort((a, b) => a.checkIn.localeCompare(b.checkIn));
  const done = bookings.filter((b) => b.status === 'Checked-out');
  const stayedNights = done.reduce((s, b) => s + countNights(b.checkIn, b.checkOut), 0);
  const next = upcoming[0];

  return (
    <div className="space-y-8">
      <div className="grid gap-4 sm:grid-cols-3">
        <Stat icon={CalendarCheck} value={counted.length} label="Tổng số lần đặt phòng" />
        <Stat icon={Trophy} value={upcoming.length} label="Đặt phòng sắp tới" />
        <Stat icon={Moon} value={stayedNights} label="Đêm đã lưu trú" />
      </div>

      <section aria-labelledby="next-title">
        <h2 id="next-title" className="text-xl font-bold text-brand">Chuyến đi sắp tới</h2>
        {next ? (
          <div className="mt-4 flex flex-wrap items-center justify-between gap-4 rounded-3xl border border-slate-100 bg-white p-6 shadow-soft">
            <div>
              <p className="font-mono text-sm font-semibold text-accent">{next.id}</p>
              <p className="mt-1 text-lg font-bold text-brand">Phòng {next.roomType}</p>
              <p className="mt-1 text-sm text-muted">{formatDate(next.checkIn)} – {formatDate(next.checkOut)} ({countNights(next.checkIn, next.checkOut)} đêm), {formatCurrency(bookingGrandTotal(next))}</p>
            </div>
            <div className="flex items-center gap-3">
              <StatusBadge status={next.status} />
              <Link to={CUSTOMER_ROUTES.MY_BOOKINGS} className="c-btn-outline c-btn-sm">Xem chi tiết</Link>
            </div>
          </div>
        ) : (
          <div className="mt-4 rounded-3xl border border-dashed border-slate-300 p-8 text-center">
            <p className="font-semibold text-brand">Bạn chưa có chuyến đi nào sắp tới</p>
            <Link to={CUSTOMER_ROUTES.ROOMS} className="c-btn-primary mt-4">Tìm phòng</Link>
          </div>
        )}
      </section>

      <section aria-labelledby="info-title">
        <h2 id="info-title" className="text-xl font-bold text-brand">Thông tin tài khoản</h2>
        <dl className="mt-4 grid gap-x-8 gap-y-4 rounded-3xl bg-surface p-6 sm:grid-cols-2">
          {[['Họ và tên', customer.fullName], ['Email', account.email], ['Số điện thoại', customer.phone], ['Quốc tịch', customer.nationality], ['Địa chỉ', customer.address], ['Thành viên từ', formatDate(customer.createdAt)]].map(([k, v]) => (
            <div key={k}><dt className="text-xs text-muted">{k}</dt><dd className="mt-0.5 font-semibold text-ink">{v || 'Chưa cập nhật'}</dd></div>
          ))}
        </dl>
      </section>
    </div>
  );
}

function ProfileForm() {
  const { customer, account, updateProfile } = useCustomerAuth();
  const toast = useToast();
  const [values, setValues] = useState({ fullName: customer.fullName || '', phone: customer.phone || '', nationality: customer.nationality || 'Việt Nam', address: customer.address || '' });
  const [errors, setErrors] = useState({});
  const [formError, setFormError] = useState('');
  const [saving, setSaving] = useState(false);
  const set = (k) => (e) => { setValues((v) => ({ ...v, [k]: e.target.value })); setErrors((er) => ({ ...er, [k]: undefined })); };

  const submit = async (e) => {
    e.preventDefault();
    setSaving(true);
    setFormError('');
    try {
      await updateProfile(values);
      toast.success('Đã lưu thông tin cá nhân');
    } catch (err) {
      if (err.fields) setErrors(err.fields); else setFormError(err.message);
    } finally {
      setSaving(false);
    }
  };

  return (
    <form onSubmit={submit} noValidate className="grid max-w-2xl gap-5 sm:grid-cols-2">
      <div className="sm:col-span-2"><FormAlert>{formError}</FormAlert></div>
      <TextField className="sm:col-span-2" label="Họ và tên" autoComplete="name" value={values.fullName} onChange={set('fullName')} error={errors.fullName} required />
      <TextField label="Email" value={account.email} readOnly disabled hint="Email đăng nhập không thể thay đổi." />
      <TextField label="Số điện thoại" type="tel" autoComplete="tel" value={values.phone} onChange={set('phone')} error={errors.phone} required />
      <TextField as="select" label="Quốc tịch" value={values.nationality} onChange={set('nationality')}>
        {NATIONALITIES.map((n) => <option key={n} value={n}>{n}</option>)}
      </TextField>
      <TextField label="Địa chỉ" value={values.address} onChange={set('address')} autoComplete="street-address" />
      <div className="sm:col-span-2">
        <button type="submit" className="c-btn-primary c-btn-lg" disabled={saving}>{saving && <Loader2 size={18} className="animate-spin" />} Lưu thay đổi</button>
      </div>
    </form>
  );
}

function PasswordForm() {
  const { changePassword } = useCustomerAuth();
  const toast = useToast();
  const empty = { current: '', next: '', confirm: '' };
  const [values, setValues] = useState(empty);
  const [errors, setErrors] = useState({});
  const [formError, setFormError] = useState('');
  const [saving, setSaving] = useState(false);
  const set = (k) => (e) => { setValues((v) => ({ ...v, [k]: e.target.value })); setErrors((er) => ({ ...er, [k]: undefined })); };

  const submit = async (e) => {
    e.preventDefault();
    setSaving(true);
    setFormError('');
    try {
      await changePassword(values);
      setValues(empty);
      setErrors({});
      toast.success('Đã đổi mật khẩu');
    } catch (err) {
      if (err.fields) setErrors(err.fields); else setFormError(err.message);
    } finally {
      setSaving(false);
    }
  };

  return (
    <form onSubmit={submit} noValidate className="max-w-md space-y-5">
      <FormAlert>{formError}</FormAlert>
      <TextField label="Mật khẩu hiện tại" type="password" autoComplete="current-password" value={values.current} onChange={set('current')} error={errors.current} required />
      <div>
        <TextField label="Mật khẩu mới" type="password" autoComplete="new-password" value={values.next} onChange={set('next')} error={errors.next} hint="Tối thiểu 8 ký tự, gồm chữ và số." required />
        <PasswordMeter score={passwordStrength(values.next)} />
      </div>
      <TextField label="Nhập lại mật khẩu mới" type="password" autoComplete="new-password" value={values.confirm} onChange={set('confirm')} error={errors.confirm} required />
      <button type="submit" className="c-btn-primary c-btn-lg" disabled={saving}>{saving && <Loader2 size={18} className="animate-spin" />} Đổi mật khẩu</button>
    </form>
  );
}

export default function Account() {
  const [params] = useSearchParams();
  const tab = TABS.includes(params.get('tab')) ? params.get('tab') : 'overview';
  useDocumentTitle(TITLES[tab]);
  return (
    <div className="c-container grid gap-8 py-10 pb-20 lg:grid-cols-[17rem_1fr] lg:gap-12">
      <aside className="lg:sticky lg:top-24 lg:self-start"><AccountNav active={tab} /></aside>
      <section aria-labelledby="account-title" className="min-w-0">
        <h1 id="account-title" className="mb-6 text-3xl font-extrabold tracking-tight text-brand">{TITLES[tab]}</h1>
        {tab === 'overview' && <Overview />}
        {tab === 'profile' && <ProfileForm />}
        {tab === 'password' && <PasswordForm />}
      </section>
    </div>
  );
}
