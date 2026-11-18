import { useMemo, useState } from 'react';
import { History, Pencil, Plus, Trash2, Users } from 'lucide-react';
import PageHeader from '../../components/common/PageHeader.jsx';
import SearchInput from '../../components/common/SearchInput.jsx';
import DataTable from '../../components/common/DataTable.jsx';
import ConfirmDialog from '../../components/common/ConfirmDialog.jsx';
import Avatar from '../../components/common/Avatar.jsx';
import { DataState } from '../../components/common/StateViews.jsx';
import CustomerForm from './CustomerForm.jsx';
import CustomerDetailModal from './CustomerDetailModal.jsx';
import useCrud from '../../hooks/useCrud.js';
import useFetch from '../../../shared/hooks/useFetch.js';
import { useToast } from '../../../shared/contexts/ToastContext.jsx';
import customerService from '../../../shared/services/customerService.js';
import bookingService from '../../../shared/services/bookingService.js';
import { NATIONALITIES } from '../../../shared/utils/constants.js';

export default function Customers() {
  const toast = useToast();
  const { items, loading, error, reload, create, update, remove } = useCrud(customerService);
  const bookingsQuery = useFetch(bookingService.getAll);

  const [search, setSearch] = useState('');
  const [nationality, setNationality] = useState('');
  const [formState, setFormState] = useState({ open: false, customer: null });
  const [detail, setDetail] = useState(null);
  const [deleteTarget, setDeleteTarget] = useState(null);
  const [deleting, setDeleting] = useState(false);

  const stayCount = useMemo(() => {
    const map = {};
    (bookingsQuery.data || [])
      .filter((b) => b.status === 'Checked-in' || b.status === 'Checked-out')
      .forEach((b) => {
        map[b.customerId] = (map[b.customerId] || 0) + 1;
      });
    return map;
  }, [bookingsQuery.data]);

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    return items.filter(
      (c) =>
        (!q ||
          c.fullName.toLowerCase().includes(q) ||
          c.id.toLowerCase().includes(q) ||
          c.phone.includes(q) ||
          c.email.toLowerCase().includes(q) ||
          c.idNumber.toLowerCase().includes(q)) &&
        (!nationality || c.nationality === nationality),
    );
  }, [items, search, nationality]);

  const hasFilter = search || nationality;
  const openEdit = (customer) => {
    setDetail(null);
    setFormState({ open: true, customer });
  };
  const closeForm = () => setFormState({ open: false, customer: null });

  const handleSubmit = async (payload) => {
    try {
      if (formState.customer) {
        await update(formState.customer.id, payload);
        toast.success(`Đã cập nhật khách hàng ${payload.fullName}`);
      } else {
        const created = await create(payload);
        toast.success(`Đã thêm khách hàng ${created.id}`);
      }
      closeForm();
    } catch (e) {
      toast.error(e.message);
    }
  };

  const askDelete = (customer) => {
    setDetail(null);
    setDeleteTarget(customer);
  };

  const confirmDelete = async () => {
    setDeleting(true);
    try {
      await remove(deleteTarget.id);
      toast.success(`Đã xóa khách hàng ${deleteTarget.fullName}`);
    } catch (e) {
      toast.error(e.message);
    } finally {
      setDeleting(false);
      setDeleteTarget(null);
    }
  };

  const columns = [
    { key: 'id', header: 'Mã KH', render: (c) => <span className="font-semibold text-navy-800">{c.id}</span> },
    {
      key: 'fullName',
      header: 'Họ và tên',
      render: (c) => (
        <div className="flex items-center gap-2.5">
          <Avatar name={c.fullName} size={32} />
          <div>
            <p className="font-medium text-slate-800">{c.fullName}</p>
            <p className="text-xs text-slate-400">{c.email}</p>
          </div>
        </div>
      ),
    },
    { key: 'phone', header: 'Số điện thoại', hideBelow: 'sm' },
    { key: 'idNumber', header: 'CCCD / Hộ chiếu', hideBelow: 'lg' },
    { key: 'nationality', header: 'Quốc tịch', hideBelow: 'md' },
    {
      key: 'stays',
      header: 'Lượt lưu trú',
      hideBelow: 'md',
      render: (c) => (bookingsQuery.data ? `${stayCount[c.id] || 0} lượt` : '…'),
    },
    {
      key: 'actions',
      header: '',
      className: 'text-right',
      render: (c) => (
        <div className="flex justify-end gap-1" onClick={(e) => e.stopPropagation()}>
          <button type="button" className="btn-icon" title="Lịch sử đặt phòng" onClick={() => setDetail(c)}>
            <History size={16} />
          </button>
          <button type="button" className="btn-icon" title="Chỉnh sửa" onClick={() => openEdit(c)}>
            <Pencil size={16} />
          </button>
          <button type="button" className="btn-icon hover:!bg-red-50 hover:!text-red-600" title="Xóa" onClick={() => askDelete(c)}>
            <Trash2 size={16} />
          </button>
        </div>
      ),
    },
  ];

  return (
    <div>
      <PageHeader title="Quản lý khách hàng" description={`${items.length} khách hàng`}>
        <button type="button" className="btn-primary" onClick={() => setFormState({ open: true, customer: null })}>
          <Plus size={16} /> Thêm khách hàng
        </button>
      </PageHeader>

      <div className="card overflow-hidden">
        <div className="grid grid-cols-1 gap-3 border-b border-slate-100 p-4 sm:grid-cols-3">
          <SearchInput value={search} onChange={setSearch} placeholder="Tìm tên, mã, SĐT, email, CCCD..." className="sm:col-span-2" />
          <select className="input" value={nationality} onChange={(e) => setNationality(e.target.value)} aria-label="Lọc theo quốc tịch">
            <option value="">Tất cả quốc tịch</option>
            {NATIONALITIES.map((n) => (
              <option key={n} value={n}>
                {n}
              </option>
            ))}
          </select>
        </div>

        <DataState
          loading={loading}
          error={error}
          onRetry={reload}
          isEmpty={filtered.length === 0}
          emptyProps={{
            icon: Users,
            title: hasFilter ? 'Không tìm thấy khách hàng' : 'Chưa có khách hàng',
            description: hasFilter ? 'Thử thay đổi từ khóa hoặc bộ lọc.' : 'Thêm khách hàng đầu tiên.',
            action: hasFilter ? (
              <button
                type="button"
                className="btn-secondary"
                onClick={() => {
                  setSearch('');
                  setNationality('');
                }}
              >
                Xóa bộ lọc
              </button>
            ) : undefined,
          }}
        >
          <DataTable columns={columns} data={filtered} onRowClick={setDetail} />
        </DataState>
      </div>

      <CustomerForm open={formState.open} customer={formState.customer} onClose={closeForm} onSubmit={handleSubmit} />
      <CustomerDetailModal customer={detail} onClose={() => setDetail(null)} onEdit={openEdit} onDelete={askDelete} />
      <ConfirmDialog
        open={Boolean(deleteTarget)}
        danger
        loading={deleting}
        title="Xóa khách hàng"
        message={
          <>
            Xóa khách hàng <b>{deleteTarget?.fullName}</b>? Hành động này không thể hoàn tác.
          </>
        }
        confirmText="Xóa khách hàng"
        onConfirm={confirmDelete}
        onCancel={() => setDeleteTarget(null)}
      />
    </div>
  );
}
