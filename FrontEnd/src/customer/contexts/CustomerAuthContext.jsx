import { createContext, useCallback, useContext, useMemo, useState } from 'react';
import customerAuthService from '../services/customerAuthService.js';

// Phiên đăng nhập của KHÁCH HÀNG. Hoàn toàn tách khỏi AuthContext của Admin (khóa lưu trữ khác nhau).
const CustomerAuthContext = createContext(null);

export function CustomerAuthProvider({ children }) {
  const [session, setSession] = useState(() => customerAuthService.restoreSession());

  const login = useCallback(async (values) => {
    const next = await customerAuthService.login(values);
    setSession(next);
    return next;
  }, []);

  const register = useCallback(async (values) => {
    const next = await customerAuthService.register(values);
    setSession(next);
    return next;
  }, []);

  const logout = useCallback(() => {
    customerAuthService.logout();
    setSession(null);
  }, []);

  const updateProfile = useCallback(
    async (values) => {
      const customer = await customerAuthService.updateProfile(session.account.customerId, values);
      setSession((s) => ({ ...s, customer }));
      return customer;
    },
    [session],
  );

  const changePassword = useCallback((values) => customerAuthService.changePassword(session.account.id, values), [session]);

  const value = useMemo(
    () => ({
      account: session?.account || null,
      customer: session?.customer || null,
      isAuthenticated: Boolean(session),
      login,
      register,
      logout,
      updateProfile,
      changePassword,
    }),
    [session, login, register, logout, updateProfile, changePassword],
  );

  return <CustomerAuthContext.Provider value={value}>{children}</CustomerAuthContext.Provider>;
}

export function useCustomerAuth() {
  const ctx = useContext(CustomerAuthContext);
  if (!ctx) throw new Error('useCustomerAuth phải được dùng bên trong CustomerAuthProvider');
  return ctx;
}
