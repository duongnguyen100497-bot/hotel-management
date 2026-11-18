import { useCallback, useEffect, useState } from 'react';

// Gọi một hàm async (tham chiếu phải ổn định) và quản lý loading / error.
// reload(): hiện loading; refresh(): tải lại âm thầm (giữ nguyên dữ liệu đang hiển thị).
export default function useFetch(fn) {
  const [data, setData] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  const run = useCallback(
    async (silent) => {
      if (!silent) setLoading(true);
      setError(null);
      try {
        setData(await fn());
      } catch (e) {
        setError(e.message || 'Không thể tải dữ liệu');
      } finally {
        setLoading(false);
      }
    },
    [fn],
  );

  useEffect(() => {
    run(false);
  }, [run]);

  const reload = useCallback(() => run(false), [run]);
  const refresh = useCallback(() => run(true), [run]);

  return { data, loading, error, reload, refresh };
}
