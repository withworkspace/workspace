import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./CodeBlock.module.css";

type CodeBlockProps = {
  // Its language, shown in the corner.
  lang?: string;
  // The code highlighted, as HTML (review/highlight.ts); or the code as
  // it is, as children.
  html?: string;
  children?: ReactNode;
  className?: string;
  // The block's data- attributes (which lines of a document it's from).
  data?: Record<string, unknown>;
};

// CodeBlock is a fenced block of code: on a well, in the code's font, its
// language in the corner (not the document's text: data-skip).
export function CodeBlock({ lang, html, children, className, data }: CodeBlockProps) {
  return (
    <pre className={cx(css.block, className)} {...data}>
      {lang && (
        <span className={css.lang} data-skip="">
          {lang}
        </span>
      )}
      {html !== undefined ? <code dangerouslySetInnerHTML={{ __html: html }} /> : <code>{children}</code>}
    </pre>
  );
}

// codeBlockClass and codeLangClass are its look for a block drawn by hand:
// the editor's (editor/views.ts), and code shown as written while what
// draws it loads.
export const codeBlockClass = css.block;
export const codeLangClass = css.lang;
