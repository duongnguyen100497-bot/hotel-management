import { useEffect, useRef } from 'react';

// Gọi handler khi click ra ngoài phần tử `ref`. Chỉ lắng nghe khi enabled = true.
export default function useClickOutside(ref, handler, enabled = true) {
  const handlerRef = useRef(handler);
  handlerRef.current = handler;

  useEffect(() => {
    if (!enabled) return undefined;
    const listener = (event) => {
      if (ref.current && !ref.current.contains(event.target)) handlerRef.current(event);
    };
    document.addEventListener('mousedown', listener);
    return () => document.removeEventListener('mousedown', listener);
  }, [ref, enabled]);
}
