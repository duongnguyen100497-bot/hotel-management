import { BedDouble, Users } from 'lucide-react';
import { ROOM_STATUS } from '../../../shared/utils/constants.js';
import { formatCurrency } from '../../../shared/utils/formatters.js';

// Chế độ Room Grid: thẻ phòng nhóm theo tầng, màu theo trạng thái.
export default function RoomGrid({ rooms, onSelect }) {
  const floors = [...new Set(rooms.map((r) => r.floor))].sort((a, b) => a - b);

  return (
    <div className="space-y-6 p-4 sm:p-5">
      {floors.map((floor) => (
        <section key={floor}>
          <h3 className="mb-3 flex items-center gap-2 text-sm font-semibold text-navy-800">
            <span className="rounded-md bg-navy-800 px-2 py-0.5 text-xs text-white">Tầng {floor}</span>
            <span className="text-xs font-normal text-slate-400">
              {rooms.filter((r) => r.floor === floor).length} phòng
            </span>
          </h3>
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-6">
            {rooms
              .filter((r) => r.floor === floor)
              .map((room) => {
                const meta = ROOM_STATUS[room.status];
                return (
                  <button
                    key={room.id}
                    type="button"
                    onClick={() => onSelect(room)}
                    className={`group rounded-xl border-2 p-3 text-left transition-all duration-200 hover:-translate-y-0.5 hover:shadow-card-hover ${meta.card}`}
                  >
                    <div className="flex items-center justify-between">
                      <span className="text-lg font-bold text-navy-900">{room.code}</span>
                      <BedDouble size={16} className="text-slate-400" />
                    </div>
                    <p className="mt-0.5 text-xs font-medium text-slate-600">{room.type}</p>
                    <p className="mt-2 text-xs text-slate-500">{formatCurrency(room.price)}</p>
                    <div className="mt-2 flex items-center justify-between">
                      <span className="flex items-center gap-1.5 text-xs font-medium text-slate-700">
                        <span className={`h-2 w-2 rounded-full ${meta.dot}`} />
                        {meta.label}
                      </span>
                      <span className="flex items-center gap-0.5 text-xs text-slate-400">
                        <Users size={12} />
                        {room.capacity}
                      </span>
                    </div>
                  </button>
                );
              })}
          </div>
        </section>
      ))}
    </div>
  );
}
