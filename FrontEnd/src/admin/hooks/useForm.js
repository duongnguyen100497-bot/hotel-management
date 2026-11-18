import { useCallback, useRef, useState } from 'react';

/**
 * Quản lý state + validation cho form.
 * validateFn(values) trả về object { field: 'thông báo lỗi' | '' }.
 * handleSubmit(onValid) trả về submit handler: chỉ gọi onValid(values) khi không còn lỗi.
 * reset(next) ổn định giữa các lần render nên có thể đặt trong deps của useEffect.
 */
export default function useForm(initialValues, validateFn) {
  const [values, setValues] = useState(initialValues);
  const [errors, setErrors] = useState({});
  const initialRef = useRef(initialValues);
  const valuesRef = useRef(values);
  const validateRef = useRef(validateFn);
  valuesRef.current = values;
  validateRef.current = validateFn;

  const handleChange = useCallback((e) => {
    const { name, value, type, checked } = e.target;
    setValues((prev) => ({ ...prev, [name]: type === 'checkbox' ? checked : value }));
    setErrors((prev) => (prev[name] ? { ...prev, [name]: '' } : prev));
  }, []);

  const reset = useCallback((next) => {
    setValues(next ?? initialRef.current);
    setErrors({});
  }, []);

  const handleSubmit = useCallback(
    (onValid) => async (e) => {
      e?.preventDefault?.();
      const result = validateRef.current ? validateRef.current(valuesRef.current) : {};
      const found = Object.fromEntries(Object.entries(result).filter(([, message]) => message));
      setErrors(found);
      if (Object.keys(found).length === 0) await onValid(valuesRef.current);
    },
    [],
  );

  return { values, errors, setValues, setErrors, handleChange, handleSubmit, reset };
}
