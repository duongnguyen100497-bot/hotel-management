import customerService from '../../shared/services/customerService.js';
import { persistOnlineData } from '../../shared/services/onlineDataStore.js';
import { delay } from '../../shared/services/api.js';
import { hashPassword, passwordIssues, randomSalt } from '../utils/password.js';
import { firstError, isEmail, isPhone, minLength, required } from '../../shared/utils/validators.js';

// Xác thực khách hàng (tách biệt hoàn toàn với đăng nhập Admin: khóa lưu trữ riêng, không dùng chung phiên).
// Tài khoản khách gắn với một bản ghi khách hàng (customerId) trong dữ liệu Admin để lễ tân thấy cùng một người.
const ACCOUNTS_KEY = 'hms_customer_accounts';
const SESSION_KEY = 'hms_customer_session';

// Tài khoản DEMO gắn với khách KH001 (có sẵn lịch sử đặt phòng) để xem thử "Đặt phòng của tôi".
export const DEMO_ACCOUNT = { email: 'demo@grandnavy.vn', password: 'Demo@123', customerId: 'KH001' };

const store = (kind) => {
  try {
    return window[kind];
  } catch {
    return null;
  }
};
const readJson = (storage, key, fallback) => {
  try {
    const raw = storage?.getItem(key);
    return raw ? JSON.parse(raw) : fallback;
  } catch {
    return fallback;
  }
};
const writeJson = (storage, key, value) => {
  try {
    storage?.setItem(key, JSON.stringify(value));
  } catch {
    /* ignore */
  }
};

const normEmail = (e) => String(e || '').trim().toLowerCase();
const loadAccounts = () => readJson(store('localStorage'), ACCOUNTS_KEY, []);
const saveAccounts = (list) => writeJson(store('localStorage'), ACCOUNTS_KEY, list);

async function ensureDemo() {
  const list = loadAccounts();
  if (list.some((a) => a.email === DEMO_ACCOUNT.email)) return list;
  const salt = randomSalt();
  const demo = {
    id: 'TK-DEMO',
    customerId: DEMO_ACCOUNT.customerId,
    email: DEMO_ACCOUNT.email,
    salt,
    passwordHash: await hashPassword(DEMO_ACCOUNT.password, salt),
    createdAt: new Date().toISOString(),
    demo: true,
  };
  const next = [demo, ...list];
  saveAccounts(next);
  return next;
}

export function validateRegister(v) {
  const errors = {};
  const set = (k, msg) => msg && (errors[k] = msg);
  set('fullName', firstError(required(v.fullName, 'Họ tên'), minLength(v.fullName, 2, 'Họ tên')));
  set('email', firstError(required(v.email, 'Email'), isEmail(v.email)));
  set('phone', firstError(required(v.phone, 'Số điện thoại'), isPhone(v.phone)));
  const issues = passwordIssues(v.password);
  if (!v.password) errors.password = 'Mật khẩu không được để trống';
  else if (issues.length) errors.password = `Mật khẩu cần: ${issues.join(', ').toLowerCase()}`;
  if (!v.confirmPassword) errors.confirmPassword = 'Vui lòng nhập lại mật khẩu';
  else if (v.confirmPassword !== v.password) errors.confirmPassword = 'Mật khẩu nhập lại không khớp';
  if (!v.agree) errors.agree = 'Bạn cần đồng ý với điều khoản để tiếp tục';
  return errors;
}

export function validateLogin(v) {
  const errors = {};
  const e = firstError(required(v.email, 'Email'), isEmail(v.email));
  if (e) errors.email = e;
  if (!v.password) errors.password = 'Mật khẩu không được để trống';
  return errors;
}

const toView = (account) => {
  const customer = customerService.internal.get().find((c) => c.id === account.customerId) || null;
  return { account: { id: account.id, email: account.email, customerId: account.customerId, demo: Boolean(account.demo) }, customer };
};

