// Màn hình chờ trung tính khi tải chế độ Admin hoặc Khách hàng (không mang giao diện của bên nào).
export default function AppLoading() {
  return (
    <div role="status" aria-label="Đang tải" style={{ display: 'grid', placeItems: 'center', minHeight: '100vh', color: '#64748B', fontFamily: 'Inter, system-ui, sans-serif', fontSize: 14 }}>
      Đang tải...
    </div>
  );
}
