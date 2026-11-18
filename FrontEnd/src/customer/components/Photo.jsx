import { useState } from 'react';
import { SCENES } from './scenes.jsx';

// Ảnh dùng trên website khách hàng.
// - Có `src` -> hiển thị ảnh thật (lazy-load).
// - Không có `src` (hoặc ảnh lỗi khi tải) -> hiển thị ảnh minh họa SVG theo `variant` (xem scenes.jsx), để giao diện hoàn chỉnh
//   khi chưa có ảnh thật. Thay ảnh thật bằng cách truyền src (xem data/customer/roomCatalog.js).
export default function Photo({ src, alt = '', variant = 'lobby', label, className = '', children }) {
  // Chỉ tự thêm `relative` khi nơi dùng chưa tự đặt kiểu định vị (vd: absolute inset-0 cho ảnh nền).
  const [failed, setFailed] = useState(false);
  const position = /(^|\s)(absolute|fixed|sticky|relative)(\s|$)/.test(className) ? '' : 'relative';
  if (src && !failed) {
    return (
      <div className={`${position} overflow-hidden bg-slate-100 ${className}`}>
        <img src={src} alt={alt} loading="lazy" onError={() => setFailed(true)} className="c-zoom h-full w-full object-cover" />
        {children}
      </div>
    );
  }

  const Scene = SCENES[variant] || SCENES.lobby;
  return (
    <div role="img" aria-label={alt || label || 'Ảnh minh họa'} className={`${position} isolate overflow-hidden bg-brand ${className}`}>
      <Scene />
      {label && (
        <span className="absolute bottom-3 left-3 rounded-full bg-brand-dark/60 px-2.5 py-1 text-xs font-medium text-white backdrop-blur">{label}</span>
      )}
      {children}
    </div>
  );
}
