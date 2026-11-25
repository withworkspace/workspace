import { lazy, Suspense } from "react";
import type { Pinned } from "../refs/refs";
import { cx } from "../ui/cx";
import type { Place } from "./links";
import css from "./Markdown.module.css";

// The markdown renderer loads with the first page that shows markdown, not
// with every page.
const Inner = lazy(() => import("./Markdown").then((m) => ({ default: m.Markdown })));

export function Markdown({ text, lines, at, pinned, headingLink }: { text: string; lines?: boolean; at?: Place; pinned?: Pinned[]; headingLink?: (anchor: string) => void }) {
  return (
    // markdown: the name other places' rules and code still know a
    // document by (Markdown.tsx says which).
    <Suspense fallback={<div className={cx("markdown", css.markdown, css.loading)}>{text}</div>}>
      <Inner text={text} lines={lines} at={at} pinned={pinned} headingLink={headingLink} />
    </Suspense>
  );
}

// markdownClass is a document's look as the reader draws it, for the
// editor's document, which is drawn the same. In it, markdownCodeClass is
// a block of code (a size down in a document to read) and
// markdownDiagramClass a diagram.
export const markdownClass = css.markdown;
export const markdownCodeClass = css.code;
export const markdownDiagramClass = css.diagram;
