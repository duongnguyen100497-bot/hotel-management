import { useState } from 'react';
import { CheckCircle2, ChevronDown, Clock, ExternalLink, Loader2, Mail, MapPin, Phone } from 'lucide-react';
import Photo from '../components/Photo.jsx';
import { FormAlert, TextField } from '../components/FormControls.jsx';
import SectionHeading from '../components/SectionHeading.jsx';
import { useCustomerAuth } from '../contexts/CustomerAuthContext.jsx';
import { useSettings } from '../../shared/contexts/SettingsContext.jsx';
import useDocumentTitle from '../hooks/useDocumentTitle.js';
import supportService, { MESSAGE_MAX, SUPPORT_TOPICS, validateSupport } from '../services/supportService.js';
import { BOOKING_POLICY } from '../data/bookingPolicy.js';

function InfoRow({ icon: Icon, label, children }) {
  return (
    <li className="flex gap-4">
      <span className="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl bg-accent-soft text-accent"><Icon size={22} /></span>
      <div>
        <p className="text-sm text-muted">{label}</p>
        <div className="mt-0.5 font-semibold text-brand">{children}</div>
      </div>
    </li>
  );
}

function SupportForm() {
  const { customer, account } = useCustomerAuth();
  const [values, setValues] = useState({ fullName: customer?.fullName || '', email: account?.email || '', phone: customer?.phone || '', topic: SUPPORT_TOPICS[0], message: '' });
  const [errors, setErrors] = useState({});
  const [formError, setFormError] = useState('');
  const [sending, setSending] = useState(false);
  const [sent, setSent] = useState(null);
  const set = (k) => (e) => { setValues((v) => ({ ...v, [k]: e.target.value })); setErrors((er) => ({ ...er, [k]: undefined })); };

  const submit = async (e) => {
    e.preventDefault();
    const found = validateSupport(values);
    setErrors(found);
    if (Object.keys(found).length) return;
    setSending(true);
    setFormError('');
    try {
      setSent(await supportService.submit(values));
    } catch (err) {
      if (err.fields) setErrors(err.fields); else setFormError(err.message || 'Không thể gửi yêu cầu. Vui lòng thử lại.');
    } finally {
      setSending(false);
    }
  };

  if (sent) {
    return (
      <div role="status" className="rounded-3xl bg-green-50 p-8 text-center">
        <span className="mx-auto flex h-16 w-16 items-center justify-center rounded-full bg-white text-success"><CheckCircle2 size={34} /></span>
        <h3 className="mt-4 text-xl font-bold text-brand">Đã ghi nhận yêu cầu {sent.id}</h3>
        <p className="mt-2 text-slate-600">Cảm ơn bạn, {sent.fullName}. Chủ đề: {sent.topic}.</p>
        <p className="mt-2 text-xs text-muted">Bản dùng thử: yêu cầu mới được lưu trên trình duyệt này, chưa chuyển tới nhân viên khách sạn. Nếu cần gấp, vui lòng gọi hotline.</p>
        <button type="button" className="c-btn-outline mt-6" onClick={() => { setSent(null); setValues((v) => ({ ...v, message: '' })); }}>Gửi yêu cầu khác</button>
      </div>
    );
  }

  return (
    <form onSubmit={submit} noValidate className="grid gap-5 sm:grid-cols-2">
      <div className="sm:col-span-2"><FormAlert>{formError}</FormAlert></div>
      <TextField label="Họ và tên" autoComplete="name" value={values.fullName} onChange={set('fullName')} error={errors.fullName} required />
      <TextField label="Email" type="email" autoComplete="email" value={values.email} onChange={set('email')} error={errors.email} required />
      <TextField label="Số điện thoại" type="tel" autoComplete="tel" value={values.phone} onChange={set('phone')} error={errors.phone} hint="Không bắt buộc" />
      <TextField as="select" label="Chủ đề" value={values.topic} onChange={set('topic')} error={errors.topic} required>
        {SUPPORT_TOPICS.map((t) => <option key={t} value={t}>{t}</option>)}
      </TextField>
      <TextField as="textarea" rows={5} className="sm:col-span-2" label="Nội dung" value={values.message} onChange={set('message')} error={errors.message} maxLength={MESSAGE_MAX + 100} hint={`${values.message.trim().length}/${MESSAGE_MAX} ký tự`} required />
      <div className="sm:col-span-2">
        <button type="submit" className="c-btn-primary c-btn-lg" disabled={sending}>{sending && <Loader2 size={18} className="animate-spin" />} Gửi yêu cầu</button>
      </div>
    </form>
  );
}

