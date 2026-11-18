import axios from 'axios';

// Axios instance dùng chung. Khi có Backend chỉ cần đặt VITE_API_BASE_URL.
const api = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api',
  timeout: 15000,
  headers: { 'Content-Type': 'application/json' },
});

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('hms_token');
  if (token) config.headers.Authorization = `Bearer ${token}`;
  return config;
});

api.interceptors.response.use(
  (response) => response,
  (error) => Promise.reject(error),
);

export const USE_MOCK = import.meta.env.VITE_USE_MOCK !== 'false';

// Giả lập độ trễ mạng cho Mock Data (để thấy trạng thái loading).
export const delay = (ms = 400) => new Promise((resolve) => setTimeout(resolve, ms));

export default api;
