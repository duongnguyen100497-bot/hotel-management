import { useEffect } from 'react';

// Đặt tiêu đề tab trình duyệt cho một trang, trả về tiêu đề cũ khi rời trang.
export default function useDocumentTitle(title) {
  useEffect(() => {
    const previous = document.title;
    if (title) document.title = title;
    return () => {
      document.title = previous;
    };
  }, [title]);
}
