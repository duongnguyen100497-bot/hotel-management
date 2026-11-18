import { useEffect, useState } from 'react';
import { Loader2 } from 'lucide-react';
import Modal from '../../components/common/Modal.jsx';
import Toggle from '../../components/common/Toggle.jsx';
import { Input, Select, Textarea } from '../../components/common/FormField.jsx';
import useForm from '../../hooks/useForm.js';
import { SERVICE_TYPES } from '../../../shared/utils/constants.js';
import { firstError, minLength, positiveNumber, required } from '../../../shared/utils/validators.js';

const EMPTY = { name: '', price: '', type: SERVICE_TYPES[0], description: '', active: true };

const validate = (v) => ({
  name: firstError(required(v.name, 'Tên dịch vụ'), minLength(v.name.trim(), 2, 'Tên dịch vụ')),
  price: firstError(required(v.price, 'Giá dịch vụ'), positiveNumber(v.price, 'Giá dịch vụ')),
  type: required(v.type, 'Loại dịch vụ'),
});

export default function ServiceForm({ open, service, onClose, onSubmit }) {
  const isEdit = Boolean(service);
  const [saving, setSaving] = useState(false);
  const { values, errors, setValues, handleChange, handleSubmit, reset } = useForm(EMPTY, validate);

  useEffect(() => {
    if (open) reset(service ? { ...EMPTY, ...service } : EMPTY);
  }, [open, service, reset]);

  const submit = async (v) => {
    setSaving(true);
    try {
      await onSubmit({
        name: v.name.trim(),
        price: Number(v.price),
        type: v.type,
        description: v.description.trim(),
        active: Boolean(v.active),
      });
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal
      open={open}
      onClose={saving ? undefined : onClose}
      title={isEdit ? `Chỉnh sửa dịch vụ ${service.id}` : 'Thêm dịch vụ mới'}
      footer={
        <>
          <button type="button" className="btn-secondary" onClick={onClose} disabled={saving}>
            Hủy bỏ
          </button>
          <button type="submit" form="service-form" className="btn-primary" disabled={saving}>
            {saving && <Loader2 size={16} className="animate-spin" />}
            {isEdit ? 'Lưu thay đổi' : 'Thêm dịch vụ'}
          </button>
        </>
      }
    >
      <form id="service-form" onSubmit={handleSubmit(submit)} noValidate className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <Input label="Tên dịch vụ" name="name" value={values.name} onChange={handleChange} error={errors.name} placeholder="VD: Massage 90 phút" className="sm:col-span-2" required />
        <Select label="Loại dịch vụ" name="type" value={values.type} onChange={handleChange} options={SERVICE_TYPES} error={errors.type} required />
        <Input label="Giá (VND)" name="price" type="number" min="0" step="1000" value={values.price} onChange={handleChange} error={errors.price} required />
        <Textarea label="Mô tả" name="description" value={values.description} onChange={handleChange} className="sm:col-span-2" placeholder="Mô tả ngắn về dịch vụ..." />
        <div className="flex items-center justify-between rounded-lg border border-slate-200 px-3 py-2.5 sm:col-span-2">
          <div>
            <p className="text-sm font-medium text-slate-700">Trạng thái hoạt động</p>
            <p className="text-xs text-slate-400">Dịch vụ tạm ngưng sẽ không được chọn khi check-out.</p>
          </div>
          <Toggle checked={Boolean(values.active)} onChange={(active) => setValues((v) => ({ ...v, active }))} label="Trạng thái hoạt động" />
        </div>
      </form>
    </Modal>
  );
}
