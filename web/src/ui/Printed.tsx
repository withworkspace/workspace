import type { ReactNode } from "react";
import { cx } from "./cx";
import css from "./Printed.module.css";

// Printed is what a command printed, kept: as it was written, in a well,
// scrolling in itself when it's long. A page gives it another height, or
// a size, by a class of its own. (A run's output that's still coming is
// RunOutput, which follows it.)
export function Printed({ children, className }: { children: ReactNode; className?: string }) {
  return <pre className={cx(css.printed, className)}>{children}</pre>;
}
