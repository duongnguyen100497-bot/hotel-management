import { AuthProvider } from './contexts/AuthContext.jsx';
import AdminRoutes from './routes/AdminRoutes.jsx';

// Chế độ quản trị: có phiên đăng nhập nhân viên riêng, layout Sidebar riêng.
export default function AdminApp() {
  return (
    <AuthProvider>
      <AdminRoutes />
    </AuthProvider>
  );
}
