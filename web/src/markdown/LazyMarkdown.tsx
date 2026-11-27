import { lazy, Suspense } from "react";
import { cx } from "../ui/cx";
import type { MarkdownProps } from "./Markdown";
import css from "./Markdown.module.css";

// The markdown renderer loads with the first page that shows markdown, not
// with every page.
const Inner = lazy(() => import("./Markdown").then((m) => ({ default: m.Markdown })));

export function Markdown(props: MarkdownProps) {
  return (
    // markdown: the name other places' rules and code still know a
    // document by (Markdown.tsx says which).
    <Suspense fallback={<div className={cx("markdown", css.markdown, css.loading)}>{props.text}</div>}>
      <Inner {...props} />
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
