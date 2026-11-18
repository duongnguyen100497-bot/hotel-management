import { useEffect, useState } from 'react';
import { Loader2 } from 'lucide-react';
import Modal from '../../components/common/Modal.jsx';
import { Input, Select } from '../../components/common/FormField.jsx';
import useForm from '../../hooks/useForm.js';
import { NATIONALITIES } from '../../../shared/utils/constants.js';
import { firstError, isEmail, isPhone, minLength, required } from '../../../shared/utils/validators.js';

const EMPTY = { fullName: '', phone: '', email: '', idNumber: '', nationality: 'Việt Nam', address: '' };

const validate = (v) => ({
  fullName: firstError(required(v.fullName, 'Họ và tên'), minLength(v.fullName.trim(), 2, 'Họ và tên')),
  phone: firstError(required(v.phone, 'Số điện thoại'), isPhone(v.phone)),
  email: isEmail(v.email),
  idNumber: firstError(required(v.idNumber, 'CCCD/Hộ chiếu'), minLength(v.idNumber.trim(), 6, 'CCCD/Hộ chiếu')),
  nationality: required(v.nationality, 'Quốc tịch'),
});

export default function CustomerForm({ open, customer, onClose, onSubmit }) {
  const isEdit = Boolean(customer);
  const [saving, setSaving] = useState(false);
  const { values, errors, handleChange, handleSubmit, reset } = useForm(EMPTY, validate);

  useEffect(() => {
    if (open) reset(customer ? { ...EMPTY, ...customer } : EMPTY);
  }, [open, customer, reset]);

  const submit = async (v) => {
    setSaving(true);
    try {
      await onSubmit({
        fullName: v.fullName.trim(),
        phone: v.phone.replace(/[\s.-]/g, ''),
        email: v.email.trim(),
        idNumber: v.idNumber.trim(),
        nationality: v.nationality,
        address: v.address.trim(),
      });
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal
      open={open}
      onClose={saving ? undefined : onClose}
      title={isEdit ? `Chỉnh sửa khách hàng ${customer.id}` : 'Thêm khách hàng mới'}
      size="lg"
      footer={
        <>
          <button type="button" className="btn-secondary" onClick={onClose} disabled={saving}>
            Hủy bỏ
          </button>
          <button type="submit" form="customer-form" className="btn-primary" disabled={saving}>
            {saving && <Loader2 size={16} className="animate-spin" />}
            {isEdit ? 'Lưu thay đổi' : 'Thêm khách hàng'}
          </button>
        </>
      }
    >
      <form id="customer-form" onSubmit={handleSubmit(submit)} noValidate className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <Input label="Họ và tên" name="fullName" value={values.fullName} onChange={handleChange} error={errors.fullName} className="sm:col-span-2" required />
        <Input label="Số điện thoại" name="phone" value={values.phone} onChange={handleChange} error={errors.phone} placeholder="0901234567" required />
        <Input label="Email" name="email" type="email" value={values.email} onChange={handleChange} error={errors.email} placeholder="ten@email.com" />
        <Input label="CCCD / Hộ chiếu" name="idNumber" value={values.idNumber} onChange={handleChange} error={errors.idNumber} required />
        <Select label="Quốc tịch" name="nationality" value={values.nationality} onChange={handleChange} options={NATIONALITIES} error={errors.nationality} required />
        <Input label="Địa chỉ" name="address" value={values.address} onChange={handleChange} className="sm:col-span-2" />
      </form>
    </Modal>
  );
}
