import { useEffect, useState } from 'react';
import { Loader2 } from 'lucide-react';
import Modal from '../../components/common/Modal.jsx';
import { Input, Select, Textarea } from '../../components/common/FormField.jsx';
import useForm from '../../hooks/useForm.js';
import { ROOM_STATUS, ROOM_STATUS_LIST, ROOM_TYPES } from '../../../shared/utils/constants.js';
import { firstError, positiveNumber, required } from '../../../shared/utils/validators.js';

const EMPTY = {
  code: '',
  name: '',
  type: 'Standard',
  price: '',
  floor: 1,
  capacity: 2,
  status: 'Available',
  description: '',
};

const validate = (v) => ({
  code: firstError(
    required(v.code, 'Mã phòng'),
    /^[A-Za-z0-9-]{2,10}$/.test(String(v.code).trim()) ? '' : 'Mã phòng gồm 2-10 ký tự chữ hoặc số',
  ),
  name: required(v.name, 'Tên phòng'),
  type: required(v.type, 'Loại phòng'),
  price: firstError(required(v.price, 'Giá phòng'), positiveNumber(v.price, 'Giá phòng')),
  floor: firstError(required(v.floor, 'Tầng'), positiveNumber(v.floor, 'Tầng')),
  capacity: firstError(required(v.capacity, 'Sức chứa'), positiveNumber(v.capacity, 'Sức chứa')),
});

export default function RoomForm({ open, room, onClose, onSubmit }) {
  const isEdit = Boolean(room);
  const [saving, setSaving] = useState(false);
  const { values, errors, handleChange, handleSubmit, reset } = useForm(EMPTY, validate);

  useEffect(() => {
    if (open) reset(room ? { ...EMPTY, ...room } : EMPTY);
  }, [open, room, reset]);

  const submit = async (v) => {
    setSaving(true);
    try {
      await onSubmit({
        ...v,
        code: v.code.trim().toUpperCase(),
        name: v.name.trim(),
        price: Number(v.price),
        floor: Number(v.floor),
        capacity: Number(v.capacity),
      });
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal
      open={open}
      onClose={saving ? undefined : onClose}
      title={isEdit ? `Chỉnh sửa ${room.name}` : 'Thêm phòng mới'}
      size="lg"
      footer={
        <>
          <button type="button" className="btn-secondary" onClick={onClose} disabled={saving}>
            Hủy bỏ
          </button>
          <button type="submit" form="room-form" className="btn-primary" disabled={saving}>
            {saving && <Loader2 size={16} className="animate-spin" />}
            {isEdit ? 'Lưu thay đổi' : 'Thêm phòng'}
          </button>
        </>
      }
    >
      <form id="room-form" onSubmit={handleSubmit(submit)} noValidate className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <Input label="Mã phòng" name="code" value={values.code} onChange={handleChange} error={errors.code} disabled={isEdit} placeholder="VD: P107" required />
        <Input label="Tên phòng" name="name" value={values.name} onChange={handleChange} error={errors.name} placeholder="VD: Phòng 107" required />
        <Select label="Loại phòng" name="type" value={values.type} onChange={handleChange} options={ROOM_TYPES} error={errors.type} required />
        <Input label="Giá phòng (VND / đêm)" name="price" type="number" min="0" step="10000" value={values.price} onChange={handleChange} error={errors.price} required />
        <Input label="Tầng" name="floor" type="number" min="1" value={values.floor} onChange={handleChange} error={errors.floor} required />
        <Input label="Sức chứa (người)" name="capacity" type="number" min="1" value={values.capacity} onChange={handleChange} error={errors.capacity} required />
        <Select
          label="Trạng thái"
          name="status"
          value={values.status}
          onChange={handleChange}
          options={ROOM_STATUS_LIST.map((s) => ({ value: s, label: ROOM_STATUS[s].label }))}
          className="sm:col-span-2"
        />
        <Textarea label="Mô tả" name="description" value={values.description} onChange={handleChange} className="sm:col-span-2" placeholder="Tiện nghi, diện tích, hướng nhìn..." />
      </form>
    </Modal>
  );
}
