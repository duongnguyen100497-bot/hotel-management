import React from 'react';
import ReactDOM from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import App from './App.jsx';
import { SettingsProvider } from './shared/contexts/SettingsContext.jsx';
import { ToastProvider } from './shared/contexts/ToastContext.jsx';
import { hydrateOnlineData } from './shared/services/onlineDataStore.js';
import './index.css';

// Nạp lại khách đăng ký / đặt phòng trực tuyến đã lưu để Admin và website khách hàng vẫn nhất quán sau khi tải lại trang.
hydrateOnlineData();

ReactDOM.createRoot(document.getElementById('root')).render(
  <React.StrictMode>
    <BrowserRouter>
      <SettingsProvider>
        <ToastProvider>
          <App />
        </ToastProvider>
      </SettingsProvider>
    </BrowserRouter>
  </React.StrictMode>,
);
