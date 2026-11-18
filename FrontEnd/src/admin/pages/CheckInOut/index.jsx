import { useMemo, useState } from 'react';
import { BedDouble, Clock, Eye, LogIn, LogOut, SprayCan } from 'lucide-react';
import PageHeader from '../../components/common/PageHeader.jsx';
import Tabs from '../../components/common/Tabs.jsx';
import Avatar from '../../components/common/Avatar.jsx';
import SearchInput from '../../components/common/SearchInput.jsx';
import { Badge } from '../../components/common/StatusBadge.jsx';
import { DataState } from '../../components/common/StateViews.jsx';
import CheckInModal from './CheckInModal.jsx';
import CheckOutModal from './CheckOutModal.jsx';
import useFetch from '../../../shared/hooks/useFetch.js';
import { useToast } from '../../../shared/contexts/ToastContext.jsx';
import { useSettings } from '../../../shared/contexts/SettingsContext.jsx';
import bookingService from '../../../shared/services/bookingService.js';
import roomService from '../../../shared/services/roomService.js';
import customerService from '../../../shared/services/customerService.js';
import serviceCatalogService from '../../services/serviceCatalogService.js';
import checkInOutService from '../../services/checkInOutService.js';
import { diffDays, todayStr } from '../../../shared/utils/dateUtils.js';
import { formatDate } from '../../../shared/utils/formatters.js';

const loadAll = async () => {
  const [bookings, rooms, customers, services] = await Promise.all([
    bookingService.getAll(),
    roomService.getAll(),
    customerService.getAll(),
    serviceCatalogService.getAll(),
  ]);
  return { bookings, rooms, customers, services };
};

function DueBadge({ date }) {
  const diff = diffDays(todayStr(), date);
  if (diff < 0) return <Badge className="bg-red-50 text-red-700 ring-red-600/20">Quá hạn {-diff} ngày</Badge>;
  if (diff === 0) return <Badge className="bg-amber-50 text-amber-700 ring-amber-600/20">Hôm nay</Badge>;
  return <Badge className="bg-slate-100 text-slate-600 ring-slate-500/20">Còn {diff} ngày</Badge>;
}

