import css from "./SrOnly.module.css";

// SrOnly is words a screen reader says that aren't drawn: what a mark or
// an icon means.
export function SrOnly({ children }: { children: React.ReactNode }) {
  return <span className={css.srOnly}>{children}</span>;
}

// srOnlyClass is the same for what isn't a span: a label kept for a
// screen reader alone.
export const srOnlyClass: string = css.srOnly;