export default function Contact() {
  useDocumentTitle('Liên hệ và hỗ trợ');
  const { hotel } = useSettings();
  const mapUrl = `https://www.google.com/maps/search/?api=1&query=${encodeURIComponent(`${hotel.name} ${hotel.address}`)}`;
  const faqs = [
    { q: 'Tôi có thể hủy hoặc đổi đặt phòng không?', a: `Bạn hủy miễn phí trước giờ nhận phòng ${BOOKING_POLICY.freeCancelHours} giờ ngay tại mục Đặt phòng của tôi. Gần hơn mốc này, vui lòng gọi hotline để được hỗ trợ đổi hoặc hủy.` },
    { q: 'Giờ nhận phòng và trả phòng là khi nào?', a: `Nhận phòng từ ${hotel.checkInTime}, trả phòng trước ${hotel.checkOutTime}. Nhận sớm hoặc trả muộn tùy tình trạng phòng; khách sạn hỗ trợ gửi hành lý miễn phí.` },
    { q: 'Tôi cần mang giấy tờ gì khi nhận phòng?', a: 'Người nhận phòng xuất trình CCCD hoặc hộ chiếu, và cho lễ tân biết mã đặt phòng.' },
    { q: 'Khi nào tôi phải thanh toán?', a: 'Bạn không trả tiền khi gửi yêu cầu đặt phòng. Hiện có thể chọn thanh toán tại khách sạn hoặc chuyển khoản theo hướng dẫn của lễ tân. Thanh toán thẻ trực tuyến chưa được mở.' },
    { q: 'Đặt phòng của tôi đang ở trạng thái Chờ xác nhận nghĩa là gì?', a: 'Phòng đã được giữ cho bạn và đang chờ lễ tân xác nhận. Bạn sẽ được liên hệ qua email hoặc điện thoại trước ngày nhận phòng.' },
    { q: 'Khách sạn có đưa đón sân bay không?', a: 'Có. Dịch vụ đưa đón sân bay bằng xe riêng tính phí theo bảng giá dịch vụ, trừ khi thuộc một ưu đãi đang áp dụng. Vui lòng đặt trước qua lễ tân.' },
  ];

  return (
    <>
      <section className="bg-brand pb-20 pt-14 sm:pt-20" aria-labelledby="contact-title">
        <div className="c-container">
          <h1 id="contact-title" className="text-4xl font-extrabold tracking-tight text-white sm:text-5xl">Liên hệ và hỗ trợ</h1>
          <p className="mt-3 max-w-xl text-sky-200/85">Cần hỗ trợ đặt phòng hoặc có câu hỏi trước chuyến đi? Chúng tôi luôn sẵn sàng, 24/7.</p>
        </div>
      </section>

      <section className="c-container -mt-10 grid gap-8 pb-20 lg:grid-cols-[1fr_1.15fr]" aria-label="Thông tin liên hệ">
        <div className="rounded-3xl border border-slate-100 bg-white p-7 shadow-lift">
          <h2 className="text-xl font-bold text-brand">{hotel.name}</h2>
          <ul className="mt-6 space-y-6">
            <InfoRow icon={MapPin} label="Địa chỉ">{hotel.address}</InfoRow>
            <InfoRow icon={Phone} label="Hotline"><a href={`tel:${String(hotel.phone).replace(/\s/g, '')}`} className="hover:text-accent">{hotel.phone}</a></InfoRow>
            <InfoRow icon={Mail} label="Email"><a href={`mailto:${hotel.email}`} className="hover:text-accent">{hotel.email}</a></InfoRow>
            <InfoRow icon={Clock} label="Giờ làm việc của lễ tân">Mở cửa 24/7. Nhận phòng từ {hotel.checkInTime}, trả phòng trước {hotel.checkOutTime}.</InfoRow>
          </ul>
        </div>
        <div className="relative overflow-hidden rounded-3xl shadow-lift">
          <Photo variant="map" alt={`Bản đồ minh họa vị trí ${hotel.name}`} className="h-full min-h-[18rem] w-full" />
          <a href={mapUrl} target="_blank" rel="noopener noreferrer" className="c-btn-primary absolute bottom-4 left-4">Mở Google Maps <ExternalLink size={15} /></a>
          <span className="absolute right-4 top-4 rounded-full bg-white/90 px-3 py-1 text-xs font-medium text-slate-600">Bản đồ minh họa</span>
        </div>
      </section>

      <section className="bg-surface c-section" aria-labelledby="support-title">
        <div className="c-container grid gap-12 lg:grid-cols-[1fr_1.1fr] lg:gap-16">
          <div>
            <SectionHeading id="support-title" title="Gửi yêu cầu hỗ trợ" description="Mô tả vấn đề của bạn, chúng tôi sẽ phản hồi qua email hoặc điện thoại." />
            <div className="mt-8 rounded-3xl bg-white p-6 shadow-soft sm:p-8"><SupportForm /></div>
          </div>
          <div>
            <SectionHeading id="faq-title" title="Câu hỏi thường gặp" />
            <div className="mt-8 divide-y divide-slate-200 rounded-3xl bg-white px-6 shadow-soft" role="list">
              {faqs.map((f) => (
                <details key={f.q} className="group py-5" role="listitem">
                  <summary className="flex cursor-pointer list-none items-start justify-between gap-4 font-semibold text-brand focus:outline-none focus-visible:ring-2 focus-visible:ring-accent [&::-webkit-details-marker]:hidden">
                    {f.q}
                    <ChevronDown size={20} className="mt-0.5 shrink-0 text-accent transition-transform group-open:rotate-180" />
                  </summary>
                  <p className="mt-3 leading-relaxed text-slate-600">{f.a}</p>
                </details>
              ))}
            </div>
          </div>
        </div>
      </section>
    </>
  );
}
