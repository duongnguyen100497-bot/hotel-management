import { useState } from 'react';
import { Eye, EyeOff, KeyRound, Loader2 } from 'lucide-react';
import { Input } from '../../components/common/FormField.jsx';
import useForm from '../../hooks/useForm.js';
import { useAuth } from '../../contexts/AuthContext.jsx';
import { useToast } from '../../../shared/contexts/ToastContext.jsx';
import authService from '../../services/authService.js';
import { firstError, minLength, required } from '../../../shared/utils/validators.js';

const EMPTY = { current: '', next: '', confirm: '' };

const validate = (v) => ({
  current: required(v.current, 'Mật khẩu hiện tại'),
  next: firstError(
    required(v.next, 'Mật khẩu mới'),
    minLength(v.next, 6, 'Mật khẩu mới'),
    v.next && v.next === v.current ? 'Mật khẩu mới phải khác mật khẩu hiện tại' : '',
  ),
  confirm: firstError(
    required(v.confirm, 'Xác nhận mật khẩu'),
    v.confirm && v.confirm !== v.next ? 'Xác nhận mật khẩu không khớp' : '',
  ),
});

export default function PasswordTab() {
  const toast = useToast();
  const { user } = useAuth();
  const [saving, setSaving] = useState(false);
  const [show, setShow] = useState(false);
  const { values, errors, setErrors, handleChange, handleSubmit, reset } = useForm(EMPTY, validate);

  const submit = async (v) => {
    setSaving(true);
    try {
      await authService.changePassword(user.id, v.current, v.next);
      toast.success('Đã đổi mật khẩu thành công');
      reset(EMPTY);
    } catch (e) {
      setErrors({ current: e.message });
      toast.error(e.message);
    } finally {
      setSaving(false);
    }
  };

  const type = show ? 'text' : 'password';

  return (
    <form onSubmit={handleSubmit(submit)} noValidate className="card max-w-xl p-5 sm:p-6">
      <h2 className="text-base font-semibold">Đổi mật khẩu</h2>
      <p className="mb-5 text-sm text-slate-500">Mật khẩu mới cần tối thiểu 6 ký tự.</p>

      <div className="space-y-4">
        <Input label="Mật khẩu hiện tại" name="current" type={type} value={values.current} onChange={handleChange} error={errors.current} autoComplete="current-password" required />
        <Input label="Mật khẩu mới" name="next" type={type} value={values.next} onChange={handleChange} error={errors.next} autoComplete="new-password" required />
        <Input label="Xác nhận mật khẩu mới" name="confirm" type={type} value={values.confirm} onChange={handleChange} error={errors.confirm} autoComplete="new-password" required />
      </div>

      <div className="mt-6 flex items-center justify-between gap-3">
        <button type="button" className="btn-ghost" onClick={() => setShow((s) => !s)}>
          {show ? <EyeOff size={16} /> : <Eye size={16} />}
          {show ? 'Ẩn mật khẩu' : 'Hiện mật khẩu'}
        </button>
        <button type="submit" className="btn-primary" disabled={saving}>
          {saving ? <Loader2 size={16} className="animate-spin" /> : <KeyRound size={16} />}
          Đổi mật khẩu
        </button>
      </div>
    </form>
  );
}
