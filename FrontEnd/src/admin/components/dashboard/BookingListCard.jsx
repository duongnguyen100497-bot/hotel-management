import { Link } from 'react-router-dom';
import { Inbox } from 'lucide-react';
import { BookingStatusBadge } from '../common/StatusBadge.jsx';
import Avatar from '../common/Avatar.jsx';
import { formatDate } from '../../../shared/utils/formatters.js';

/**
 * Bảng rút gọn trên Dashboard.
 * dateField: trường ngày hiển thị (checkIn | checkOut); showStatus: hiện trạng thái đặt phòng.
 */
export default function BookingListCard({ title, icon: Icon, rows, dateField = 'checkIn', dateLabel, showStatus = false, linkTo }) {
  return (
    <div className="card flex flex-col">
      <div className="flex items-center justify-between border-b border-slate-100 px-5 py-4">
        <h3 className="flex items-center gap-2 text-base font-semibold">
          {Icon && <Icon size={18} className="text-navy-500" />}
          {title}
        </h3>
        {linkTo && (
          <Link to={linkTo} className="text-xs font-medium text-navy-600 hover:underline">
            Xem tất cả
          </Link>
        )}
      </div>
      {rows.length === 0 ? (
        <div className="flex flex-1 flex-col items-center justify-center gap-2 px-4 py-10 text-slate-400">
          <Inbox size={24} />
          <p className="text-sm">Không có dữ liệu</p>
        </div>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full min-w-max text-left text-sm">
            <thead>
              <tr className="text-xs uppercase tracking-wide text-slate-400">
                <th className="px-5 py-2.5 font-medium">Khách hàng</th>
                <th className="px-3 py-2.5 font-medium">Phòng</th>
                <th className="px-3 py-2.5 font-medium">{dateLabel}</th>
                {showStatus && <th className="px-3 py-2.5 font-medium">Trạng thái</th>}
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {rows.map((b) => (
                <tr key={b.id} className="transition-colors hover:bg-sky-50/50">
                  <td className="px-5 py-2.5">
                    <div className="flex items-center gap-2.5">
                      <Avatar name={b.customerName} size={30} />
                      <div>
                        <p className="font-medium text-slate-800">{b.customerName}</p>
                        <p className="text-xs text-slate-400">{b.id}</p>
                      </div>
                    </div>
                  </td>
                  <td className="px-3 py-2.5 text-slate-600">
                    {b.roomCode}
                    <span className="ml-1 text-xs text-slate-400">({b.roomType})</span>
                  </td>
                  <td className="px-3 py-2.5 text-slate-600">{formatDate(b[dateField])}</td>
                  {showStatus && (
                    <td className="px-3 py-2.5">
                      <BookingStatusBadge status={b.status} />
                    </td>
                  )}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
