import { type ComponentProps, type ReactNode, useContext } from "react";
import { cx } from "../ui/cx";
import { HeadingLinkContext } from "./context";
import css from "./Markdown.module.css";

// MarkdownHeading is a document's h2 or h3. Where the reader links to
// headings it has a #, drawn by CSS so it isn't among the heading's words,
// and marked as not the document's text.
export function MarkdownHeading({ level, children, ...props }: { level: 2 | 3; children?: ReactNode } & ComponentProps<"h2">) {
  const link = useContext(HeadingLinkContext);
  const H = level === 2 ? "h2" : "h3";
  const anchor = (props as Record<string, unknown>)["data-anchor"];
  if (!link || typeof anchor !== "string") return <H {...props}>{children}</H>;
  return (
    <H {...props} className={cx(props.className, css.linked)}>
      <button type="button" className={css.headingLink} data-skip="" title="Copy link to section" aria-label="Copy link to section" onClick={() => link(anchor)} />
      {children}
    </H>
  );
}
