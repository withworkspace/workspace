import type { KeyboardEventHandler, ReactNode } from "react";
import css from "./PageFields.module.css";

// PageFields are what's filled in to file a page, or start one: a
// PageField each, their names in a column before them.
export function PageFields({ onKeyDown, children }: { onKeyDown?: KeyboardEventHandler; children: ReactNode }) {
  return (
    <div className={css.fields} onKeyDown={onKeyDown}>
      {children}
    </div>
  );
}
