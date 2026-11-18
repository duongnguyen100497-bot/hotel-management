import { useState } from 'react';
import { Link, Navigate, useLocation, useNavigate } from 'react-router-dom';
import { Loader2 } from 'lucide-react';
import AuthShell from '../components/AuthShell.jsx';
import { FormAlert, PasswordMeter, TextField } from '../components/FormControls.jsx';
import { useCustomerAuth } from '../contexts/CustomerAuthContext.jsx';
import useDocumentTitle from '../hooks/useDocumentTitle.js';
import { safeReturnPath } from '../routes/RequireCustomerAuth.jsx';
import { CUSTOMER_ROUTES } from '../routes/paths.js';
import { validateRegister } from '../services/customerAuthService.js';
import { passwordStrength } from '../utils/password.js';

export default function Register() {
  useDocumentTitle('Đăng ký');
  const { register, isAuthenticated } = useCustomerAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const target = safeReturnPath(location.state?.from);

  const [values, setValues] = useState({ fullName: '', email: '', phone: '', password: '', confirmPassword: '', agree: false });
  const [errors, setErrors] = useState({});
  const [formError, setFormError] = useState('');
  const [loading, setLoading] = useState(false);

  if (isAuthenticated && !loading) return <Navigate to={target} replace />;

  const set = (key) => (e) => {
    setValues((v) => ({ ...v, [key]: e.target.type === 'checkbox' ? e.target.checked : e.target.value }));
    setErrors((er) => ({ ...er, [key]: undefined }));
    setFormError('');
  };

  const submit = async (e) => {
    e.preventDefault();
    const found = validateRegister(values);
    setErrors(found);
    if (Object.keys(found).length) return;
    setLoading(true);
    setFormError('');
    try {
      await register(values);
      navigate(target, { replace: true });
    } catch (err) {
      if (err.fields) setErrors(err.fields);
      if (!err.fields || err.fields.email) setFormError(err.message || 'Không thể đăng ký. Vui lòng thử lại.');
      setLoading(false);
    }
  };

  return (
    <AuthShell
      title="Tạo tài khoản"
      subtitle="Chỉ mất một phút. Bạn sẽ đặt phòng nhanh hơn và theo dõi được mọi đặt phòng."
      footer={<>Đã có tài khoản? <Link to={CUSTOMER_ROUTES.LOGIN} state={location.state} className="font-semibold text-accent hover:underline">Đăng nhập</Link></>}
    >
      <form onSubmit={submit} noValidate className="space-y-5">
        <FormAlert>{formError}</FormAlert>
        <TextField label="Họ và tên" autoComplete="name" value={values.fullName} onChange={set('fullName')} error={errors.fullName} required />
        <div className="grid gap-5 sm:grid-cols-2">
          <TextField label="Email" type="email" autoComplete="email" value={values.email} onChange={set('email')} error={errors.email} required />
          <TextField label="Số điện thoại" type="tel" autoComplete="tel" value={values.phone} onChange={set('phone')} error={errors.phone} placeholder="0912345678" required />
        </div>
        <div>
          <TextField label="Mật khẩu" type="password" autoComplete="new-password" value={values.password} onChange={set('password')} error={errors.password} hint="Tối thiểu 8 ký tự, gồm chữ và số." required />
          <PasswordMeter score={passwordStrength(values.password)} />
        </div>
        <TextField label="Nhập lại mật khẩu" type="password" autoComplete="new-password" value={values.confirmPassword} onChange={set('confirmPassword')} error={errors.confirmPassword} required />
        <div>
          <label className="flex cursor-pointer items-start gap-2.5 text-sm text-slate-700">
            <input type="checkbox" checked={values.agree} onChange={set('agree')} aria-invalid={Boolean(errors.agree)} className="mt-0.5 h-4 w-4 rounded border-slate-300 text-accent focus:ring-accent" />
            <span>Tôi đồng ý với điều khoản sử dụng và chính sách bảo mật của khách sạn.</span>
          </label>
          {errors.agree && <p role="alert" className="mt-1.5 text-sm text-danger">{errors.agree}</p>}
        </div>
        <button type="submit" className="c-btn-primary c-btn-lg w-full" disabled={loading}>
          {loading && <Loader2 size={18} className="animate-spin" />} Tạo tài khoản
        </button>
      </form>
    </AuthShell>
  );
}
