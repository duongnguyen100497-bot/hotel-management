import { useRef, useState } from 'react';
import { Hotel, ImagePlus, Loader2, Save, Trash2 } from 'lucide-react';
import { Input, Select } from '../../components/common/FormField.jsx';
import useForm from '../../hooks/useForm.js';
import { useSettings } from '../../../shared/contexts/SettingsContext.jsx';
import { useToast } from '../../../shared/contexts/ToastContext.jsx';
import { firstError, isEmail, isPhone, required } from '../../../shared/utils/validators.js';

const MAX_LOGO_BYTES = 512 * 1024;

const validate = (v) => ({
  name: required(v.name, 'Tên khách sạn'),
  address: required(v.address, 'Địa chỉ'),
  phone: firstError(required(v.phone, 'Số điện thoại'), isPhone(v.phone)),
  email: firstError(required(v.email, 'Email'), isEmail(v.email)),
  checkInTime: required(v.checkInTime, 'Giờ nhận phòng'),
  checkOutTime: required(v.checkOutTime, 'Giờ trả phòng'),
});

export default function HotelInfoTab() {
  const toast = useToast();
  const { hotel, updateHotel } = useSettings();
  const [saving, setSaving] = useState(false);
  const [logo, setLogo] = useState(hotel.logo);
  const fileRef = useRef(null);
  const { values, errors, handleChange, handleSubmit } = useForm({ ...hotel, stars: String(hotel.stars ?? 5) }, validate);

  const pickLogo = (e) => {
    const file = e.target.files?.[0];
    e.target.value = '';
    if (!file) return;
    if (!file.type.startsWith('image/')) {
      toast.error('Vui lòng chọn file hình ảnh (PNG, JPG, SVG...)');
      return;
    }
    if (file.size > MAX_LOGO_BYTES) {
      toast.error('Logo tối đa 512KB');
      return;
    }
    const reader = new FileReader();
    reader.onload = () => setLogo(reader.result);
    reader.onerror = () => toast.error('Không đọc được file ảnh');
    reader.readAsDataURL(file);
  };

  const submit = async (v) => {
    setSaving(true);
    try {
      await new Promise((resolve) => setTimeout(resolve, 300));
      updateHotel({
        name: v.name.trim(),
        address: v.address.trim(),
        phone: v.phone.trim(),
        email: v.email.trim(),
        website: (v.website || '').trim(),
        stars: Number(v.stars),
        checkInTime: v.checkInTime,
        checkOutTime: v.checkOutTime,
        logo,
      });
      toast.success('Đã lưu thông tin khách sạn');
    } finally {
      setSaving(false);
    }
  };

  return (
    <form onSubmit={handleSubmit(submit)} noValidate className="card p-5 sm:p-6">
      <h2 className="text-base font-semibold">Thông tin khách sạn</h2>
      <p className="mb-5 text-sm text-slate-500">Hiển thị trên thanh menu, màn hình đăng nhập và hóa đơn.</p>

      <div className="mb-6 flex items-center gap-4">
        {logo ? (
          <img src={logo} alt="Logo khách sạn" className="h-20 w-20 rounded-xl border border-slate-200 bg-white object-contain p-1" />
        ) : (
          <span className="flex h-20 w-20 items-center justify-center rounded-xl bg-gradient-to-br from-sky-300 to-sky-500 text-navy-950">
            <Hotel size={36} />
          </span>
        )}
        <div>
          <p className="text-sm font-medium text-slate-700">Logo khách sạn</p>
          <p className="mb-2 text-xs text-slate-400">PNG, JPG hoặc SVG, tối đa 512KB.</p>
          <div className="flex gap-2">
            <button type="button" className="btn-secondary" onClick={() => fileRef.current?.click()}>
              <ImagePlus size={16} /> Chọn ảnh
            </button>
            {logo && (
              <button type="button" className="btn-ghost" onClick={() => setLogo(null)}>
                <Trash2 size={16} /> Gỡ logo
              </button>
            )}
          </div>
          <input ref={fileRef} type="file" accept="image/*" className="hidden" onChange={pickLogo} />
        </div>
      </div>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <Input label="Tên khách sạn" name="name" value={values.name} onChange={handleChange} error={errors.name} className="sm:col-span-2" required />
        <Input label="Địa chỉ" name="address" value={values.address} onChange={handleChange} error={errors.address} className="sm:col-span-2" required />
        <Input label="Số điện thoại" name="phone" value={values.phone} onChange={handleChange} error={errors.phone} required />
        <Input label="Email liên hệ" name="email" type="email" value={values.email} onChange={handleChange} error={errors.email} required />
        <Input label="Website" name="website" value={values.website || ''} onChange={handleChange} />
        <Select label="Hạng sao" name="stars" value={values.stars} onChange={handleChange} options={['1', '2', '3', '4', '5'].map((s) => ({ value: s, label: `${s} sao` }))} />
        <Input label="Giờ nhận phòng" name="checkInTime" type="time" value={values.checkInTime} onChange={handleChange} error={errors.checkInTime} required />
        <Input label="Giờ trả phòng" name="checkOutTime" type="time" value={values.checkOutTime} onChange={handleChange} error={errors.checkOutTime} required />
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
