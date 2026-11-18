// Một dòng "nhãn: giá trị" dùng trong các modal chi tiết.
export default function InfoRow({ label, children }) {
  return (
    <div className="flex flex-col gap-0.5 sm:flex-row sm:gap-4">
      <dt className="w-40 shrink-0 text-sm text-slate-500">{label}</dt>
      <dd className="text-sm font-medium text-slate-800">{children || '—'}</dd>
    </div>
  );
}
