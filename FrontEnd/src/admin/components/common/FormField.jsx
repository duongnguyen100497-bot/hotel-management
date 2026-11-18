export function Field({ label, error, required, hint, className = '', children }) {
  return (
    <div className={className}>
      {label && (
        <label className="mb-1.5 block text-sm font-medium text-slate-700">
          {label}
          {required && <span className="ml-0.5 text-red-500">*</span>}
        </label>
      )}
      {children}
      {error ? (
        <p className="mt-1 text-xs text-red-600">{error}</p>
      ) : (
        hint && <p className="mt-1 text-xs text-slate-400">{hint}</p>
      )}
    </div>
  );
}

export function Input({ label, error, required, hint, className = '', ...rest }) {
  return (
    <Field label={label} error={error} required={required} hint={hint} className={className}>
      <input className={`input ${error ? 'input-error' : ''}`} {...rest} />
    </Field>
  );
}

// options: ['A', 'B'] hoặc [{ value, label }]
export function Select({ label, error, required, hint, className = '', options = [], placeholder, ...rest }) {
  return (
    <Field label={label} error={error} required={required} hint={hint} className={className}>
      <select className={`input ${error ? 'input-error' : ''}`} {...rest}>
        {placeholder !== undefined && <option value="">{placeholder}</option>}
        {options.map((o) => {
          const opt = typeof o === 'string' ? { value: o, label: o } : o;
          return (
            <option key={opt.value} value={opt.value}>
              {opt.label}
            </option>
          );
        })}
      </select>
    </Field>
  );
}

export function Textarea({ label, error, required, hint, className = '', rows = 3, ...rest }) {
  return (
    <Field label={label} error={error} required={required} hint={hint} className={className}>
      <textarea rows={rows} className={`input resize-none ${error ? 'input-error' : ''}`} {...rest} />
    </Field>
  );
}
