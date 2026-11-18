// Ảnh thật dùng cho website khách hàng (nguồn: Unsplash, giấy phép miễn phí cho mục đích thương mại).
// Ảnh được nhúng trực tiếp từ images.unsplash.com nên cần có Internet. Nếu một ảnh không tải được,
// component <Photo> tự quay về ảnh minh họa SVG.
//
// Khi có ảnh của chính khách sạn: đặt file vào public/images/ và thay giá trị bằng '/images/ten-anh.jpg'.
const unsplash = (id, w = 1200) => `https://images.unsplash.com/photo-${id}?auto=format&fit=crop&w=${w}&q=75`;

export const ROOM_PHOTOS = {
  Standard: [unsplash('1631049307264-da0ec9d70304'), unsplash('1629140727571-9b5c6f6267b4'), unsplash('1618773928121-c32242e63f39')],
  Superior: [unsplash('1711059985570-4c32ed12a12c'), unsplash('1759328257280-28bdb3050be3'), unsplash('1562438668-bcf0ca6578f0')],
  Deluxe: [unsplash('1611892440504-42a792e24d32'), unsplash('1675409145919-277c0fc2aa7d'), unsplash('1770232274485-b35ee5092cbe')],
  Family: [unsplash('1680210851458-b7dc5685e06e'), unsplash('1710388663701-8c4aad200795'), unsplash('1697618009092-01b24159a55a')],
  Suite: [unsplash('1778205015308-1ed324aea202'), unsplash('1666880521091-704fe682c175'), unsplash('1690935986319-c11e6cae84f7')],
};

export const SITE_PHOTOS = {
  lobby: unsplash('1621293954908-907159247fc8'),
  lobbyLounge: unsplash('1573052905904-34ad8c27f0cc'),
  reception: unsplash('1759038085950-1234ca8f5fed'),
  pool: unsplash('1551882547-ff40c63fe5fa'),
  roomView: unsplash('1690935986319-c11e6cae84f7', 800),
  suite: unsplash('1666880521091-704fe682c175'),
  suiteBedroom: unsplash('1778205015308-1ed324aea202'),
  deluxeBedroom: unsplash('1675409145919-277c0fc2aa7d'),
};
