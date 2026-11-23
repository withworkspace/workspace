import type { ReactNode } from "react";
import css from "./DockHead.module.css";

// DockHead is the top of a dock: what's open in it, as tabs (DockTabs),
// and at its end what the dock itself does (float it, close it, which of
// what's below).
export function DockHead({
  children,
  tools,
}: {
  children: ReactNode;
  tools?: ReactNode;
}) {
  return (
    <div className={css.head}>
      {children}
      {tools && <div className={css.tools}>{tools}</div>}
    </div>
  );
}
