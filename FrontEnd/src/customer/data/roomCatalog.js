import { ROOM_PHOTOS } from './photos.js';

// Thông tin trình bày dành riêng cho website khách hàng, theo LOẠI PHÒNG.
// Tên loại phòng, giá, sức chứa vẫn lấy từ dữ liệu Admin (roomService) để luôn đồng bộ;
// file này chỉ bổ sung những thứ Admin không lưu: diện tích, loại giường, tiện nghi, hình ảnh.
// Diện tích khớp với mô tả phòng trong Admin (25 / 30 / 38 / 45 / 65 m²).
//
// images: lấy từ photos.js (ảnh Unsplash). Để trống thì hiển thị ảnh minh họa mặc định. Muốn dùng ảnh của khách sạn, thay bằng đường dẫn,
// ví dụ: images: ['/images/rooms/deluxe-1.jpg', '/images/rooms/deluxe-2.jpg'].
export const ROOM_CATALOG = {
  Standard: {
    description:
      'Căn phòng gọn gàng với giường đôi thoải mái, phòng tắm vòi sen và đầy đủ tiện nghi cần thiết. Lựa chọn hợp lý cho chuyến công tác hoặc khám phá thành phố ngắn ngày.',
    promoPercent: 0, // % giảm giá (0 = không khuyến mãi). Chỉ bật khi khách sạn xác nhận chương trình thật.
    area: 25,
    bedType: '1 giường đôi',
    bedCount: 1,
    view: 'Hướng phố',
    amenities: ['wifi', 'ac', 'tv', 'shower', 'safe'],
    images: ROOM_PHOTOS.Standard,
    highlight: 'Gọn gàng, đủ tiện nghi cho chuyến đi ngắn ngày.',
  },
  Superior: {
    description:
      'Giường King, bàn làm việc rộng và minibar. Không gian 30 m² yên tĩnh, phù hợp cho cả công tác lẫn nghỉ ngơi.',
    promoPercent: 0, // % giảm giá (0 = không khuyến mãi). Chỉ bật khi khách sạn xác nhận chương trình thật.
    area: 30,
    bedType: '1 giường King',
    bedCount: 1,
    view: 'Hướng phố',
    amenities: ['wifi', 'ac', 'tv', 'desk', 'minibar', 'shower', 'safe'],
    images: ROOM_PHOTOS.Superior,
    highlight: 'Giường King, bàn làm việc và góc tiếp khách nhỏ.',
  },
  Deluxe: {
    description:
      'Phòng 38 m² có ban công riêng nhìn ra thành phố, bồn tắm đứng và khu làm việc. Buổi tối ngồi ngoài ban công là điểm nhấn của hạng phòng này.',
    promoPercent: 0, // % giảm giá (0 = không khuyến mãi). Chỉ bật khi khách sạn xác nhận chương trình thật.
    area: 38,
    bedType: '1 giường King',
    bedCount: 1,
    view: 'Tầm nhìn thành phố',
    amenities: ['wifi', 'ac', 'tv', 'desk', 'minibar', 'balcony', 'bathtub', 'cityView', 'safe'],
    images: ROOM_PHOTOS.Deluxe,
    highlight: 'Ban công riêng nhìn ra thành phố, bồn tắm đứng.',
  },
  Family: {
    description:
      'Hai giường đôi trong không gian 45 m², đủ chỗ cho gia đình 3–4 người. Có minibar và bàn làm việc, thuận tiện cho kỳ nghỉ có trẻ nhỏ.',
    promoPercent: 0, // % giảm giá (0 = không khuyến mãi). Chỉ bật khi khách sạn xác nhận chương trình thật.
    area: 45,
    bedType: '2 giường đôi',
    bedCount: 2,
    view: 'Hướng phố',
    amenities: ['wifi', 'ac', 'tv', 'minibar', 'desk', 'shower', 'safe'],
    images: ROOM_PHOTOS.Family,
    highlight: 'Hai giường đôi rộng rãi cho gia đình 3–4 người.',
  },
  Suite: {
    description:
      'Hạng phòng cao nhất với 65 m²: phòng khách riêng có sofa, phòng ngủ giường King, bồn tắm nằm và ban công nhìn toàn cảnh thành phố.',
    promoPercent: 0, // % giảm giá (0 = không khuyến mãi). Chỉ bật khi khách sạn xác nhận chương trình thật.
    area: 65,
    bedType: '1 giường King + sofa',
    bedCount: 1,
    view: 'Toàn cảnh thành phố',
    amenities: ['wifi', 'ac', 'tv', 'desk', 'minibar', 'living', 'bathtub', 'balcony', 'cityView', 'safe'],
    images: ROOM_PHOTOS.Suite,
    highlight: 'Phòng khách riêng, bồn tắm nằm, toàn cảnh thành phố.',
  },
};

// Loại phòng được giới thiệu ở trang chủ.
export const FEATURED_ROOM_TYPES = ['Deluxe', 'Family', 'Suite'];

// Số tiện nghi hiển thị tối đa trên Room Card.
export const CARD_AMENITY_LIMIT = 4;
