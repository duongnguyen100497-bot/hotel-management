import { Bath, DoorOpen, LampDesk, Martini, ShieldCheck, ShowerHead, Snowflake, Sofa, Tv, Wifi, Building2 } from 'lucide-react';

// Tiện nghi trong phòng (hiển thị ở Room Card, Room Detail, bộ lọc).
// Khóa (key) là định danh ổn định để lọc; nhãn/biểu tượng chỉ phục vụ hiển thị.
export const ROOM_AMENITIES = {
  wifi: { label: 'Wi-Fi miễn phí', icon: Wifi },
  ac: { label: 'Điều hòa', icon: Snowflake },
  tv: { label: 'Smart TV', icon: Tv },
  minibar: { label: 'Minibar', icon: Martini },
  desk: { label: 'Bàn làm việc', icon: LampDesk },
  balcony: { label: 'Ban công riêng', icon: DoorOpen },
  shower: { label: 'Vòi sen', icon: ShowerHead },
  bathtub: { label: 'Bồn tắm', icon: Bath },
  living: { label: 'Phòng khách riêng', icon: Sofa },
  safe: { label: 'Két an toàn', icon: ShieldCheck },
  cityView: { label: 'Tầm nhìn thành phố', icon: Building2 },
};

export const ROOM_AMENITY_LIST = Object.entries(ROOM_AMENITIES).map(([key, v]) => ({ key, ...v }));
