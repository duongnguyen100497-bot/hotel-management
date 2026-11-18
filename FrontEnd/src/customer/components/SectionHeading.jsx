// Tiêu đề section: căn trái, không nhãn phụ. `light` dùng trên nền tối.
export default function SectionHeading({ id, title, description, light = false, className = '' }) {
  return (
    <div className={`max-w-2xl ${className}`}>
      <h2 id={id} className={`text-3xl font-extrabold tracking-tight sm:text-4xl lg:text-[2.75rem] lg:leading-[1.1] ${light ? 'text-white' : 'text-brand'}`}>
        {title}
      </h2>
      {description && <p className={`mt-4 text-base leading-relaxed sm:text-lg ${light ? 'text-sky-200/80' : 'text-muted'}`}>{description}</p>}
    </div>
  );
}
