// Quy tắc mật khẩu + băm mật khẩu phía trình duyệt (SHA-256 + salt).
// LƯU Ý: đây chỉ là cơ chế DEMO cho bản chưa có Backend. Khi có máy chủ, mật khẩu phải được
// băm (bcrypt/argon2) và xác thực ở phía máy chủ, không lưu ở trình duyệt.
export const PASSWORD_MIN = 8;

export function passwordIssues(pw = '') {
  const issues = [];
  if (pw.length < PASSWORD_MIN) issues.push(`Tối thiểu ${PASSWORD_MIN} ký tự`);
  if (!/[A-Za-zÀ-ỹ]/.test(pw)) issues.push('Có ít nhất 1 chữ cái');
  if (!/\d/.test(pw)) issues.push('Có ít nhất 1 chữ số');
  return issues;
}

// 0–4, dùng cho thanh đo độ mạnh.
export function passwordStrength(pw = '') {
  if (!pw) return 0;
  let score = 0;
  if (pw.length >= PASSWORD_MIN) score += 1;
  if (pw.length >= 12) score += 1;
  if (/[a-z]/.test(pw) && /[A-Z]/.test(pw)) score += 1;
  if (/\d/.test(pw) && /[^A-Za-z0-9]/.test(pw)) score += 1;
  return Math.min(4, score);
}

const toHex = (buf) => [...new Uint8Array(buf)].map((b) => b.toString(16).padStart(2, '0')).join('');

export function randomSalt() {
  const bytes = new Uint8Array(16);
  globalThis.crypto.getRandomValues(bytes);
  return toHex(bytes);
}

export async function hashPassword(password, salt) {
  const subtle = globalThis.crypto?.subtle;
  if (!subtle) throw new Error('Trình duyệt không hỗ trợ mã hóa an toàn. Vui lòng mở bằng HTTPS hoặc localhost.');
  const data = new TextEncoder().encode(`${salt}:${password}`);
  return toHex(await subtle.digest('SHA-256', data));
}
