import { useState } from 'react';
import Pagination from './Pagination.jsx';
import { PAGE_SIZE } from '../../../shared/utils/constants.js';

const HIDE = {
  sm: 'hidden sm:table-cell',
  md: 'hidden md:table-cell',
  lg: 'hidden lg:table-cell',
  xl: 'hidden xl:table-cell',
};

/**
 * columns: [{ key, header, render?(row), className?, hideBelow?: 'sm'|'md'|'lg'|'xl' }]
 * Phân trang xử lý ngay tại Frontend.
 */
export default function DataTable({
  columns,
  data,
  rowKey = 'id',
  pageSize = PAGE_SIZE,
  onRowClick,
  emptyState,
}) {
  const [page, setPage] = useState(1);
  const totalPages = Math.max(1, Math.ceil(data.length / pageSize));
  const current = Math.min(page, totalPages);
  const rows = data.slice((current - 1) * pageSize, current * pageSize);

  if (data.length === 0) return emptyState || null;

  return (
    <div>
      <div className="overflow-x-auto">
        <table className="w-full min-w-max text-left text-sm">
          <thead>
            <tr className="border-b border-slate-100 bg-slate-50/70 text-xs uppercase tracking-wide text-slate-500">
              {columns.map((c) => (
                <th
                  key={c.key}
                  className={`px-4 py-3 font-semibold ${c.hideBelow ? HIDE[c.hideBelow] : ''} ${c.className || ''}`}
                >
                  {c.header}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100">
            {rows.map((row) => (
              <tr
                key={row[rowKey]}
                onClick={onRowClick ? () => onRowClick(row) : undefined}
                onKeyDown={
                  onRowClick
                    ? (e) => {
                        if (e.target === e.currentTarget && (e.key === 'Enter' || e.key === ' ')) {
                          e.preventDefault();
                          onRowClick(row);
                        }
                      }
                    : undefined
                }
                tabIndex={onRowClick ? 0 : undefined}
                className={`transition-colors hover:bg-sky-50/50 ${onRowClick ? 'cursor-pointer focus:bg-sky-50/70 focus:outline-none' : ''}`}
              >
                {columns.map((c) => (
                  <td
                    key={c.key}
                    className={`px-4 py-3 align-middle ${c.hideBelow ? HIDE[c.hideBelow] : ''} ${c.className || ''}`}
                  >
                    {c.render ? c.render(row) : row[c.key]}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <Pagination
        page={current}
        totalPages={totalPages}
        total={data.length}
        pageSize={pageSize}
        onPageChange={setPage}
      />
    </div>
  );
}
