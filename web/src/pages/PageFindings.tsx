import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./PageFindings.module.css";

// PageFindings are what a pass found: a PageFinding each. settled are
// those dealt with, said smaller.
export function PageFindings({ settled, children }: { settled?: boolean; children: ReactNode }) {
  return <ol className={cx(css.findings, settled && css.settled)}>{children}</ol>;
}
