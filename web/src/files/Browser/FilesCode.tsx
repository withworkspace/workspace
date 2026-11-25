import type { ReactNode, Ref } from "react";
import css from "./FilesCode.module.css";

// FilesCode is where a file's text is in the explorer, as an editor has
// it: no card around it. What's drawn over the lines (a name's card, the
// picker of its uses) comes before them (ui/CodeLines).
export function FilesCode({ ref, children }: { ref?: Ref<HTMLDivElement>; children: ReactNode }) {
  return (
    <div className={css.code} ref={ref}>
      {children}
    </div>
  );
}

// filesCodeLinesClass is the lines' own, there: on the page, not a well.
export const filesCodeLinesClass = css.lines;
