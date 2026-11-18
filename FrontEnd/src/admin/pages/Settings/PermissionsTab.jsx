import { useState } from 'react';
import { Lock, RotateCcw, Save } from 'lucide-react';
import { useAuth } from '../../contexts/AuthContext.jsx';
import { useToast } from '../../../shared/contexts/ToastContext.jsx';
import { DEFAULT_PERMISSIONS, MODULES, ROLES } from '../../utils/permissions.js';

const EDITABLE_ROLES = Object.keys(ROLES).filter((r) => r !== 'admin');

const sameSet = (a, b) => a.length === b.length && a.every((x) => b.includes(x));

// Ma trận phân quyền giao diện: vai trò × chức năng (menu & trang).
export default function PermissionsTab() {
  const toast = useToast();
  const { permissions, updatePermissions, resetPermissions } = useAuth();
  const [draft, setDraft] = useState(() =>
    Object.fromEntries(EDITABLE_ROLES.map((r) => [r, [...(permissions[r] || [])]])),
  );

  const dirty = EDITABLE_ROLES.some((r) => !sameSet(draft[r], permissions[r] || []));

  const toggle = (role, key) =>
    setDraft((d) => ({
      ...d,
      [role]: d[role].includes(key) ? d[role].filter((k) => k !== key) : [...d[role], key],
    }));

  const save = () => {
    updatePermissions({ ...permissions, ...draft, admin: DEFAULT_PERMISSIONS.admin });
    toast.success('Đã lưu phân quyền. Thay đổi áp dụng ngay cho menu và các trang.');
  };

  const reset = () => {
    resetPermissions();
    setDraft(Object.fromEntries(EDITABLE_ROLES.map((r) => [r, [...DEFAULT_PERMISSIONS[r]]])));
    toast.info('Đã khôi phục phân quyền mặc định');
  };

  return (
    <div className="card overflow-hidden">
      <div className="p-5 sm:p-6">
        <h2 className="text-base font-semibold">Phân quyền giao diện theo vai trò</h2>
        <p className="text-sm text-slate-500">
          Chọn chức năng mà mỗi vai trò được phép truy cập. Chức năng không được cấp sẽ bị ẩn khỏi menu và bị chặn khi truy cập trực tiếp.
        </p>
      </div>

      <div className="overflow-x-auto">
        <table className="w-full min-w-[640px] text-left text-sm">
          <thead>
            <tr className="border-y border-slate-100 bg-slate-50/70 text-xs uppercase tracking-wide text-slate-500">
              <th className="px-4 py-3 font-semibold">Chức năng</th>
              <th className="px-4 py-3 text-center font-semibold">
                <span className="inline-flex items-center gap-1">
                  <Lock size={12} /> {ROLES.admin}
                </span>
              </th>
              {EDITABLE_ROLES.map((r) => (
                <th key={r} className="px-4 py-3 text-center font-semibold">
                  {ROLES[r]}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100">
            {MODULES.map((m) => (
              <tr key={m.key} className="transition-colors hover:bg-sky-50/40">
                <td className="px-4 py-3 font-medium text-slate-700">{m.label}</td>
                <td className="px-4 py-3 text-center">
                  <input type="checkbox" checked disabled className="h-4 w-4 accent-navy-700" aria-label={`${ROLES.admin} - ${m.label}`} />
                </td>
                {EDITABLE_ROLES.map((r) => (
                  <td key={r} className="px-4 py-3 text-center">
                    <input
                      type="checkbox"
                      checked={draft[r].includes(m.key)}
                      onChange={() => toggle(r, m.key)}
                      className="h-4 w-4 cursor-pointer accent-navy-700"
                      aria-label={`${ROLES[r]} - ${m.label}`}
                    />
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="flex flex-col-reverse items-stretch justify-between gap-3 border-t border-slate-100 bg-slate-50/60 px-5 py-3 sm:flex-row sm:items-center">
        <p className="text-xs text-slate-400">Quản trị viên luôn có toàn quyền và không thể chỉnh sửa.</p>
        <div className="flex gap-2">
          <button type="button" className="btn-secondary" onClick={reset}>
            <RotateCcw size={16} /> Mặc định
          </button>
          <button type="button" className="btn-primary" onClick={save} disabled={!dirty}>
            <Save size={16} /> Lưu phân quyền
          </button>
        </div>
      </div>
    </div>
  );
}
