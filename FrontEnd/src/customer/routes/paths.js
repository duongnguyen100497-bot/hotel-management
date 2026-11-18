// Đường dẫn của website khách hàng (URL công khai).
export const CUSTOMER_ROUTES = {
  HOME: '/',
  ROOMS: '/rooms',
  ROOM_DETAIL: (id = ':id') => `/rooms/${id}`,
  BOOKING: (roomId = ':roomId') => `/booking/${roomId}`,
  LOGIN: '/login',
  REGISTER: '/register',
  ACCOUNT: '/account',
  MY_BOOKINGS: '/my-bookings',
  CONTACT: '/contact',
};
