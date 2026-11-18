import { createContext, useCallback, useContext, useMemo, useState } from 'react';
import authService from '../services/authService.js';
import { DEFAULT_PERMISSIONS } from '../utils/permissions.js';

const AuthContext = createContext(null);

const SESSION_KEY = 'hms_session';
const PERMISSION_KEY = 'hms_permissions';

const readJson = (storage, key) => {
  try {
    const raw = storage.getItem(key);
    return raw ? JSON.parse(raw) : null;
  } catch {
    return null;
  }
};

const readSession = () => {
  const inLocal = readJson(localStorage, SESSION_KEY);
  if (inLocal) return { ...inLocal, remember: true };
  const inSession = readJson(sessionStorage, SESSION_KEY);
  return inSession ? { ...inSession, remember: false } : null;
};

const saveSession = (session) => {
  localStorage.removeItem(SESSION_KEY);
  sessionStorage.removeItem(SESSION_KEY);
  if (!session) return;
  const { remember, ...data } = session;
  (remember ? localStorage : sessionStorage).setItem(SESSION_KEY, JSON.stringify(data));
};

export function AuthProvider({ children }) {
  const [session, setSession] = useState(readSession);
  const [permissions, setPermissions] = useState(() => ({
    ...DEFAULT_PERMISSIONS,
    ...(readJson(localStorage, PERMISSION_KEY) || {}),
  }));

  const login = useCallback(async (username, password, remember = true) => {
    const { user, token } = await authService.login(username, password);
    const next = { user, token, remember };
    saveSession(next);
    setSession(next);
    return user;
  }, []);

  const logout = useCallback(() => {
    saveSession(null);
    setSession(null);
  }, []);

  // Cập nhật thông tin hồ sơ của người đang đăng nhập (sau khi đổi tên / email).
  const updateUser = useCallback((patch) => {
    setSession((prev) => {
      if (!prev) return prev;
      const next = { ...prev, user: { ...prev.user, ...patch } };
      saveSession(next);
      return next;
    });
  }, []);

  const updatePermissions = useCallback((next) => {
    setPermissions(next);
    try {
      localStorage.setItem(PERMISSION_KEY, JSON.stringify(next));
    } catch {
      /* bỏ qua nếu hết dung lượng lưu trữ */
    }
  }, []);

  const resetPermissions = useCallback(() => {
    setPermissions(DEFAULT_PERMISSIONS);
    localStorage.removeItem(PERMISSION_KEY);
  }, []);

  const user = session?.user ?? null;

  // Quản trị viên luôn có toàn quyền để không tự khóa mình khỏi hệ thống.
  const hasAccess = useCallback(
    (module) => !!user && (user.role === 'admin' || (permissions[user.role] || []).includes(module)),
    [user, permissions],
  );

  const value = useMemo(
    () => ({
      user,
      isAuthenticated: !!user,
      permissions,
      login,
      logout,
      updateUser,
      updatePermissions,
      resetPermissions,
      hasAccess,
    }),
    [user, permissions, login, logout, updateUser, updatePermissions, resetPermissions, hasAccess],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export const useAuth = () => useContext(AuthContext);
