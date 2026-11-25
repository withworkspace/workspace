import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./RefExcerpt.module.css";

// RefExcerpt is a few lines of a file in a ref's card (or what's in a
// folder), each a RefExcerptLine; more: there's more after them. raised:
// it's in Find's pane, which is the page's colour, so it's the raised one.
export function RefExcerpt({ more, raised, children }: { more?: boolean; raised?: boolean; children: ReactNode }) {
  return (
    <span className={cx(css.excerpt, raised && css.raised)}>
      {children}
      {more && <span className={cx(css.line, css.more)}>…</span>}
    </span>
  );
}

// RefExcerptLine is one line of it: its number (n, if it has one) and what
// it says; named, one of the lines the ref names.
export function RefExcerptLine({ n, named, html, children }: { n?: number; named?: boolean; html?: string; children?: ReactNode }) {
  return (
    <span className={cx(css.line, named && css.named)}>
      <span className={css.ln}>{n}</span>
      {html !== undefined ? <span dangerouslySetInnerHTML={{ __html: html }} /> : <span>{children}</span>}
    </span>
  );
}
