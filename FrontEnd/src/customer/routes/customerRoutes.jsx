import { lazy } from 'react';
import { Route } from 'react-router-dom';
import { CUSTOMER_ROUTES as R } from './paths.js';
import RequireCustomerAuth from './RequireCustomerAuth.jsx';

const Home = lazy(() => import('../pages/Home.jsx'));
const Rooms = lazy(() => import('../pages/Rooms.jsx'));
const RoomDetail = lazy(() => import('../pages/RoomDetail.jsx'));
const Login = lazy(() => import('../pages/Login.jsx'));
const Register = lazy(() => import('../pages/Register.jsx'));
const Booking = lazy(() => import('../pages/Booking.jsx'));
const Account = lazy(() => import('../pages/Account.jsx'));
const MyBookings = lazy(() => import('../pages/MyBookings.jsx'));
const Contact = lazy(() => import('../pages/Contact.jsx'));

// Các route của website khách hàng, được gắn vào CustomerLayout trong CustomerApp.
export const customerRoutes = (
  <>
    <Route path={R.HOME} element={<Home />} />
    <Route path={R.ROOMS} element={<Rooms />} />
    <Route path={R.ROOM_DETAIL()} element={<RoomDetail />} />
    <Route path={R.LOGIN} element={<Login />} />
    <Route path={R.REGISTER} element={<Register />} />
    {/* Khách phải đăng nhập mới vào được các trang dưới đây */}
    <Route element={<RequireCustomerAuth />}>
      <Route path={R.BOOKING()} element={<Booking />} />
      <Route path={R.ACCOUNT} element={<Account />} />
      <Route path={R.MY_BOOKINGS} element={<MyBookings />} />
    </Route>
    <Route path={R.CONTACT} element={<Contact />} />
  </>
);
