import { Star } from 'lucide-react';

export default function StarRating({ value = 5, max = 5, size = 16 }) {
  return (
    <span className="inline-flex items-center gap-0.5" role="img" aria-label={`${value} trên ${max} sao`}>
      {Array.from({ length: max }, (_, i) => (
        <Star
          key={i}
          size={size}
          aria-hidden="true"
          className={i < value ? 'fill-warning text-warning' : 'fill-slate-200 text-slate-200'}
        />
      ))}
    </span>
  );
}
