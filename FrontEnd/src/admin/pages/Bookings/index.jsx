import { useCallback, useMemo, useState } from 'react';
import { Ban, CalendarCheck, Eye, Pencil, Plus } from 'lucide-react';
import PageHeader from '../../components/common/PageHeader.jsx';
import SearchInput from '../../components/common/SearchInput.jsx';
import DataTable from '../../components/common/DataTable.jsx';
import ConfirmDialog from '../../components/common/ConfirmDialog.jsx';
import { DataState } from '../../components/common/StateViews.jsx';
import { BookingStatusBadge } from '../../components/common/StatusBadge.jsx';
import BookingForm from './BookingForm.jsx';
import BookingDetailModal from './BookingDetailModal.jsx';
import useCrud from '../../hooks/useCrud.js';
import useFetch from '../../../shared/hooks/useFetch.js';
import { useToast } from '../../../shared/contexts/ToastContext.jsx';
import bookingService from '../../../shared/services/bookingService.js';
import customerService from '../../../shared/services/customerService.js';
import roomService from '../../../shared/services/roomService.js';
import { BOOKING_STATUS, BOOKING_STATUS_LIST } from '../../../shared/utils/constants.js';
import { formatCurrency, formatDate } from '../../../shared/utils/formatters.js';

export default function Bookings() {
  const toast = useToast();
  const { items, loading, error, reload, create, update } = useCrud(bookingService);
  const customersQuery = useFetch(customerService.getAll);
  const roomsQuery = useFetch(useCallback(() => roomService.getAll(), []));

  const [search, setSearch] = useState('');
  const [status, setStatus] = useState('');
  const [from, setFrom] = useState('');
  const [to, setTo] = useState('');

  const [formState, setFormState] = useState({ open: false, booking: null });
  const [detail, setDetail] = useState(null);
  const [cancelTarget, setCancelTarget] = useState(null);
  const [cancelling, setCancelling] = useState(false);

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    return items.filter(
      (b) =>
        (!q ||
          b.id.toLowerCase().includes(q) ||
          b.customerName.toLowerCase().includes(q) ||
          b.phone.includes(q) ||
          b.roomCode.toLowerCase().includes(q)) &&
        (!status || b.status === status) &&
        // Lọc theo ngày: đặt phòng có khoảng lưu trú giao với [from, to]
        (!from || b.checkOut >= from) &&
        (!to || b.checkIn <= to),
    );
  }, [items, search, status, from, to]);

  const hasFilter = search || status || from || to;
  const clearFilters = () => {
    setSearch('');
    setStatus('');
    setFrom('');
    setTo('');
  };

  const openCreate = () => setFormState({ open: true, booking: null });
  const openEdit = (booking) => {
    setDetail(null);
    setFormState({ open: true, booking });
  };
  const closeForm = () => setFormState({ open: false, booking: null });

  const handleSubmit = async (payload) => {
    try {
      if (formState.booking) {
        await update(formState.booking.id, payload);
        toast.success(`Đã cập nhật đặt phòng ${formState.booking.id}`);
      } else {
        const created = await create(payload);
        toast.success(`Đã tạo đặt phòng ${created.id}`);
      }
      closeForm();
    } catch (e) {
      toast.error(e.message);
    }
  };

  const confirmCancel = async () => {
    setCancelling(true);
    try {
      await update(cancelTarget.id, { status: 'Cancelled' });
      toast.success(`Đã hủy đặt phòng ${cancelTarget.id}`);
    } catch (e) {
      toast.error(e.message);
    } finally {
      setCancelling(false);
      setCancelTarget(null);
    }
  };

  const columns = [
    { key: 'id', header: 'Mã đặt phòng', render: (b) => <span className="font-semibold text-navy-800">{b.id}</span> },
    {
      key: 'customerName',
      header: 'Khách hàng',
      render: (b) => (
        <div>
          <p className="font-medium text-slate-800">{b.customerName}</p>
          <p className="text-xs text-slate-400">{b.phone}</p>
        </div>
      ),
    },
    {
      key: 'room',
      header: 'Phòng',
      hideBelow: 'md',
      render: (b) => (
        <div>
          <p>{b.roomCode}</p>
          <p className="text-xs text-slate-400">{b.roomType}</p>
        </div>
      ),
    },
    { key: 'checkIn', header: 'Nhận phòng', hideBelow: 'lg', render: (b) => formatDate(b.checkIn) },
    { key: 'checkOut', header: 'Trả phòng', hideBelow: 'lg', render: (b) => formatDate(b.checkOut) },
    { key: 'total', header: 'Tổng tiền', hideBelow: 'sm', render: (b) => <span className="font-medium">{formatCurrency(b.total)}</span> },
    { key: 'status', header: 'Trạng thái', render: (b) => <BookingStatusBadge status={b.status} /> },
    {
      key: 'actions',
      header: '',
      className: 'text-right',
      render: (b) => (
        <div className="flex justify-end gap-1" onClick={(e) => e.stopPropagation()}>
          <button type="button" className="btn-icon" title="Xem chi tiết" onClick={() => setDetail(b)}>
            <Eye size={16} />
          </button>
          {['Pending', 'Confirmed', 'Checked-in'].includes(b.status) && (
            <button type="button" className="btn-icon" title="Chỉnh sửa" onClick={() => openEdit(b)}>
              <Pencil size={16} />
            </button>
          )}
          {['Pending', 'Confirmed'].includes(b.status) && (
            <button type="button" className="btn-icon hover:!bg-red-50 hover:!text-red-600" title="Hủy đặt phòng" onClick={() => setCancelTarget(b)}>
              <Ban size={16} />
            </button>
          )}
        </div>
      ),
    },
  ];

  const lookupsReady = customersQuery.data && roomsQuery.data;

  return (
    <div>
      <PageHeader title="Quản lý đặt phòng" description={`${items.length} đặt phòng`}>
        <button type="button" className="btn-primary" onClick={openCreate} disabled={!lookupsReady}>
          <Plus size={16} /> Tạo đặt phòng
        </button>
      </PageHeader>

      <div className="card overflow-hidden">
        <div className="grid grid-cols-1 gap-3 border-b border-slate-100 p-4 sm:grid-cols-2 xl:grid-cols-5">
          <SearchInput value={search} onChange={setSearch} placeholder="Tìm mã, tên khách, SĐT, phòng..." className="sm:col-span-2" />
          <select className="input" value={status} onChange={(e) => setStatus(e.target.value)} aria-label="Lọc theo trạng thái">
            <option value="">Tất cả trạng thái</option>
            {BOOKING_STATUS_LIST.map((s) => (
              <option key={s} value={s}>
                {BOOKING_STATUS[s].label}
              </option>
            ))}
          </select>
          <input type="date" className="input" value={from} max={to || undefined} onChange={(e) => setFrom(e.target.value)} aria-label="Từ ngày" title="Từ ngày" />
          <input type="date" className="input" value={to} min={from || undefined} onChange={(e) => setTo(e.target.value)} aria-label="Đến ngày" title="Đến ngày" />
        </div>

        <DataState
          loading={loading}
          error={error}
          onRetry={reload}
          isEmpty={filtered.length === 0}
          emptyProps={{
            icon: CalendarCheck,
            title: hasFilter ? 'Không tìm thấy đặt phòng phù hợp' : 'Chưa có đặt phòng',
            description: hasFilter ? 'Thử thay đổi từ khóa hoặc bộ lọc.' : 'Tạo đặt phòng đầu tiên cho khách.',
            action: hasFilter ? (
              <button type="button" className="btn-secondary" onClick={clearFilters}>
                Xóa bộ lọc
              </button>
            ) : undefined,
          }}
        >
          <DataTable columns={columns} data={filtered} onRowClick={setDetail} />
        </DataState>
      </div>

      {lookupsReady && (
        <BookingForm
          open={formState.open}
          booking={formState.booking}
          customers={customersQuery.data}
          rooms={roomsQuery.data}
          onClose={closeForm}
          onSubmit={handleSubmit}
        />
      )}
      <BookingDetailModal
        booking={detail}
        onClose={() => setDetail(null)}
        onEdit={openEdit}
        onCancel={(b) => {
          setDetail(null);
          setCancelTarget(b);
        }}
      />
      <ConfirmDialog
        open={Boolean(cancelTarget)}
        danger
        loading={cancelling}
        title="Hủy đặt phòng"
        message={
          <>
            Hủy đặt phòng <b>{cancelTarget?.id}</b> của <b>{cancelTarget?.customerName}</b>?
          </>
        }
        confirmText="Hủy đặt phòng"
        cancelText="Giữ lại"
        onConfirm={confirmCancel}
        onCancel={() => setCancelTarget(null)}
      />
    </div>
  );
}
