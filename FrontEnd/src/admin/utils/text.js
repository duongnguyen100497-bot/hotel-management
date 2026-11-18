// Chuẩn hóa chuỗi để tìm kiếm không phân biệt hoa thường và dấu tiếng Việt.
export const normalizeText = (value) =>
  String(value ?? '')
    .toLowerCase()
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
    .replace(/đ/g, 'd');

// Kiểm tra bất kỳ trường nào của item khớp từ khóa.
export const matchesSearch = (item, keys, query) => {
  const q = normalizeText(query.trim());
  if (!q) return true;
  return keys.some((k) => normalizeText(item[k]).includes(q));
};
