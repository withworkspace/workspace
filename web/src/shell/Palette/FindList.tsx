import type { ReactNode, Ref } from "react";
import css from "./FindList.module.css";

// FindList is what Find found, to pick from: FindSections of rows, at a
// height that doesn't change with how many there are, scrolled where
// there are more. keepFocus leaves the focus in the text box when
// something in it is pressed.
export function FindList({ id, ref, keepFocus, children }: { id: string; ref?: Ref<HTMLDivElement>; keepFocus?: boolean; children: ReactNode }) {
  return (
    <div className={css.list} id={id} role="listbox" ref={ref} onMouseDown={keepFocus ? (e) => e.preventDefault() : undefined}>
      {children}
    </div>
  );
}
