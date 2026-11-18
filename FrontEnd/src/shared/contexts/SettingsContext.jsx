import { createContext, useCallback, useContext, useMemo, useState } from 'react';
import { defaultHotelInfo } from '../services/mockData.js';

const SettingsContext = createContext(null);
const KEY = 'hms_hotel';

const readHotel = () => {
  try {
    const raw = localStorage.getItem(KEY);
    return raw ? { ...defaultHotelInfo, ...JSON.parse(raw) } : defaultHotelInfo;
  } catch {
    return defaultHotelInfo;
  }
};

// Thông tin khách sạn dùng chung (Sidebar, Login, hóa đơn...).
export function SettingsProvider({ children }) {
  const [hotel, setHotel] = useState(readHotel);

  const updateHotel = useCallback((patch) => {
    setHotel((prev) => {
      const next = { ...prev, ...patch };
      try {
        localStorage.setItem(KEY, JSON.stringify(next));
      } catch {
        /* bỏ qua nếu hết dung lượng lưu trữ */
      }
      return next;
    });
  }, []);

  const value = useMemo(() => ({ hotel, updateHotel }), [hotel, updateHotel]);
  return <SettingsContext.Provider value={value}>{children}</SettingsContext.Provider>;
}

export const useSettings = () => useContext(SettingsContext);
