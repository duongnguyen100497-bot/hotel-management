import { delay } from '../../shared/services/api.js';
import { seedUsers } from '../../shared/services/mockData.js';

const users = seedUsers.map((u) => ({ ...u }));
const strip = ({ password, ...safe }) => safe; // eslint-disable-line no-unused-vars

const authService = {
  async login(username, password) {
    await delay(500);
    const key = username.trim().toLowerCase();
    const user = users.find(
      (u) => (u.username.toLowerCase() === key || u.email.toLowerCase() === key) && u.password === password,
    );
    if (!user) throw new Error('Tên đăng nhập hoặc mật khẩu không đúng');
    return { user: strip(user), token: `mock-token-${user.id}` };
  },

  async changePassword(userId, currentPassword, newPassword) {
    await delay(400);
    const user = users.find((u) => u.id === userId);
    if (!user) throw new Error('Không tìm thấy tài khoản');
    if (user.password !== currentPassword) throw new Error('Mật khẩu hiện tại không đúng');
    user.password = newPassword;
  },

  async updateProfile(userId, patch) {
    await delay(300);
    const index = users.findIndex((u) => u.id === userId);
    if (index === -1) throw new Error('Không tìm thấy tài khoản');
    users[index] = { ...users[index], ...patch };
    return strip(users[index]);
  },

  async requestPasswordReset(email) {
    await delay(500);
    const exists = users.some((u) => u.email.toLowerCase() === email.trim().toLowerCase());
    // Luôn trả về thành công để không lộ thông tin tài khoản.
    return { sent: true, exists };
  },
};

export default authService;
