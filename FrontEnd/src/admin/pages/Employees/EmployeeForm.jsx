import { useEffect, useState } from 'react';
import { Loader2 } from 'lucide-react';
import Modal from '../../components/common/Modal.jsx';
import { Input, Select } from '../../components/common/FormField.jsx';
import useForm from '../../hooks/useForm.js';
import { EMPLOYEE_STATUS_LIST, POSITIONS } from '../../../shared/utils/constants.js';
import { firstError, isEmail, isPhone, minLength, required } from '../../../shared/utils/validators.js';

const EMPTY = {
  fullName: '',
  phone: '',
  email: '',
  position: POSITIONS[1],
  username: '',
  status: 'Đang làm việc',
};

const validate = (v) => ({
  fullName: firstError(required(v.fullName, 'Họ và tên'), minLength(v.fullName.trim(), 2, 'Họ và tên')),
  phone: firstError(required(v.phone, 'Số điện thoại'), isPhone(v.phone)),
  email: firstError(required(v.email, 'Email'), isEmail(v.email)),
  position: required(v.position, 'Chức vụ'),
  username: firstError(
    required(v.username, 'Tài khoản'),
    /^[a-z0-9._]{3,20}$/.test(String(v.username).trim()) ? '' : 'Tài khoản gồm 3-20 ký tự: chữ thường, số, dấu . hoặc _',
  ),
});

export default function EmployeeForm({ open, employee, onClose, onSubmit }) {
  const isEdit = Boolean(employee);
  const [saving, setSaving] = useState(false);
  const { values, errors, handleChange, handleSubmit, reset } = useForm(EMPTY, validate);

  useEffect(() => {
    if (open) reset(employee ? { ...EMPTY, ...employee } : EMPTY);
  }, [open, employee, reset]);

  const submit = async (v) => {
    setSaving(true);
    try {
      await onSubmit({
        fullName: v.fullName.trim(),
        phone: v.phone.replace(/\s/g, ''),
        email: v.email.trim(),
        position: v.position,
        username: v.username.trim(),
        status: v.status,
      });
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal
      open={open}
      onClose={saving ? undefined : onClose}
      title={isEdit ? `Chỉnh sửa nhân viên ${employee.id}` : 'Thêm nhân viên mới'}
      size="lg"
      footer={
        <>
          <button type="button" className="btn-secondary" onClick={onClose} disabled={saving}>
            Hủy bỏ
          </button>
          <button type="submit" form="employee-form" className="btn-primary" disabled={saving}>
            {saving && <Loader2 size={16} className="animate-spin" />}
            {isEdit ? 'Lưu thay đổi' : 'Thêm nhân viên'}
          </button>
        </>
      }
    >
      <form id="employee-form" onSubmit={handleSubmit(submit)} noValidate className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <Input label="Họ và tên" name="fullName" value={values.fullName} onChange={handleChange} error={errors.fullName} className="sm:col-span-2" required />
        <Input label="Số điện thoại" name="phone" value={values.phone} onChange={handleChange} error={errors.phone} placeholder="0912345678" required />
        <Input label="Email" name="email" type="email" value={values.email} onChange={handleChange} error={errors.email} required />
        <Select label="Chức vụ" name="position" value={values.position} onChange={handleChange} options={POSITIONS} error={errors.position} required />
        <Select label="Trạng thái làm việc" name="status" value={values.status} onChange={handleChange} options={EMPLOYEE_STATUS_LIST} />
        <Input
          label="Tài khoản"
          name="username"
          value={values.username}
          onChange={handleChange}
          error={errors.username}
          placeholder="VD: letan03"
          hint="Dùng để đăng nhập hệ thống"
          className="sm:col-span-2"
          required
        />
      </form>
    </Modal>
  );
}
