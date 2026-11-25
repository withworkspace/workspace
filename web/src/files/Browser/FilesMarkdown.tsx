import type { ReactNode } from "react";
import { cx } from "../../ui/cx";
import css from "./FilesMarkdown.module.css";
import { Prose } from "../../ui/Prose";

// FilesMarkdown is a markdown file that's open, rendered (children: its
// Markdown), as the page's prose.
export function FilesMarkdown({ children }: { children: ReactNode }) {
  return <Prose as="article" className={cx("artifact-body", css.markdown)}>{children}</Prose>;
}
