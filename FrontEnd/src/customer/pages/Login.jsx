import { useState } from 'react';
import { Link, Navigate, useLocation, useNavigate } from 'react-router-dom';
import { Loader2 } from 'lucide-react';
import AuthShell from '../components/AuthShell.jsx';
import { FormAlert, TextField } from '../components/FormControls.jsx';
import Modal from '../components/Modal.jsx';
import { useCustomerAuth } from '../contexts/CustomerAuthContext.jsx';
import { useSettings } from '../../shared/contexts/SettingsContext.jsx';
import useDocumentTitle from '../hooks/useDocumentTitle.js';
import { safeReturnPath } from '../routes/RequireCustomerAuth.jsx';
import { CUSTOMER_ROUTES } from '../routes/paths.js';
import { DEMO_ACCOUNT, validateLogin } from '../services/customerAuthService.js';

export default function Login() {
  useDocumentTitle('Đăng nhập');
  const { login, isAuthenticated } = useCustomerAuth();
  const { hotel } = useSettings();
  const navigate = useNavigate();
  const location = useLocation();
  const target = safeReturnPath(location.state?.from);

  const [values, setValues] = useState({ email: '', password: '', remember: true });
  const [errors, setErrors] = useState({});
  const [formError, setFormError] = useState('');
  const [loading, setLoading] = useState(false);
  const [forgotOpen, setForgotOpen] = useState(false);

  if (isAuthenticated && !loading) return <Navigate to={target} replace />;

  const set = (key) => (e) => {
    setValues((v) => ({ ...v, [key]: e.target.type === 'checkbox' ? e.target.checked : e.target.value }));
    setErrors((er) => ({ ...er, [key]: undefined }));
    setFormError('');
  };

  const submit = async (e) => {
    e.preventDefault();
    const found = validateLogin(values);
    setErrors(found);
    if (Object.keys(found).length) return;
    setLoading(true);
    setFormError('');
    try {
      await login(values);
      navigate(target, { replace: true });
    } catch (err) {
      if (err.fields) setErrors(err.fields);
      else setFormError(err.message || 'Không thể đăng nhập. Vui lòng thử lại.');
      setLoading(false);
    }
  };

  return (
    <AuthShell
      title="Chào mừng bạn trở lại"
      subtitle={location.state?.from ? 'Vui lòng đăng nhập để tiếp tục.' : `Đăng nhập để quản lý đặt phòng tại ${hotel.name}.`}
      footer={<>Chưa có tài khoản? <Link to={CUSTOMER_ROUTES.REGISTER} state={location.state} className="font-semibold text-accent hover:underline">Đăng ký miễn phí</Link></>}
    >
      <form onSubmit={submit} noValidate className="space-y-5">
        <FormAlert>{formError}</FormAlert>
        <TextField label="Email" type="email" autoComplete="email" value={values.email} onChange={set('email')} error={errors.email} placeholder="ten@email.com" required />
        <TextField label="Mật khẩu" type="password" autoComplete="current-password" value={values.password} onChange={set('password')} error={errors.password} required />
        <div className="flex items-center justify-between gap-3">
          <label className="flex cursor-pointer items-center gap-2 text-sm text-slate-700">
            <input type="checkbox" checked={values.remember} onChange={set('remember')} className="h-4 w-4 rounded border-slate-300 text-accent focus:ring-accent" />
            Ghi nhớ đăng nhập
          </label>
          <button type="button" onClick={() => setForgotOpen(true)} className="text-sm font-semibold text-accent hover:underline">Quên mật khẩu?</button>
        </div>
        <button type="submit" className="c-btn-primary c-btn-lg w-full" disabled={loading}>
          {loading && <Loader2 size={18} className="animate-spin" />} Đăng nhập
        </button>
      </form>

      <div className="mt-6 rounded-2xl bg-accent-soft px-4 py-3 text-sm text-slate-700">
        <p className="font-semibold text-brand">Tài khoản dùng thử</p>
        <p className="mt-1">
          Email <button type="button" className="font-mono text-accent underline" onClick={() => setValues((v) => ({ ...v, email: DEMO_ACCOUNT.email, password: DEMO_ACCOUNT.password }))}>{DEMO_ACCOUNT.email}</button>, mật khẩu <span className="font-mono">{DEMO_ACCOUNT.password}</span>. Bấm vào email để điền sẵn.
        </p>
      </div>

      <Modal open={forgotOpen} onClose={() => setForgotOpen(false)} title="Quên mật khẩu" size="sm" footer={<button type="button" className="c-btn-primary" onClick={() => setForgotOpen(false)}>Đã hiểu</button>}>
        <p className="text-sm leading-relaxed text-slate-600">
          Chức năng đặt lại mật khẩu qua email cần máy chủ gửi thư, hiện chưa được kết nối. Vui lòng liên hệ khách sạn để được hỗ trợ:
        </p>
        <p className="mt-3 text-sm font-semibold text-brand">Hotline {hotel.phone}</p>
        <p className="text-sm text-slate-600">{hotel.email}</p>
      </Modal>
    </AuthShell>
  );
}
