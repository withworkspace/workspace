import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./DocumentsFrame.module.css";

// DocumentsFrame is Documents' page: it takes the window's width.
export function DocumentsFrame({ children }: { children: ReactNode }) {
  return <div className={cx("page", css.frame)}>{children}</div>;
}
