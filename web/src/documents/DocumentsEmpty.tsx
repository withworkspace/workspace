import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./DocumentsEmpty.module.css";

// DocumentsEmpty is a line said where a list would be: there are none,
// it's being searched, nothing says that.
export function DocumentsEmpty({ children }: { children: ReactNode }) {
  return <p className={cx("dim", css.empty)}>{children}</p>;
}
