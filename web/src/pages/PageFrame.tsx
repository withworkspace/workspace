import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./PageFrame.module.css";

// PageFrame is a page's frame: the width of its words, or wider with its
// map beside them.
export function PageFrame({ map, children }: { map?: boolean; children: ReactNode }) {
  return <div className={cx("page", css.frame, map && css.withMap)}>{children}</div>;
}
