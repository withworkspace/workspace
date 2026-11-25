import { lazy, Suspense } from "react";
import { markdownClass } from "../markdown/LazyMarkdown";
import { cx } from "../ui/cx";
import type { EditorProps } from "./MarkdownEditor";
import css from "./MarkdownEditor.module.css";

export type { Draft, EditorProps } from "./MarkdownEditor";
export type { Quote } from "./views";

// The editor (ProseMirror and the markdown bridge) loads with the first
// thing you edit, not with every page. Until it's here, the text shows
// where it'll be.
const Inner = lazy(() => import("./MarkdownEditor"));

export function MarkdownEditor(props: EditorProps) {
  return (
    <Suspense
      fallback={
        <div className={css.editor}>
          <div className="prose">
            <div className={cx("markdown pm", markdownClass, css.pm, props.compact && css.compact, css.loading)}>{props.initial}</div>
          </div>
        </div>
      }
    >
      <Inner {...props} />
    </Suspense>
  );
}
