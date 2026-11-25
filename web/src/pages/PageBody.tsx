import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./PageBody.module.css";

// PageBody is the page and, with map, its map beside it. hidden keeps it
// (and its editor) while a pass's tab is shown instead.
export function PageBody({ map, hidden, children }: { map?: boolean; hidden?: boolean; children: ReactNode }) {
  return (
    <div className={cx(css.body, map && css.withMap)} style={hidden ? { display: "none" } : undefined}>
      {children}
    </div>
  );
}
