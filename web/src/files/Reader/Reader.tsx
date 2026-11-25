import type { ReactNode, Ref } from "react";
import { cx } from "../../ui/cx";
import css from "./Reader.module.css";

type ReaderProps = {
  // Its contents beside the text (a ReaderRail, in after), or folded into
  // a pill over it, where the rail doesn't fit.
  rail?: boolean;
  folded?: boolean;
  pageRef?: Ref<HTMLDivElement>;
  columnRef?: Ref<HTMLDivElement>;
  // The column: its path, its head, its text and what's under it.
  children: ReactNode;
  // Beside the column, or over it: the rail, the menus, a passage's tools.
  after?: ReactNode;
};

// Reader is a document read, as a page: its column, and its contents
// beside it where there's room. A heading gone to sits below the top (and
// below the pill), and is marked for a moment.
export function Reader({ rail, folded, pageRef, columnRef, children, after }: ReaderProps) {
  return (
    <div ref={pageRef} className={cx("page artifact-page", css.reader, rail && css.withRail, folded && css.folded)}>
      <div className={css.column} ref={columnRef}>
        {children}
      </div>
      {after}
    </div>
  );
}

// ReaderText is the document's text, rendered (children: its Markdown), as
// the page's prose.
export function ReaderText({ ref, children }: { ref?: Ref<HTMLElement>; children: ReactNode }) {
  return (
    <article className="artifact-body prose" ref={ref}>
      {children}
    </article>
  );
}
