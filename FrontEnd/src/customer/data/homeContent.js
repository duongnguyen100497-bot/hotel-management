import {
  Clock, ConciergeBell, Plane, Shirt, Sparkles, Utensils, Waves, Wifi,
} from 'lucide-react';

// Nội dung giới thiệu cho trang chủ.
// LƯU Ý: đây là nội dung MẪU (đánh giá, ưu đãi, thư viện ảnh). Thông tin liên hệ, tên khách sạn,
// giờ nhận/trả phòng lấy từ Cài đặt khách sạn (SettingsContext) nên luôn khớp với Admin.

export const NAV_LINKS = [
  { label: 'Trang chủ', to: '/' },
  { label: 'Phòng nghỉ', to: '/rooms' },
  { label: 'Tiện ích', to: '/#amenities' },
  { label: 'Giới thiệu', to: '/#about' },
  { label: 'Liên hệ', to: '/contact' },
];

export const HERO = {
  title: 'Trải nghiệm kỳ nghỉ tuyệt vời của bạn',
  // Tách dòng để hiệu ứng mở đầu chạy lần lượt từng dòng.
  lines: ['Trải nghiệm', 'kỳ nghỉ tuyệt vời', 'của bạn'],
  description:
    'Không gian sang trọng, dịch vụ tận tâm và vị trí thuận tiện ngay trung tâm thành phố. Đặt phòng trực tuyến chỉ trong vài phút.',
};

export const ABOUT = {
  title: 'Sự tinh tế trong từng chi tiết',
  paragraphs: [
    'Tọa lạc ngay trung tâm thành phố, khách sạn mang đến không gian nghỉ dưỡng hiện đại với tông màu xanh navy trang nhã, phù hợp cho cả chuyến công tác lẫn kỳ nghỉ cùng gia đình.',
    'Đội ngũ lễ tân phục vụ 24/7, nhà hàng buffet sáng, spa thư giãn và dịch vụ đưa đón sân bay giúp bạn trọn vẹn từ lúc đặt phòng đến khi trả phòng.',
  ],
};

export const AMENITIES = [
  { icon: Wifi, title: 'Wi-Fi tốc độ cao', description: 'Miễn phí trong toàn bộ khách sạn và tất cả các phòng.' },
  { icon: Utensils, title: 'Nhà hàng & Buffet sáng', description: 'Buffet sáng mỗi ngày từ 6:00 đến 10:00 cùng thực đơn tối theo set.' },
  { icon: Sparkles, title: 'Spa & Massage', description: 'Liệu trình massage thư giãn toàn thân giúp phục hồi sau ngày dài.' },
  { icon: Waves, title: 'Hồ bơi', description: 'Hồ bơi ngoài trời với khu nghỉ thư giãn dành cho khách lưu trú.' },
  { icon: Plane, title: 'Đưa đón sân bay', description: 'Xe riêng đón hoặc tiễn sân bay theo yêu cầu, đặt trước dễ dàng.' },
  { icon: Shirt, title: 'Giặt ủi', description: 'Dịch vụ giặt ủi thường và giặt nhanh trong 4 giờ.' },
  { icon: ConciergeBell, title: 'Lễ tân 24/7', description: 'Luôn sẵn sàng hỗ trợ du lịch, đặt xe và mọi yêu cầu của bạn.' },
  { icon: Clock, title: 'Nhận phòng linh hoạt', description: 'Gửi hành lý miễn phí nếu bạn đến sớm hoặc rời đi muộn.' },
];

export const OFFERS = [
  {
    id: 'stay-3',
    tag: 'Lưu trú dài ngày',
    title: 'Ở 3 đêm, tặng buffet sáng',
    description: 'Tặng buffet sáng cho 2 người trong suốt kỳ nghỉ khi đặt từ 3 đêm liên tiếp.',
  },
  {
    id: 'airport',
    tag: 'Phòng Suite',
    title: 'Đón sân bay miễn phí',
    description: 'Khách đặt phòng Suite được tặng một chiều đưa đón sân bay bằng xe riêng.',
  },
  {
    id: 'family',
    tag: 'Gia đình',
    title: 'Gia đình nhỏ, kỳ nghỉ lớn',
    description: 'Trẻ em dưới 6 tuổi ở cùng giường với bố mẹ hoàn toàn miễn phí.',
  },
];

export const TESTIMONIALS = [
  {
    id: 't1',
    name: 'Nguyễn Thanh Hà',
    trip: 'Chuyến công tác · Phòng Superior',
    rating: 5,
    quote: 'Phòng sạch sẽ, bàn làm việc rộng và Wi-Fi rất ổn định. Nhân viên lễ tân hỗ trợ nhận phòng rất nhanh.',
  },
  {
    id: 't2',
    name: 'Emma Wilson',
    trip: 'Kỳ nghỉ cặp đôi · Phòng Deluxe',
    rating: 5,
    quote: 'The balcony view at night was beautiful and the breakfast buffet had great variety. We will definitely come back.',
  },
  {
    id: 't3',
    name: 'Phạm Quốc Dũng',
    trip: 'Gia đình 4 người · Phòng Family',
    rating: 4,
    quote: 'Phòng rộng, hai giường đôi rất phù hợp cho các con. Dịch vụ đưa đón sân bay đúng giờ, tài xế thân thiện.',
  },
];

import { SITE_PHOTOS } from './photos.js';

// variant khớp với component Photo (ảnh minh họa mặc định).
export const GALLERY = [
  { id: 'g1', src: SITE_PHOTOS.lobbyLounge, variant: 'lobby', label: 'Sảnh đón tiếp', className: 'md:col-span-2 md:row-span-2' },
  { id: 'g2', src: SITE_PHOTOS.pool, variant: 'pool', label: 'Hồ bơi' },
  { id: 'g3', src: SITE_PHOTOS.reception, variant: 'restaurant', label: 'Quầy lễ tân' },
  { id: 'g4', src: SITE_PHOTOS.deluxeBedroom, variant: 'spa', label: 'Phòng Deluxe' },
  { id: 'g5', src: SITE_PHOTOS.suiteBedroom, variant: 'suite', label: 'Phòng Suite' },
];

export const FOOTER_LINKS = {
  explore: [
    { label: 'Phòng nghỉ', to: '/rooms' },
    { label: 'Tiện ích', to: '/#amenities' },
    { label: 'Giới thiệu', to: '/#about' },
    { label: 'Liên hệ & hỗ trợ', to: '/contact' },
  ],
  account: [
    { label: 'Đăng nhập', to: '/login' },
    { label: 'Đăng ký', to: '/register' },
    { label: 'Đặt phòng của tôi', to: '/my-bookings' },
    { label: 'Tài khoản', to: '/account' },
  ],
};
