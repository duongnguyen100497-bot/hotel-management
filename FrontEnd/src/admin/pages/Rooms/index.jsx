import { useMemo, useState } from 'react';
import { BedDouble, LayoutGrid, List, Pencil, Plus, Trash2, Eye } from 'lucide-react';
import PageHeader from '../../components/common/PageHeader.jsx';
import SearchInput from '../../components/common/SearchInput.jsx';
import DataTable from '../../components/common/DataTable.jsx';
import ConfirmDialog from '../../components/common/ConfirmDialog.jsx';
import { DataState } from '../../components/common/StateViews.jsx';
import { RoomStatusBadge } from '../../components/common/StatusBadge.jsx';
import RoomForm from './RoomForm.jsx';
import RoomGrid from './RoomGrid.jsx';
import RoomDetailModal from './RoomDetailModal.jsx';
import useCrud from '../../hooks/useCrud.js';
import { useToast } from '../../../shared/contexts/ToastContext.jsx';
import roomService from '../../../shared/services/roomService.js';
import { ROOM_STATUS, ROOM_STATUS_LIST, ROOM_TYPES } from '../../../shared/utils/constants.js';
import { formatCurrency } from '../../../shared/utils/formatters.js';

export default function Rooms() {
  const toast = useToast();
  const { items, loading, error, reload, create, update, remove } = useCrud(roomService);

  const [view, setView] = useState('table');
  const [search, setSearch] = useState('');
  const [floor, setFloor] = useState('');
  const [type, setType] = useState('');
  const [status, setStatus] = useState('');

  const [formState, setFormState] = useState({ open: false, room: null });
  const [detailRoom, setDetailRoom] = useState(null);
  const [deleteTarget, setDeleteTarget] = useState(null);
  const [deleting, setDeleting] = useState(false);

  const floors = useMemo(() => [...new Set(items.map((r) => r.floor))].sort((a, b) => a - b), [items]);

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    return items.filter(
      (r) =>
        (!q || r.code.toLowerCase().includes(q) || r.name.toLowerCase().includes(q)) &&
        (!floor || r.floor === Number(floor)) &&
        (!type || r.type === type) &&
        (!status || r.status === status),
    );
  }, [items, search, floor, type, status]);

  const statusCounts = useMemo(
    () => Object.fromEntries(ROOM_STATUS_LIST.map((s) => [s, items.filter((r) => r.status === s).length])),
    [items],
  );

  const hasFilter = search || floor || type || status;
  const clearFilters = () => {
    setSearch('');
    setFloor('');
    setType('');
    setStatus('');
  };

  const openCreate = () => setFormState({ open: true, room: null });
  const openEdit = (room) => {
    setDetailRoom(null);
    setFormState({ open: true, room });
  };
  const closeForm = () => setFormState({ open: false, room: null });

  const handleSubmit = async (values) => {
    try {
      if (formState.room) {
        await update(formState.room.id, values);
        toast.success(`Đã cập nhật ${values.name}`);
      } else {
        await create(values);
        toast.success(`Đã thêm ${values.name}`);
      }
      closeForm();
    } catch (e) {
      toast.error(e.message);
    }
  };

  const handleChangeStatus = async (room, next) => {
    try {
      const updated = await update(room.id, { status: next });
      setDetailRoom(updated);
      toast.success(`${room.name}: ${ROOM_STATUS[next].label}`);
    } catch (e) {
      toast.error(e.message);
    }
  };

  const askDelete = (room) => {
    setDetailRoom(null);
    setDeleteTarget(room);
  };

  const confirmDelete = async () => {
    setDeleting(true);
    try {
      await remove(deleteTarget.id);
      toast.success(`Đã xóa ${deleteTarget.name}`);
      setDeleteTarget(null);
    } catch (e) {
      toast.error(e.message);
      setDeleteTarget(null);
    } finally {
      setDeleting(false);
    }
  };

  const columns = [
    { key: 'code', header: 'Mã phòng', render: (r) => <span className="font-semibold text-navy-800">{r.code}</span> },
    { key: 'name', header: 'Tên phòng' },
    { key: 'type', header: 'Loại phòng', hideBelow: 'sm' },
    { key: 'price', header: 'Giá phòng', hideBelow: 'md', render: (r) => formatCurrency(r.price) },
    { key: 'floor', header: 'Tầng', hideBelow: 'md', render: (r) => `Tầng ${r.floor}` },
    { key: 'status', header: 'Trạng thái', render: (r) => <RoomStatusBadge status={r.status} /> },
    {
      key: 'actions',
      header: '',
      className: 'text-right',
      render: (r) => (
        <div className="flex justify-end gap-1" onClick={(e) => e.stopPropagation()}>
          <button type="button" className="btn-icon" title="Xem chi tiết" onClick={() => setDetailRoom(r)}>
            <Eye size={16} />
          </button>
          <button type="button" className="btn-icon" title="Chỉnh sửa" onClick={() => openEdit(r)}>
            <Pencil size={16} />
          </button>
          <button type="button" className="btn-icon hover:!bg-red-50 hover:!text-red-600" title="Xóa" onClick={() => askDelete(r)}>
            <Trash2 size={16} />
          </button>
        </div>
      ),
    },
  ];

  return (
    <div>
      <PageHeader title="Quản lý phòng" description={`${items.length} phòng trong hệ thống`}>
        <div className="flex rounded-lg bg-slate-100 p-0.5">
          {[
            { key: 'table', icon: List, label: 'Danh sách' },
            { key: 'grid', icon: LayoutGrid, label: 'Room Grid' },
          ].map(({ key, icon: Icon, label }) => (
            <button
              key={key}
              type="button"
              onClick={() => setView(key)}
              className={`flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm font-medium transition-colors ${
                view === key ? 'bg-white text-navy-800 shadow-sm' : 'text-slate-500 hover:text-navy-700'
              }`}
            >
              <Icon size={15} />
              <span className="hidden sm:inline">{label}</span>
            </button>
          ))}
        </div>
        <button type="button" className="btn-primary" onClick={openCreate}>
          <Plus size={16} /> Thêm phòng
        </button>
      </PageHeader>

      {/* Chú thích trạng thái (bấm để lọc nhanh) */}
      <div className="mb-4 flex flex-wrap gap-2">
        {ROOM_STATUS_LIST.map((s) => (
          <button
            key={s}
            type="button"
            onClick={() => setStatus(status === s ? '' : s)}
            className={`flex items-center gap-2 rounded-full border px-3 py-1.5 text-xs font-medium transition-colors ${
              status === s ? 'border-navy-600 bg-navy-800 text-white' : 'border-slate-200 bg-white text-slate-600 hover:border-navy-300'
            }`}
          >
            <span className={`h-2 w-2 rounded-full ${ROOM_STATUS[s].dot}`} />
            {ROOM_STATUS[s].label}
            <span className={status === s ? 'text-sky-200' : 'text-slate-400'}>{statusCounts[s]}</span>
          </button>
        ))}
      </div>

      <div className="card overflow-hidden">
        <div className="grid grid-cols-1 gap-3 border-b border-slate-100 p-4 sm:grid-cols-2 xl:grid-cols-5">
          <SearchInput value={search} onChange={setSearch} placeholder="Tìm mã hoặc tên phòng..." className="sm:col-span-2" />
          <select className="input" value={floor} onChange={(e) => setFloor(e.target.value)} aria-label="Lọc theo tầng">
            <option value="">Tất cả tầng</option>
            {floors.map((f) => (
              <option key={f} value={f}>
                Tầng {f}
              </option>
            ))}
          </select>
          <select className="input" value={type} onChange={(e) => setType(e.target.value)} aria-label="Lọc theo loại phòng">
            <option value="">Tất cả loại phòng</option>
            {ROOM_TYPES.map((t) => (
              <option key={t} value={t}>
                {t}
              </option>
            ))}
          </select>
          <select className="input" value={status} onChange={(e) => setStatus(e.target.value)} aria-label="Lọc theo trạng thái">
            <option value="">Tất cả trạng thái</option>
            {ROOM_STATUS_LIST.map((s) => (
              <option key={s} value={s}>
                {ROOM_STATUS[s].label}
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
            icon: BedDouble,
            title: hasFilter ? 'Không tìm thấy phòng phù hợp' : 'Chưa có phòng nào',
            description: hasFilter ? 'Thử thay đổi từ khóa hoặc bộ lọc.' : 'Bắt đầu bằng cách thêm phòng đầu tiên.',
            action: hasFilter ? (
              <button type="button" className="btn-secondary" onClick={clearFilters}>
                Xóa bộ lọc
              </button>
            ) : (
              <button type="button" className="btn-primary" onClick={openCreate}>
                <Plus size={16} /> Thêm phòng
              </button>
            ),
          }}
        >
          {view === 'table' ? (
            <DataTable columns={columns} data={filtered} onRowClick={setDetailRoom} />
          ) : (
            <RoomGrid rooms={filtered} onSelect={setDetailRoom} />
          )}
        </DataState>
      </div>

      <RoomForm open={formState.open} room={formState.room} onClose={closeForm} onSubmit={handleSubmit} />
      <RoomDetailModal
        room={detailRoom}
        onClose={() => setDetailRoom(null)}
        onEdit={openEdit}
        onDelete={askDelete}
        onChangeStatus={handleChangeStatus}
      />
      <ConfirmDialog
        open={Boolean(deleteTarget)}
        danger
        loading={deleting}
        title="Xóa phòng"
        message={
          <>
            Bạn có chắc muốn xóa <b>{deleteTarget?.name}</b>? Hành động này không thể hoàn tác.
          </>
        }
        confirmText="Xóa phòng"
        onConfirm={confirmDelete}
        onCancel={() => setDeleteTarget(null)}
      />
    </div>
  );
}
