import { cx } from "./cx";
import css from "./Summary.module.css";

// Summary is a list of what something is: a label, then its value, a
// SummaryRow each.
export function Summary({ className, ...rest }: React.ComponentProps<"dl">) {
  return <dl className={cx(css.summary, className)} {...rest} />;
}

// SummaryRow is one of a Summary's: its label, then what it is.
export function SummaryRow({ label, children, className }: { label: React.ReactNode; children: React.ReactNode; className?: string }) {
  return (
    <div className={cx(css.row, className)}>
      <dt>{label}</dt>
      <dd>{children}</dd>
    </div>
  );
}
