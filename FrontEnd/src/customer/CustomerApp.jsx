import { Navigate, Route, Routes } from 'react-router-dom';
import CustomerLayout from './layout/CustomerLayout.jsx';
import { customerRoutes } from './routes/customerRoutes.jsx';
import { CUSTOMER_ROUTES } from './routes/paths.js';

// Chế độ website khách hàng: layout riêng (Header + Footer), phiên đăng nhập khách riêng (trong CustomerLayout).
export default function CustomerApp() {
  return (
    <Routes>
      <Route element={<CustomerLayout />}>{customerRoutes}</Route>
      <Route path="*" element={<Navigate to={CUSTOMER_ROUTES.HOME} replace />} />
    </Routes>
  );
}
