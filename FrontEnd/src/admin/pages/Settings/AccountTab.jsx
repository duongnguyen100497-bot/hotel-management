import { useState } from 'react';
import { Loader2, Save } from 'lucide-react';
import Avatar from '../../components/common/Avatar.jsx';
import { Input } from '../../components/common/FormField.jsx';
import useForm from '../../hooks/useForm.js';
import { useAuth } from '../../contexts/AuthContext.jsx';
import { useToast } from '../../../shared/contexts/ToastContext.jsx';
import authService from '../../services/authService.js';
import { ROLES } from '../../utils/permissions.js';
import { firstError, isEmail, minLength, required } from '../../../shared/utils/validators.js';

const validate = (v) => ({
  name: firstError(required(v.name, 'Họ và tên'), minLength(v.name.trim(), 2, 'Họ và tên')),
  email: firstError(required(v.email, 'Email'), isEmail(v.email)),
});

export default function AccountTab() {
  const toast = useToast();
  const { user, updateUser } = useAuth();
  const [saving, setSaving] = useState(false);
  const { values, errors, handleChange, handleSubmit } = useForm({ name: user.name, email: user.email }, validate);

  const submit = async (v) => {
    setSaving(true);
    try {
      const updated = await authService.updateProfile(user.id, { name: v.name.trim(), email: v.email.trim() });
      updateUser({ name: updated.name, email: updated.email });
      toast.success('Đã cập nhật thông tin tài khoản');
    } catch (e) {
      toast.error(e.message);
    } finally {
      setSaving(false);
    }
  };

  return (
    <form onSubmit={handleSubmit(submit)} noValidate className="card p-5 sm:p-6">
      <h2 className="text-base font-semibold">Cấu hình tài khoản</h2>
      <p className="mb-5 text-sm text-slate-500">Thông tin cá nhân của người đang đăng nhập.</p>

      <div className="mb-6 flex items-center gap-4">
        <Avatar name={user.name} size={64} />
        <div>
          <p className="font-semibold text-navy-900">{user.name}</p>
          <p className="text-sm text-slate-500">
            @{user.username} · {ROLES[user.role]}
          </p>
        </div>
      </div>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <Input label="Họ và tên" name="name" value={values.name} onChange={handleChange} error={errors.name} required />
        <Input label="Email" name="email" type="email" value={values.email} onChange={handleChange} error={errors.email} required />
        <Input label="Tên đăng nhập" value={user.username} disabled readOnly hint="Không thể thay đổi" />
        <Input label="Vai trò" value={ROLES[user.role] || user.role} disabled readOnly hint="Do quản trị viên phân quyền" />
      </div>

      <div className="mt-6 flex justify-end">
        <button type="submit" className="btn-primary" disabled={saving}>
          {saving ? <Loader2 size={16} className="animate-spin" /> : <Save size={16} />}
          Lưu thay đổi
        </button>
      </div>
    </form>
  );
}
