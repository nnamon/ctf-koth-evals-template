type Props = {
  title: string;
  variant?: "info" | "success" | "warn" | "error";
  children?: React.ReactNode;
  style?: React.CSSProperties;
};

// Alert wraps the kit's .alert pattern so title + body live in a single flex
// child — otherwise they get spread into separate columns on narrow widths.
export function Alert({ title, variant, children, style }: Props) {
  const className = variant ? `alert ${variant}` : "alert";
  return (
    <div className={className} style={style}>
      <div>
        <div className="alert-title">{title}</div>
        {children}
      </div>
    </div>
  );
}