export default function CheckInOut() {
  const toast = useToast();
  const { hotel } = useSettings();
  const { data, loading, error, reload, refresh } = useFetch(loadAll);

  const [tab, setTab] = useState('in');
  const [search, setSearch] = useState('');
  const [checkInTarget, setCheckInTarget] = useState(null);
  const [checkOutTarget, setCheckOutTarget] = useState(null);
  const [busyRoom, setBusyRoom] = useState(null);

  const today = todayStr();
  const q = search.trim().toLowerCase();
  const match = (b) =>
    !q || b.customerName.toLowerCase().includes(q) || b.id.toLowerCase().includes(q) || b.roomCode.toLowerCase().includes(q);

  const lists = useMemo(() => {
    if (!data) return { checkIn: [], checkOut: [], cleaning: [] };
    return {
      checkIn: data.bookings.filter((b) => b.status === 'Confirmed').sort((a, b) => a.checkIn.localeCompare(b.checkIn)),
      checkOut: data.bookings.filter((b) => b.status === 'Checked-in').sort((a, b) => a.checkOut.localeCompare(b.checkOut)),
      cleaning: data.rooms.filter((r) => r.status === 'Cleaning'),
    };
  }, [data]);

  const customerOf = (b) => data.customers.find((c) => c.id === b.customerId);

  const doCheckIn = async (booking) => {
    try {
      await checkInOutService.checkIn(booking.id);
      toast.success(`Đã check-in ${booking.customerName} – phòng ${booking.roomCode}`);
      setCheckInTarget(null);
      refresh();
    } catch (e) {
      toast.error(e.message);
      throw e;
    }
  };

  const doCheckOut = async (booking, payload) => {
    try {
      const { invoice } = await checkInOutService.checkOut(booking.id, payload);
      toast.success(`Đã check-out phòng ${booking.roomCode}. Hóa đơn ${invoice.id} đã được tạo, phòng chuyển sang dọn dẹp.`);
      setCheckOutTarget(null);
      refresh();
    } catch (e) {
      toast.error(e.message);
      throw e;
    }
  };

  const markReady = async (room) => {
    setBusyRoom(room.id);
    try {
      await checkInOutService.markRoomReady(room.id);
      toast.success(`Phòng ${room.code} đã sẵn sàng`);
      await refresh();
    } catch (e) {
      toast.error(e.message);
    } finally {
      setBusyRoom(null);
    }
  };

  const tabs = [
    { key: 'in', label: `Cần check-in (${lists.checkIn.length})`, icon: LogIn },
    { key: 'out', label: `Cần check-out (${lists.checkOut.length})`, icon: LogOut },
    { key: 'clean', label: `Đang dọn dẹp (${lists.cleaning.length})`, icon: SprayCan },
  ];

  const renderBookingRow = (b, mode) => {
    const customer = data ? customerOf(b) : null;
    const dueDate = mode === 'in' ? b.checkIn : b.checkOut;
    const notReady = mode === 'in' && b.checkIn > today;
    return (
      <li key={b.id} className="flex flex-col gap-4 p-4 transition-colors hover:bg-sky-50/40 lg:flex-row lg:items-center">
        <div className="flex min-w-0 flex-1 items-center gap-3">
          <Avatar name={b.customerName} size={42} />
          <div className="min-w-0">
            <p className="truncate font-semibold text-slate-800">{b.customerName}</p>
            <p className="truncate text-xs text-slate-400">
              {b.id} · {customer?.phone || b.phone}
            </p>
          </div>
        </div>

        <div className="grid flex-[1.4] grid-cols-2 gap-x-6 gap-y-2 text-sm sm:grid-cols-4">
          <div>
            <p className="text-xs text-slate-400">Phòng</p>
            <p className="flex items-center gap-1 font-medium text-slate-700">
              <BedDouble size={14} className="text-slate-400" />
              {b.roomCode} <span className="text-xs font-normal text-slate-400">({b.roomType})</span>
            </p>
          </div>
          <div>
            <p className="text-xs text-slate-400">Nhận phòng</p>
            <p className="font-medium text-slate-700">
              {formatDate(b.checkIn)} <span className="text-xs font-normal text-slate-400">{hotel.checkInTime}</span>
            </p>
          </div>
          <div>
            <p className="text-xs text-slate-400">Trả phòng</p>
            <p className="font-medium text-slate-700">
              {formatDate(b.checkOut)} <span className="text-xs font-normal text-slate-400">{hotel.checkOutTime}</span>
            </p>
          </div>
          <div>
            <p className="text-xs text-slate-400">{mode === 'in' ? 'Hạn nhận phòng' : 'Hạn trả phòng'}</p>
            <DueBadge date={dueDate} />
          </div>
        </div>

        <div className="flex items-center gap-2 lg:justify-end">
          <button type="button" className="btn-icon" title="Xem thông tin" onClick={() => (mode === 'in' ? setCheckInTarget({ booking: b, viewOnly: true }) : setCheckOutTarget(b))}>
            <Eye size={17} />
          </button>
          {mode === 'in' ? (
            <button
              type="button"
              className="btn-primary"
              disabled={notReady}
              title={notReady ? 'Chưa đến ngày nhận phòng' : undefined}
              onClick={() => setCheckInTarget({ booking: b })}
            >
              <LogIn size={16} /> Check-in
            </button>
          ) : (
            <button type="button" className="btn-primary" onClick={() => setCheckOutTarget(b)}>
              <LogOut size={16} /> Check-out
            </button>
          )}
        </div>
      </li>
    );
  };

  const current = tab === 'in' ? lists.checkIn.filter(match) : tab === 'out' ? lists.checkOut.filter(match) : lists.cleaning;

  return (
    <div>
      <PageHeader title="Nhận phòng / Trả phòng" description="Xác nhận check-in, check-out và cập nhật trạng thái phòng.">
        <span className="flex items-center gap-1.5 rounded-lg bg-white px-3 py-2 text-sm text-slate-500 shadow-card">
          <Clock size={15} className="text-navy-500" />
          Nhận phòng từ {hotel.checkInTime} · Trả phòng trước {hotel.checkOutTime}
        </span>
      </PageHeader>

      <div className="card overflow-hidden">
        <Tabs tabs={tabs} active={tab} onChange={setTab} className="px-2" />
        {tab !== 'clean' && (
          <div className="border-b border-slate-100 p-4">
            <SearchInput value={search} onChange={setSearch} placeholder="Tìm tên khách, mã đặt phòng, phòng..." className="max-w-md" />
          </div>
        )}

        <DataState
          loading={loading}
          error={error}
          onRetry={reload}
          isEmpty={current.length === 0}
          emptyProps={{
            icon: tab === 'clean' ? SprayCan : tab === 'in' ? LogIn : LogOut,
            title:
              tab === 'in' ? 'Không có khách cần check-in' : tab === 'out' ? 'Không có khách cần check-out' : 'Không có phòng đang dọn dẹp',
            description: search ? 'Không có kết quả khớp từ khóa.' : 'Danh sách sẽ cập nhật khi có thay đổi.',
          }}
        >
          {tab === 'clean' ? (
            <ul className="divide-y divide-slate-100">
              {lists.cleaning.map((room) => (
                <li key={room.id} className="flex flex-wrap items-center justify-between gap-3 p-4 hover:bg-sky-50/40">
                  <div className="flex items-center gap-3">
                    <span className="flex h-11 w-11 items-center justify-center rounded-xl bg-cyan-50 text-cyan-600">
                      <SprayCan size={20} />
                    </span>
                    <div>
                      <p className="font-semibold text-slate-800">
                        {room.name} <span className="text-xs font-normal text-slate-400">({room.type})</span>
                      </p>
                      <p className="text-xs text-slate-400">Tầng {room.floor} · Đang dọn dẹp sau khi khách trả phòng</p>
                    </div>
                  </div>
                  <button type="button" className="btn-secondary" disabled={busyRoom === room.id} onClick={() => markReady(room)}>
                    Đánh dấu đã dọn xong
                  </button>
                </li>
              ))}
            </ul>
          ) : (
            <ul className="divide-y divide-slate-100">{current.map((b) => renderBookingRow(b, tab))}</ul>
          )}
        </DataState>
      </div>

      {data && (
        <>
          <CheckInModal
            target={checkInTarget}
            customer={checkInTarget ? customerOf(checkInTarget.booking) : null}
            onClose={() => setCheckInTarget(null)}
            onConfirm={doCheckIn}
          />
          <CheckOutModal
            booking={checkOutTarget}
            customer={checkOutTarget ? customerOf(checkOutTarget) : null}
            services={data.services.filter((s) => s.active)}
            onClose={() => setCheckOutTarget(null)}
            onConfirm={doCheckOut}
          />
        </>
      )}
    </div>
  );
}