const customerAuthService = {
  // Khôi phục phiên đăng nhập (đồng bộ, dùng khi khởi động để không bị nháy giao diện).
  restoreSession() {
    const accountId = readJson(store('localStorage'), SESSION_KEY, null) || readJson(store('sessionStorage'), SESSION_KEY, null);
    if (!accountId) return null;
    const account = loadAccounts().find((a) => a.id === accountId);
    return account ? toView(account) : null;
  },

  async register(values) {
    await delay(300);
    const errors = validateRegister(values);
    if (Object.keys(errors).length) throw Object.assign(new Error('Thông tin chưa hợp lệ'), { fields: errors });

    const accounts = await ensureDemo();
    const email = normEmail(values.email);
    if (accounts.some((a) => a.email === email)) {
      throw Object.assign(new Error('Email này đã được đăng ký. Hãy đăng nhập hoặc dùng email khác.'), { fields: { email: 'Email đã được đăng ký' } });
    }

    // Luôn tạo bản ghi khách mới để không ai có thể "nhận" lịch sử đặt phòng của khách khác chỉ bằng email.
    const customer = await customerService.create({
      fullName: values.fullName.trim(),
      phone: values.phone.replace(/[\s.-]/g, ''),
      email,
      idNumber: '', // khách cung cấp CCCD/hộ chiếu khi nhận phòng tại quầy
      nationality: 'Việt Nam',
      address: '',
      source: 'online',
    });
    const salt = randomSalt();
    const account = {
      id: `TK${String(accounts.length + 1).padStart(4, '0')}`,
      customerId: customer.id,
      email,
      salt,
      passwordHash: await hashPassword(values.password, salt),
      createdAt: new Date().toISOString(),
    };
    saveAccounts([...accounts, account]);
    persistOnlineData();
    this.startSession(account.id, true);
    return toView(account);
  },

  async login({ email, password, remember }) {
    await delay(300);
    const errors = validateLogin({ email, password });
    if (Object.keys(errors).length) throw Object.assign(new Error('Thông tin chưa hợp lệ'), { fields: errors });
    const accounts = await ensureDemo();
    const account = accounts.find((a) => a.email === normEmail(email));
    // Cùng một thông báo cho "sai email" và "sai mật khẩu" để không lộ email nào đã đăng ký.
    const fail = () => new Error('Email hoặc mật khẩu không đúng');
    if (!account) throw fail();
    if ((await hashPassword(password, account.salt)) !== account.passwordHash) throw fail();
    this.startSession(account.id, Boolean(remember));
    return toView(account);
  },

  startSession(accountId, remember) {
    this.logout();
    writeJson(store(remember ? 'localStorage' : 'sessionStorage'), SESSION_KEY, accountId);
  },

  logout() {
    ['localStorage', 'sessionStorage'].forEach((k) => {
      try {
        store(k)?.removeItem(SESSION_KEY);
      } catch {
        /* ignore */
      }
    });
  },

  async updateProfile(customerId, values) {
    const errors = {};
    const name = firstError(required(values.fullName, 'Họ tên'), minLength(values.fullName, 2, 'Họ tên'));
    const phone = firstError(required(values.phone, 'Số điện thoại'), isPhone(values.phone));
    if (name) errors.fullName = name;
    if (phone) errors.phone = phone;
    if (Object.keys(errors).length) throw Object.assign(new Error('Thông tin chưa hợp lệ'), { fields: errors });
    const updated = await customerService.update(customerId, {
      fullName: values.fullName.trim(),
      phone: values.phone.replace(/[\s.-]/g, ''),
      nationality: values.nationality,
      address: (values.address || '').trim(),
    });
    persistOnlineData();
    return updated;
  },

  async changePassword(accountId, { current, next, confirm }) {
    await delay(300);
    const errors = {};
    if (!current) errors.current = 'Vui lòng nhập mật khẩu hiện tại';
    const issues = passwordIssues(next);
    if (!next) errors.next = 'Vui lòng nhập mật khẩu mới';
    else if (issues.length) errors.next = `Mật khẩu cần: ${issues.join(', ').toLowerCase()}`;
    if (next && next === current) errors.next = 'Mật khẩu mới phải khác mật khẩu hiện tại';
    if (!confirm) errors.confirm = 'Vui lòng nhập lại mật khẩu mới';
    else if (confirm !== next) errors.confirm = 'Mật khẩu nhập lại không khớp';
    if (Object.keys(errors).length) throw Object.assign(new Error('Thông tin chưa hợp lệ'), { fields: errors });

    const accounts = loadAccounts();
    const account = accounts.find((a) => a.id === accountId);
    if (!account) throw new Error('Không tìm thấy tài khoản');
    if ((await hashPassword(current, account.salt)) !== account.passwordHash) {
      throw Object.assign(new Error('Mật khẩu hiện tại không đúng'), { fields: { current: 'Mật khẩu hiện tại không đúng' } });
    }
    const salt = randomSalt();
    const passwordHash = await hashPassword(next, salt);
    saveAccounts(accounts.map((a) => (a.id === accountId ? { ...a, salt, passwordHash } : a)));
  },
};

export default customerAuthService;
