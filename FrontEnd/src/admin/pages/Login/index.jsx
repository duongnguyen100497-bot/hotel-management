import { useState } from 'react';
import { Navigate, useLocation, useNavigate } from 'react-router-dom';
import { CalendarCheck, Eye, EyeOff, Loader2, Lock, ShieldCheck, User, BarChart3 } from 'lucide-react';
import Logo from '../../components/common/Logo.jsx';
import Modal from '../../components/common/Modal.jsx';
import { Input } from '../../components/common/FormField.jsx';
import { useAuth } from '../../contexts/AuthContext.jsx';
import { useSettings } from '../../../shared/contexts/SettingsContext.jsx';
import { useToast } from '../../../shared/contexts/ToastContext.jsx';
import useForm from '../../hooks/useForm.js';
import authService from '../../services/authService.js';
import { ROUTES } from '../../routes/paths.js';
import { firstError, isEmail, minLength, required } from '../../../shared/utils/validators.js';

const DEMO_ACCOUNTS = [
  { label: 'Quản trị viên', username: 'admin', password: 'admin123' },
  { label: 'Quản lý', username: 'manager', password: 'manager123' },
  { label: 'Lễ tân', username: 'letan', password: 'letan123' },
  { label: 'Kế toán', username: 'ketoan', password: 'ketoan123' },
];

const FEATURES = [
  { icon: CalendarCheck, text: 'Quản lý đặt phòng và nhận / trả phòng nhanh chóng' },
  { icon: BarChart3, text: 'Báo cáo doanh thu và công suất phòng trực quan' },
  { icon: ShieldCheck, text: 'Phân quyền rõ ràng theo vai trò nhân viên' },
];

const validateLogin = (v) => ({
  username: required(v.username, 'Tên đăng nhập / Email'),
  password: firstError(required(v.password, 'Mật khẩu'), minLength(v.password, 6, 'Mật khẩu')),
});

function ForgotPasswordModal({ open, onClose }) {
  const toast = useToast();
  const [sending, setSending] = useState(false);
  const { values, errors, handleChange, handleSubmit, reset } = useForm({ email: '' }, (v) => ({
    email: firstError(required(v.email, 'Email'), isEmail(v.email)),
  }));

  const close = () => {
    reset({ email: '' });
    onClose();
  };

  const submit = async ({ email }) => {
    setSending(true);
    try {
      await authService.requestPasswordReset(email);
      toast.success('Nếu email tồn tại, hướng dẫn đặt lại mật khẩu đã được gửi.');
      close();
    } finally {
      setSending(false);
    }
  };

  return (
    <Modal
      open={open}
      onClose={close}
      title="Quên mật khẩu"
      subtitle="Nhập email tài khoản để nhận hướng dẫn đặt lại mật khẩu."
      size="sm"
      footer={
        <>
          <button type="button" className="btn-secondary" onClick={close}>
            Hủy bỏ
          </button>
          <button type="submit" form="forgot-form" className="btn-primary" disabled={sending}>
            {sending && <Loader2 size={16} className="animate-spin" />} Gửi yêu cầu
          </button>
        </>
      }
    >
      <form id="forgot-form" onSubmit={handleSubmit(submit)} noValidate>
        <Input
          label="Email"
          name="email"
          type="email"
          placeholder="ban@khachsan.vn"
          value={values.email}
          onChange={handleChange}
          error={errors.email}
          required
        />
      </form>
    </Modal>
  );
}

