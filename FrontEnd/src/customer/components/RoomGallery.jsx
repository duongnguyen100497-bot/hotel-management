import { useCallback, useEffect, useState } from 'react';
import { ChevronLeft, ChevronRight, Expand, X } from 'lucide-react';
import Photo from './Photo.jsx';

// Bộ sưu tập ảnh phòng + lightbox. `slides`: [{ src?, variant, alt }]
export default function RoomGallery({ slides }) {
  const [open, setOpen] = useState(-1);
  const count = slides.length;
  const go = useCallback((d) => setOpen((i) => (i < 0 ? i : (i + d + count) % count)), [count]);

  useEffect(() => {
    if (open < 0) return undefined;
    const onKey = (e) => {
      if (e.key === 'Escape') setOpen(-1);
      if (e.key === 'ArrowRight') go(1);
      if (e.key === 'ArrowLeft') go(-1);
    };
    document.addEventListener('keydown', onKey);
    document.body.style.overflow = 'hidden';
    return () => {
      document.removeEventListener('keydown', onKey);
      document.body.style.overflow = '';
    };
  }, [open, go]);

  const tile = (slide, i, className) => (
    <button
      key={i}
      type="button"
      onClick={() => setOpen(i)}
      className={`group relative block overflow-hidden rounded-3xl focus:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2 ${className}`}
      aria-label={`Xem ảnh lớn: ${slide.alt}`}
    >
      <Photo src={slide.src} variant={slide.variant} alt={slide.alt} className="h-full w-full" />
      {i === 0 && (
        <span className="absolute bottom-4 right-4 flex items-center gap-1.5 rounded-full bg-white/95 px-3 py-1.5 text-xs font-semibold text-brand shadow-soft">
          <Expand size={14} /> {count} ảnh
        </span>
      )}
    </button>
  );

  return (
    <>
      <div className="grid gap-3 md:grid-cols-[2fr_1fr] md:grid-rows-2">
        {tile(slides[0], 0, 'aspect-[16/10] md:row-span-2 md:aspect-auto md:min-h-[420px]')}
        {slides.slice(1, 3).map((s, i) => tile(s, i + 1, 'hidden aspect-[4/3] md:block md:aspect-auto'))}
      </div>

      {open >= 0 && (
        <div className="fixed inset-0 z-[60] flex flex-col bg-brand-dark/95" role="dialog" aria-modal="true" aria-label="Thư viện ảnh phòng">
          <div className="flex items-center justify-between px-4 py-3 text-white">
            <p className="text-sm">{open + 1} / {count} – {slides[open].alt}</p>
            <button type="button" onClick={() => setOpen(-1)} className="flex h-10 w-10 items-center justify-center rounded-full hover:bg-white/10" aria-label="Đóng">
              <X size={22} />
            </button>
          </div>
          <div className="relative flex flex-1 items-center justify-center px-4 pb-6">
            <button type="button" onClick={() => go(-1)} className="absolute left-3 z-10 flex h-11 w-11 items-center justify-center rounded-full bg-white/15 text-white hover:bg-white/25" aria-label="Ảnh trước">
              <ChevronLeft size={24} />
            </button>
            <Photo src={slides[open].src} variant={slides[open].variant} alt={slides[open].alt} className="aspect-[4/3] max-h-full w-full max-w-4xl rounded-3xl" />
            <button type="button" onClick={() => go(1)} className="absolute right-3 z-10 flex h-11 w-11 items-center justify-center rounded-full bg-white/15 text-white hover:bg-white/25" aria-label="Ảnh sau">
              <ChevronRight size={24} />
            </button>
          </div>
        </div>
      )}
    </>
  );
}
