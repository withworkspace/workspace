import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./PageVersion.module.css";

// PageVersion is a line about a version of a cell: written, when, and a
// way to put it back. yours is one under what you asked, on your side.
export function PageVersion({ yours, children }: { yours?: boolean; children: ReactNode }) {
  return <div className={cx(css.version, yours && css.yours)}>{children}</div>;
}
