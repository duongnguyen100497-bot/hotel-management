import { useMemo, useState } from 'react';
import { Pencil, Plus, Trash2, UserCog } from 'lucide-react';
import PageHeader from '../../components/common/PageHeader.jsx';
import SearchInput from '../../components/common/SearchInput.jsx';
import DataTable from '../../components/common/DataTable.jsx';
import ConfirmDialog from '../../components/common/ConfirmDialog.jsx';
import Avatar from '../../components/common/Avatar.jsx';
import { EmployeeStatusBadge } from '../../components/common/StatusBadge.jsx';
import { DataState } from '../../components/common/StateViews.jsx';
import EmployeeForm from './EmployeeForm.jsx';
import useCrud from '../../hooks/useCrud.js';
import { useAuth } from '../../contexts/AuthContext.jsx';
import { useToast } from '../../../shared/contexts/ToastContext.jsx';
import employeeService from '../../services/employeeService.js';
import { EMPLOYEE_STATUS_LIST, POSITIONS } from '../../../shared/utils/constants.js';
import { matchesSearch } from '../../utils/text.js';

export default function Employees() {
  const toast = useToast();
  const { user } = useAuth();
  const { items, loading, error, reload, create, update, remove } = useCrud(employeeService);

  const [search, setSearch] = useState('');
  const [position, setPosition] = useState('');
  const [status, setStatus] = useState('');
  const [formState, setFormState] = useState({ open: false, employee: null });
  const [deleteTarget, setDeleteTarget] = useState(null);
  const [deleting, setDeleting] = useState(false);

  const filtered = useMemo(
    () =>
      items.filter(
        (e) =>
          matchesSearch(e, ['id', 'fullName', 'phone', 'email', 'username'], search) &&
          (!position || e.position === position) &&
          (!status || e.status === status),
      ),
    [items, search, position, status],
  );

  const hasFilter = Boolean(search || position || status);
  const working = items.filter((e) => e.status === 'Đang làm việc').length;
  const closeForm = () => setFormState({ open: false, employee: null });

  const handleSubmit = async (payload) => {
    try {
      if (formState.employee) {
        await update(formState.employee.id, payload);
        toast.success(`Đã cập nhật nhân viên ${payload.fullName}`);
      } else {
        const created = await create(payload);
        toast.success(`Đã thêm nhân viên ${created.id}`);
      }
      closeForm();
    } catch (e) {
      toast.error(e.message);
    }
  };

  const askDelete = (employee) => {
    if (employee.username === user?.username) {
      toast.error('Bạn không thể xóa tài khoản đang đăng nhập');
      return;
    }
    setDeleteTarget(employee);
  };

  const confirmDelete = async () => {
    setDeleting(true);
    try {
      await remove(deleteTarget.id);
      toast.success(`Đã xóa nhân viên ${deleteTarget.fullName}`);
    } catch (e) {
      toast.error(e.message);
    } finally {
      setDeleting(false);
      setDeleteTarget(null);
    }
  };

  const columns = [
    { key: 'id', header: 'Mã NV', render: (e) => <span className="font-semibold text-navy-800">{e.id}</span> },
    {
      key: 'fullName',
      header: 'Họ và tên',
      render: (e) => (
        <div className="flex items-center gap-2.5">
          <Avatar name={e.fullName} size={32} />
          <div>
            <p className="font-medium text-slate-800">{e.fullName}</p>
            <p className="text-xs text-slate-400">{e.email}</p>
          </div>
        </div>
      ),
    },
    { key: 'phone', header: 'Số điện thoại', hideBelow: 'sm' },
    { key: 'position', header: 'Chức vụ', hideBelow: 'md' },
    {
      key: 'username',
      header: 'Tài khoản',
      hideBelow: 'lg',
      render: (e) => <code className="rounded bg-slate-100 px-1.5 py-0.5 text-xs text-slate-600">{e.username}</code>,
    },
    { key: 'status', header: 'Trạng thái', render: (e) => <EmployeeStatusBadge status={e.status} /> },
    {
      key: 'actions',
      header: '',
      className: 'text-right',
      render: (e) => (
        <div className="flex justify-end gap-1">
          <button type="button" className="btn-icon" title="Chỉnh sửa" onClick={() => setFormState({ open: true, employee: e })}>
            <Pencil size={16} />
          </button>
          <button type="button" className="btn-icon hover:!bg-red-50 hover:!text-red-600" title="Xóa" onClick={() => askDelete(e)}>
            <Trash2 size={16} />
          </button>
        </div>
      ),
    },
  ];

  return (
    <div>
      <PageHeader title="Quản lý nhân viên" description={`${items.length} nhân viên · ${working} đang làm việc`}>
        <button type="button" className="btn-primary" onClick={() => setFormState({ open: true, employee: null })}>
          <Plus size={16} /> Thêm nhân viên
        </button>
      </PageHeader>

      <div className="card overflow-hidden">
        <div className="grid grid-cols-1 gap-3 border-b border-slate-100 p-4 sm:grid-cols-2 xl:grid-cols-4">
          <SearchInput value={search} onChange={setSearch} placeholder="Tìm tên, mã, SĐT, email, tài khoản..." className="sm:col-span-2" />
          <select className="input" value={position} onChange={(e) => setPosition(e.target.value)} aria-label="Lọc theo chức vụ">
            <option value="">Tất cả chức vụ</option>
            {POSITIONS.map((p) => (
              <option key={p} value={p}>
                {p}
              </option>
            ))}
          </select>
          <select className="input" value={status} onChange={(e) => setStatus(e.target.value)} aria-label="Lọc theo trạng thái">
            <option value="">Tất cả trạng thái</option>
            {EMPLOYEE_STATUS_LIST.map((s) => (
              <option key={s} value={s}>
                {s}
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
            icon: UserCog,
            title: hasFilter ? 'Không tìm thấy nhân viên' : 'Chưa có nhân viên',
            description: hasFilter ? 'Thử thay đổi từ khóa hoặc bộ lọc.' : 'Thêm nhân viên đầu tiên.',
            action: hasFilter ? (
              <button
                type="button"
                className="btn-secondary"
                onClick={() => {
                  setSearch('');
                  setPosition('');
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

      <EmployeeForm open={formState.open} employee={formState.employee} onClose={closeForm} onSubmit={handleSubmit} />
      <ConfirmDialog
        open={Boolean(deleteTarget)}
        danger
        loading={deleting}
        title="Xóa nhân viên"
        message={
          <>
            Xóa nhân viên <b>{deleteTarget?.fullName}</b>? Hành động này không thể hoàn tác.
          </>
        }
        confirmText="Xóa nhân viên"
        onConfirm={confirmDelete}
        onCancel={() => setDeleteTarget(null)}
      />
    </div>
  );
}