export default function Login() {
  const { isAuthenticated, login } = useAuth();
  const { hotel } = useSettings();
  const navigate = useNavigate();
  const location = useLocation();
  const [showPassword, setShowPassword] = useState(false);
  const [remember, setRemember] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [serverError, setServerError] = useState('');
  const [forgotOpen, setForgotOpen] = useState(false);

  const { values, errors, setValues, handleChange, handleSubmit } = useForm(
    { username: '', password: '' },
    validateLogin,
  );

  if (isAuthenticated) return <Navigate to={location.state?.from || ROUTES.DASHBOARD} replace />;

  const submit = async ({ username, password }) => {
    setSubmitting(true);
    setServerError('');
    try {
      await login(username, password, remember);
      navigate(location.state?.from || ROUTES.DASHBOARD, { replace: true });
    } catch (e) {
      setServerError(e.message);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="grid min-h-screen lg:grid-cols-2">
      {/* Cột thương hiệu */}
      <div className="relative hidden flex-col justify-between overflow-hidden bg-gradient-to-br from-navy-950 via-navy-900 to-navy-700 p-12 text-white lg:flex">
        <div className="pointer-events-none absolute -right-24 -top-24 h-96 w-96 rounded-full bg-sky-400/10 blur-3xl" />
        <div className="pointer-events-none absolute -bottom-32 -left-20 h-96 w-96 rounded-full bg-sky-300/10 blur-3xl" />
        <Logo size={48} dark />
        <div className="relative max-w-md">
          <h1 className="text-4xl font-semibold leading-tight !text-white">
            Vận hành khách sạn
            <br />
            chuyên nghiệp, tinh gọn.
          </h1>
          <p className="mt-4 text-sky-200/80">
            Hệ thống quản lý khách sạn (PMS) giúp bạn kiểm soát phòng, đặt phòng, khách hàng và doanh thu trên một nền tảng duy nhất.
          </p>
          <ul className="mt-8 space-y-4">
            {FEATURES.map(({ icon: Icon, text }) => (
              <li key={text} className="flex items-center gap-3 text-sm text-sky-100">
                <span className="flex h-9 w-9 items-center justify-center rounded-lg bg-white/10 text-sky-300">
                  <Icon size={18} />
                </span>
                {text}
              </li>
            ))}
          </ul>
        </div>
        <p className="relative text-xs text-sky-200/60">© {new Date().getFullYear()} {hotel.name}. Bảo lưu mọi quyền.</p>
      </div>

      {/* Form đăng nhập */}
      <div className="flex items-center justify-center bg-white p-6 sm:p-10">
        <div className="w-full max-w-md animate-slide-up">
          <Logo size={44} className="mb-8 lg:hidden" />
          <h2 className="text-2xl font-semibold">Chào mừng trở lại</h2>
          <p className="mt-1 text-sm text-slate-500">Đăng nhập để tiếp tục quản lý khách sạn.</p>

          {serverError && (
            <div role="alert" className="mt-5 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
              {serverError}
            </div>
          )}

          <form onSubmit={handleSubmit(submit)} noValidate className="mt-6 space-y-4">
            <div className="relative">
              <Input
                label="Email / Tên đăng nhập"
                name="username"
                autoComplete="username"
                placeholder="admin"
                value={values.username}
                onChange={handleChange}
                error={errors.username}
                className="[&_input]:pl-9"
                required
              />
              <User size={16} className="pointer-events-none absolute left-3 top-[2.3rem] text-slate-400" />
            </div>

            <div className="relative">
              <Input
                label="Mật khẩu"
                name="password"
                type={showPassword ? 'text' : 'password'}
                autoComplete="current-password"
                placeholder="••••••••"
                value={values.password}
                onChange={handleChange}
                error={errors.password}
                className="[&_input]:pl-9 [&_input]:pr-10"
                required
              />
              <Lock size={16} className="pointer-events-none absolute left-3 top-[2.3rem] text-slate-400" />
              <button
                type="button"
                onClick={() => setShowPassword((s) => !s)}
                className="absolute right-3 top-[2.3rem] text-slate-400 hover:text-slate-600"
                aria-label={showPassword ? 'Ẩn mật khẩu' : 'Hiện mật khẩu'}
              >
                {showPassword ? <EyeOff size={17} /> : <Eye size={17} />}
              </button>
            </div>

            <div className="flex items-center justify-between text-sm">
              <label className="flex cursor-pointer items-center gap-2 text-slate-600">
                <input
                  type="checkbox"
                  checked={remember}
                  onChange={(e) => setRemember(e.target.checked)}
                  className="h-4 w-4 rounded border-slate-300 text-navy-700 focus:ring-sky-300"
                />
                Ghi nhớ đăng nhập
              </label>
              <button type="button" onClick={() => setForgotOpen(true)} className="font-medium text-navy-600 hover:text-navy-800 hover:underline">
                Quên mật khẩu?
              </button>
            </div>

            <button type="submit" disabled={submitting} className="btn-primary w-full py-2.5">
              {submitting && <Loader2 size={16} className="animate-spin" />}
              {submitting ? 'Đang đăng nhập...' : 'Đăng nhập'}
            </button>
          </form>

          <div className="mt-8 rounded-xl border border-sky-100 bg-sky-50/60 p-4">
            <p className="text-xs font-semibold uppercase tracking-wide text-navy-700">Tài khoản demo (bấm để điền)</p>
            <div className="mt-2 grid grid-cols-2 gap-2">
              {DEMO_ACCOUNTS.map((a) => (
                <button
                  key={a.username}
                  type="button"
                  onClick={() => setValues({ username: a.username, password: a.password })}
                  className="rounded-lg border border-sky-100 bg-white px-3 py-2 text-left text-xs transition-colors hover:border-navy-300 hover:bg-sky-50"
                >
                  <span className="block font-semibold text-navy-800">{a.label}</span>
                  <span className="text-slate-500">
                    {a.username} / {a.password}
                  </span>
                </button>
              ))}
            </div>
          </div>
        </div>
      </div>

      <ForgotPasswordModal open={forgotOpen} onClose={() => setForgotOpen(false)} />
    </div>
  );
}
