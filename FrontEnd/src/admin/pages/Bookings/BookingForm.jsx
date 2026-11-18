import { useEffect, useMemo, useState } from 'react';
import { Loader2 } from 'lucide-react';
import Modal from '../../components/common/Modal.jsx';
import { Input, Select, Textarea } from '../../components/common/FormField.jsx';
import useForm from '../../hooks/useForm.js';
import { ROOM_TYPES } from '../../../shared/utils/constants.js';
import { addDays, diffDays, todayStr } from '../../../shared/utils/dateUtils.js';
import { formatCurrency } from '../../../shared/utils/formatters.js';
import { firstError, positiveNumber, required } from '../../../shared/utils/validators.js';

const emptyForm = () => ({
  customerId: '',
  roomType: '',
  roomCode: '',
  checkIn: todayStr(),
  checkOut: addDays(todayStr(), 1),
  guests: 2,
  status: 'Confirmed',
  note: '',
});

export default function BookingForm({ open, booking, customers, rooms, onClose, onSubmit }) {
  const isEdit = Boolean(booking);
  const [saving, setSaving] = useState(false);

  const validate = (v) => {
    const room = rooms.find((r) => r.code === v.roomCode);
    return {
      customerId: required(v.customerId, 'Khách hàng'),
      roomType: required(v.roomType, 'Loại phòng'),
      roomCode: required(v.roomCode, 'Phòng'),
      checkIn: firstError(
        required(v.checkIn, 'Ngày nhận phòng'),
        !isEdit && v.checkIn && v.checkIn < todayStr() ? 'Ngày nhận phòng không được ở quá khứ' : '',
      ),
      checkOut: firstError(
        required(v.checkOut, 'Ngày trả phòng'),
        v.checkIn && v.checkOut && v.checkOut <= v.checkIn ? 'Ngày trả phòng phải sau ngày nhận phòng' : '',
      ),
      guests: firstError(
        required(v.guests, 'Số khách'),
        positiveNumber(v.guests, 'Số khách'),
        room && Number(v.guests) > room.capacity ? `Phòng ${room.code} tối đa ${room.capacity} khách` : '',
      ),
    };
  };

  const { values, errors, setValues, handleChange, handleSubmit, reset } = useForm(emptyForm(), validate);

  useEffect(() => {
    if (!open) return;
    reset(
      booking
        ? {
            customerId: booking.customerId,
            roomType: booking.roomType,
            roomCode: booking.roomCode,
            checkIn: booking.checkIn,
            checkOut: booking.checkOut,
            guests: booking.guests,
            status: booking.status,
            note: booking.note || '',
          }
        : emptyForm(),
    );
  }, [open, booking, reset]);

  const roomOptions = useMemo(
    () => rooms.filter((r) => r.type === values.roomType && (r.status !== 'Maintenance' || r.code === values.roomCode)),
    [rooms, values.roomType, values.roomCode],
  );
  const selectedRoom = rooms.find((r) => r.code === values.roomCode);
  const nights = values.checkIn && values.checkOut ? Math.max(0, diffDays(values.checkIn, values.checkOut)) : 0;
  const total = selectedRoom ? nights * selectedRoom.price : 0;

  const onTypeChange = (e) => setValues((v) => ({ ...v, roomType: e.target.value, roomCode: '' }));

  const submit = async (v) => {
    const customer = customers.find((c) => c.id === v.customerId);
    setSaving(true);
    try {
      await onSubmit({
        customerId: customer.id,
        customerName: customer.fullName,
        phone: customer.phone,
        roomType: v.roomType,
        roomCode: v.roomCode,
        checkIn: v.checkIn,
        checkOut: v.checkOut,
        guests: Number(v.guests),
        total,
        status: v.status,
        note: v.note.trim(),
      });
    } finally {
      setSaving(false);
    }
  };

  const statusOptions = isEdit
    ? [
        { value: 'Pending', label: 'Chờ xác nhận' },
        { value: 'Confirmed', label: 'Đã xác nhận' },
        ...(booking.status === 'Checked-in' ? [{ value: 'Checked-in', label: 'Đang lưu trú' }] : []),
      ]
    : [
        { value: 'Pending', label: 'Chờ xác nhận' },
        { value: 'Confirmed', label: 'Đã xác nhận' },
      ];

  return (
    <Modal
      open={open}
      onClose={saving ? undefined : onClose}
      title={isEdit ? `Chỉnh sửa đặt phòng ${booking.id}` : 'Tạo đặt phòng mới'}
      size="lg"
      footer={
        <>
          <button type="button" className="btn-secondary" onClick={onClose} disabled={saving}>
            Hủy bỏ
          </button>
          <button type="submit" form="booking-form" className="btn-primary" disabled={saving}>
            {saving && <Loader2 size={16} className="animate-spin" />}
            {isEdit ? 'Lưu thay đổi' : 'Tạo đặt phòng'}
          </button>
        </>
      }
    >
      <form id="booking-form" onSubmit={handleSubmit(submit)} noValidate className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <Select
          label="Khách hàng"
          name="customerId"
          value={values.customerId}
          onChange={handleChange}
          error={errors.customerId}
          placeholder="-- Chọn khách hàng --"
          options={customers.map((c) => ({ value: c.id, label: `${c.id} – ${c.fullName} (${c.phone})` }))}
          className="sm:col-span-2"
          required
        />
        <Select
          label="Loại phòng"
          name="roomType"
          value={values.roomType}
          onChange={onTypeChange}
          error={errors.roomType}
          placeholder="-- Chọn loại phòng --"
          options={ROOM_TYPES}
          required
        />
        <Select
          label="Phòng"
          name="roomCode"
          value={values.roomCode}
          onChange={handleChange}
          error={errors.roomCode}
          disabled={!values.roomType}
          placeholder={values.roomType ? '-- Chọn phòng --' : 'Chọn loại phòng trước'}
          options={roomOptions.map((r) => ({ value: r.code, label: `${r.code} – Tầng ${r.floor} – ${formatCurrency(r.price)}` }))}
          required
        />
        <Input label="Ngày nhận phòng" name="checkIn" type="date" value={values.checkIn} onChange={handleChange} error={errors.checkIn} required />
        <Input label="Ngày trả phòng" name="checkOut" type="date" min={values.checkIn} value={values.checkOut} onChange={handleChange} error={errors.checkOut} required />
        <Input label="Số khách" name="guests" type="number" min="1" value={values.guests} onChange={handleChange} error={errors.guests} required />
        <Select label="Trạng thái" name="status" value={values.status} onChange={handleChange} options={statusOptions} />
        <Textarea label="Ghi chú" name="note" value={values.note} onChange={handleChange} className="sm:col-span-2" placeholder="Yêu cầu đặc biệt của khách..." />

        <div className="rounded-xl bg-sky-50 p-4 sm:col-span-2">
          <div className="flex items-center justify-between text-sm text-slate-600">
            <span>
              {nights} đêm × {selectedRoom ? formatCurrency(selectedRoom.price) : '—'}
            </span>
            <span className="text-lg font-semibold text-navy-900">{formatCurrency(total)}</span>
          </div>
          <p className="mt-1 text-xs text-slate-400">Tổng tiền phòng được tính tự động.</p>
        </div>
      </form>
    </Modal>
  );
}
