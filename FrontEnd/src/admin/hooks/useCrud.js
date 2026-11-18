import { useCallback, useEffect, useState } from 'react';

// Hook CRUD chung: tách logic dữ liệu khỏi giao diện.
// `service` cần có getAll / create / update / remove (xem services/).
export default function useCrud(service) {
  const [items, setItems] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setItems(await service.getAll());
    } catch (e) {
      setError(e.message || 'Không thể tải dữ liệu');
    } finally {
      setLoading(false);
    }
  }, [service]);

  useEffect(() => {
    load();
  }, [load]);

  const create = useCallback(
    async (data) => {
      const item = await service.create(data);
      setItems((prev) => [item, ...prev]);
      return item;
    },
    [service],
  );

  const update = useCallback(
    async (id, data) => {
      const item = await service.update(id, data);
      setItems((prev) => prev.map((x) => (x.id === id ? item : x)));
      return item;
    },
    [service],
  );

  const remove = useCallback(
    async (id) => {
      await service.remove(id);
      setItems((prev) => prev.filter((x) => x.id !== id));
    },
    [service],
  );

  const replaceItem = useCallback((item) => {
    setItems((prev) => prev.map((x) => (x.id === item.id ? item : x)));
  }, []);

  return { items, loading, error, reload: load, create, update, remove, replaceItem, setItems };
}
