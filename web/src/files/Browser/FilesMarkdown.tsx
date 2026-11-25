import type { ReactNode } from "react";
import { cx } from "../../ui/cx";
import css from "./FilesMarkdown.module.css";

// FilesMarkdown is a markdown file that's open, rendered (children: its
// Markdown), as the page's prose.
export function FilesMarkdown({ children }: { children: ReactNode }) {
  return <article className={cx("artifact-body prose", css.markdown)}>{children}</article>;
}
