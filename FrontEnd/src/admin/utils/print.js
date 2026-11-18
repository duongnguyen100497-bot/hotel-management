export const printPage = () => window.print();

// In nội dung trong Modal: tạm ẩn toàn bộ ứng dụng, chỉ in lớp modal (xem index.css).
export const printModalContent = () => {
  const cleanup = () => {
    document.body.classList.remove('print-modal');
    window.removeEventListener('afterprint', cleanup);
  };
  document.body.classList.add('print-modal');
  window.addEventListener('afterprint', cleanup);
  window.print();
};
