import { lazy, Suspense } from "react";
import type { EditorProps } from "./MarkdownEditor";

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
        <div className={`doc-editor loading${props.compact ? " compact" : ""}`}>
          <div className="prose">
            <div className="markdown pm">{props.initial}</div>
          </div>
        </div>
      }
    >
      <Inner {...props} />
    </Suspense>
  );
}
