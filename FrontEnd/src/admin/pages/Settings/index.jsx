import { useState } from 'react';
import { Building2, KeyRound, ShieldCheck, UserRound } from 'lucide-react';
import PageHeader from '../../components/common/PageHeader.jsx';
import Tabs from '../../components/common/Tabs.jsx';
import HotelInfoTab from './HotelInfoTab.jsx';
import AccountTab from './AccountTab.jsx';
import PasswordTab from './PasswordTab.jsx';
import PermissionsTab from './PermissionsTab.jsx';
import { useAuth } from '../../contexts/AuthContext.jsx';

export default function Settings() {
  const { user } = useAuth();
  const isAdmin = user.role === 'admin';
  const canEditHotel = isAdmin || user.role === 'manager';

  // Thông tin khách sạn: quản trị viên / quản lý. Phân quyền: chỉ quản trị viên.
  const tabs = [
    canEditHotel && { key: 'hotel', label: 'Thông tin khách sạn', icon: Building2 },
    { key: 'account', label: 'Tài khoản', icon: UserRound },
    { key: 'password', label: 'Đổi mật khẩu', icon: KeyRound },
    isAdmin && { key: 'permissions', label: 'Phân quyền', icon: ShieldCheck },
  ].filter(Boolean);

  const [tab, setTab] = useState(tabs[0].key);

  return (
    <div>
      <PageHeader title="Cài đặt hệ thống" description="Thông tin khách sạn, tài khoản và phân quyền." />
      <Tabs tabs={tabs} active={tab} onChange={setTab} className="mb-5" />
      {tab === 'hotel' && <HotelInfoTab />}
      {tab === 'account' && <AccountTab />}
      {tab === 'password' && <PasswordTab />}
      {tab === 'permissions' && <PermissionsTab />}
    </div>
  );
}
