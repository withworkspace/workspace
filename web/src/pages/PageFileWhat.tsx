import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./PageFileWhat.module.css";

// PageFileWhat says what filing or saving a page does, or what starting one
// as a kind of document does, under its fields.
export function PageFileWhat({ children }: { children: ReactNode }) {
  return <p className={cx("dim", css.what)}>{children}</p>;
}
