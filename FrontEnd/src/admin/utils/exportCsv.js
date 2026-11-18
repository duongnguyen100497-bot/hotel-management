// Xuất CSV (mở được bằng Excel). Có BOM để hiển thị đúng tiếng Việt.
export function downloadCsv(filename, columns, rows) {
  const escape = (value) => {
    const s = String(value ?? '');
    return /[",\n;]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
  };
  const header = columns.map((c) => escape(c.header)).join(',');
  const body = rows
    .map((row) =>
      columns
        .map((c) => escape(typeof c.value === 'function' ? c.value(row) : row[c.key]))
        .join(','),
    )
    .join('\n');

  const blob = new Blob(['﻿' + header + '\n' + body], { type: 'text/csv;charset=utf-8;' });
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  URL.revokeObjectURL(url);
}
