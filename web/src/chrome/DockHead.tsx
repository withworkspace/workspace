import type { ReactNode } from "react";
import css from "./DockHead.module.css";

// DockHead is the top of a dock: what's open in it, as tabs (DockTabs),
// and at its end what the dock itself does (which of what's below; a
// sheet's way out). Closing a dock, and floating it, are its toggle's at
// the foot, so the tabs have the head's room.
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
