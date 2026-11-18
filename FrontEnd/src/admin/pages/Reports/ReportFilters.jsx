import { Download, FileText } from 'lucide-react';
import { Select } from '../../components/common/FormField.jsx';

const THIS_YEAR = new Date().getFullYear();
const YEARS = Array.from({ length: 5 }, (_, i) => THIS_YEAR - i);
const MONTHS = Array.from({ length: 12 }, (_, i) => ({ value: String(i + 1), label: `Tháng ${i + 1}` }));
const MODES = [
  { value: 'day', label: 'Theo ngày (trong tháng)' },
  { value: 'month', label: 'Theo tháng (trong năm)' },
  { value: 'year', label: 'Theo năm (5 năm gần nhất)' },
];

export const periodLabel = ({ mode, year, month }) => {
  if (mode === 'day') return `Tháng ${month}/${year}`;
  if (mode === 'month') return `Năm ${year}`;
  return `${year - 4} – ${year}`;
};

export const periodSlug = ({ mode, year, month }) =>
  mode === 'day' ? `${year}-${String(month).padStart(2, '0')}` : mode === 'month' ? `${year}` : `${year - 4}-${year}`;

// Bộ lọc theo ngày / tháng / năm dùng chung cho mọi báo cáo.
export default function ReportFilters({ filter, onChange }) {
  return (
    <div className="no-print card mb-5 grid grid-cols-1 gap-4 p-4 sm:grid-cols-3">
      <Select
        label="Kiểu thống kê"
        value={filter.mode}
        onChange={(e) => onChange({ mode: e.target.value })}
        options={MODES}
      />
      <Select
        label={filter.mode === 'year' ? 'Năm kết thúc' : 'Năm'}
        value={String(filter.year)}
        onChange={(e) => onChange({ year: Number(e.target.value) })}
        options={YEARS.map((y) => ({ value: String(y), label: String(y) }))}
      />
      <Select
        label="Tháng"
        value={String(filter.month)}
        onChange={(e) => onChange({ month: Number(e.target.value) })}
        options={MONTHS}
        disabled={filter.mode !== 'day'}
      />
    </div>
  );
}

// Nút xuất báo cáo: Excel (CSV mở được bằng Excel) và PDF (hộp thoại in của trình duyệt).
export function ExportButtons({ onExcel }) {
  return (
    <div className="no-print flex gap-2">
      <button type="button" className="btn-secondary" onClick={onExcel}>
        <Download size={16} /> Xuất Excel
      </button>
      <button type="button" className="btn-secondary" onClick={() => window.print()}>
        <FileText size={16} /> Xuất PDF
      </button>
    </div>
  );
}
