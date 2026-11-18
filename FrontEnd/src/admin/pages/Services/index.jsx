import { useMemo, useState } from 'react';
import { ConciergeBell, Pencil, Plus, Trash2 } from 'lucide-react';
import PageHeader from '../../components/common/PageHeader.jsx';
import SearchInput from '../../components/common/SearchInput.jsx';
import DataTable from '../../components/common/DataTable.jsx';
import ConfirmDialog from '../../components/common/ConfirmDialog.jsx';
import Toggle from '../../components/common/Toggle.jsx';
import { ActiveBadge } from '../../components/common/StatusBadge.jsx';
import { DataState } from '../../components/common/StateViews.jsx';
import ServiceForm from './ServiceForm.jsx';
import useCrud from '../../hooks/useCrud.js';
import { useToast } from '../../../shared/contexts/ToastContext.jsx';
import serviceCatalogService from '../../services/serviceCatalogService.js';
import { SERVICE_TYPES } from '../../../shared/utils/constants.js';
import { formatCurrency } from '../../../shared/utils/formatters.js';
import { matchesSearch } from '../../utils/text.js';

export default function Services() {
  const toast = useToast();
  const { items, loading, error, reload, create, update, remove } = useCrud(serviceCatalogService);

  const [search, setSearch] = useState('');
  const [type, setType] = useState('');
  const [status, setStatus] = useState('');
  const [formState, setFormState] = useState({ open: false, service: null });
  const [deleteTarget, setDeleteTarget] = useState(null);
  const [deleting, setDeleting] = useState(false);
  const [togglingId, setTogglingId] = useState(null);

  const filtered = useMemo(
    () =>
      items.filter(
        (s) =>
          matchesSearch(s, ['id', 'name', 'type', 'description'], search) &&
          (!type || s.type === type) &&
          (!status || String(s.active) === status),
      ),
    [items, search, type, status],
  );

  const hasFilter = Boolean(search || type || status);
  const activeCount = items.filter((s) => s.active).length;
  const closeForm = () => setFormState({ open: false, service: null });

  const handleSubmit = async (payload) => {
    try {
      if (formState.service) {
        await update(formState.service.id, payload);
        toast.success(`Đã cập nhật dịch vụ ${payload.name}`);
      } else {
        const created = await create(payload);
        toast.success(`Đã thêm dịch vụ ${created.id}`);
      }
      closeForm();
    } catch (e) {
      toast.error(e.message);
    }
  };

  const handleToggle = async (service, active) => {
    setTogglingId(service.id);
    try {
      await update(service.id, { active });
      toast.success(`${service.name}: ${active ? 'đang hoạt động' : 'tạm ngưng'}`);
    } catch (e) {
      toast.error(e.message);
    } finally {
      setTogglingId(null);
    }
  };

  const confirmDelete = async () => {
    setDeleting(true);
    try {
      await remove(deleteTarget.id);
      toast.success(`Đã xóa dịch vụ ${deleteTarget.name}`);
    } catch (e) {
      toast.error(e.message);
    } finally {
      setDeleting(false);
      setDeleteTarget(null);
    }
  };

  const columns = [
    { key: 'id', header: 'Mã DV', render: (s) => <span className="font-semibold text-navy-800">{s.id}</span> },
    {
      key: 'name',
      header: 'Dịch vụ',
      render: (s) => (
        <div className="max-w-xs">
          <p className="font-medium text-slate-800">{s.name}</p>
          <p className="truncate text-xs text-slate-400">{s.description}</p>
        </div>
      ),
    },
    {
      key: 'type',
      header: 'Loại',
      hideBelow: 'sm',
      render: (s) => <span className="rounded-full bg-sky-50 px-2.5 py-0.5 text-xs font-medium text-navy-700">{s.type}</span>,
    },
    { key: 'price', header: 'Giá', render: (s) => <span className="font-medium">{formatCurrency(s.price)}</span> },
    {
      key: 'active',
      header: 'Trạng thái',
      render: (s) => (
        <div className="flex items-center gap-2" onClick={(e) => e.stopPropagation()}>
          <Toggle checked={s.active} disabled={togglingId === s.id} onChange={(v) => handleToggle(s, v)} label={`Bật/tắt ${s.name}`} />
          <span className="hidden lg:inline">
            <ActiveBadge active={s.active} />
          </span>
        </div>
      ),
    },
    {
      key: 'actions',
      header: '',
      className: 'text-right',
      render: (s) => (
        <div className="flex justify-end gap-1" onClick={(e) => e.stopPropagation()}>
          <button type="button" className="btn-icon" title="Chỉnh sửa" onClick={() => setFormState({ open: true, service: s })}>
            <Pencil size={16} />
          </button>
          <button type="button" className="btn-icon hover:!bg-red-50 hover:!text-red-600" title="Xóa" onClick={() => setDeleteTarget(s)}>
            <Trash2 size={16} />
          </button>
        </div>
      ),
    },
  ];

  return (
    <div>
      <PageHeader title="Quản lý dịch vụ" description={`${items.length} dịch vụ · ${activeCount} đang hoạt động`}>
        <button type="button" className="btn-primary" onClick={() => setFormState({ open: true, service: null })}>
          <Plus size={16} /> Thêm dịch vụ
        </button>
      </PageHeader>

      <div className="card overflow-hidden">
        <div className="grid grid-cols-1 gap-3 border-b border-slate-100 p-4 sm:grid-cols-2 xl:grid-cols-4">
          <SearchInput value={search} onChange={setSearch} placeholder="Tìm tên, mã, mô tả..." className="sm:col-span-2" />
          <select className="input" value={type} onChange={(e) => setType(e.target.value)} aria-label="Lọc theo loại">
            <option value="">Tất cả loại</option>
            {SERVICE_TYPES.map((t) => (
              <option key={t} value={t}>
                {t}
              </option>
            ))}
          </select>
          <select className="input" value={status} onChange={(e) => setStatus(e.target.value)} aria-label="Lọc theo trạng thái">
            <option value="">Tất cả trạng thái</option>
            <option value="true">Đang hoạt động</option>
            <option value="false">Tạm ngưng</option>
          </select>
        </div>

        <DataState
          loading={loading}
          error={error}
          onRetry={reload}
          isEmpty={filtered.length === 0}
          emptyProps={{
            icon: ConciergeBell,
            title: hasFilter ? 'Không tìm thấy dịch vụ' : 'Chưa có dịch vụ',
            description: hasFilter ? 'Thử thay đổi từ khóa hoặc bộ lọc.' : 'Thêm dịch vụ đầu tiên của khách sạn.',
            action: hasFilter ? (
              <button
                type="button"
                className="btn-secondary"
                onClick={() => {
                  setSearch('');
                  setType('');
                  setStatus('');
                }}
              >
                Xóa bộ lọc
              </button>
            ) : undefined,
          }}
        >
          <DataTable columns={columns} data={filtered} />
        </DataState>
      </div>

      <ServiceForm open={formState.open} service={formState.service} onClose={closeForm} onSubmit={handleSubmit} />
      <ConfirmDialog
        open={Boolean(deleteTarget)}
        danger
        loading={deleting}
        title="Xóa dịch vụ"
        message={
          <>
            Xóa dịch vụ <b>{deleteTarget?.name}</b>? Hóa đơn đã lập trước đó không bị ảnh hưởng.
          </>
        }
        confirmText="Xóa dịch vụ"
        onConfirm={confirmDelete}
        onCancel={() => setDeleteTarget(null)}
      />
    </div>
  );
}
