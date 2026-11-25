import type { ReactNode } from "react";
import { createPortal } from "react-dom";
import type { EditorProps } from "./MarkdownEditor";

// placed is an editor's toolbar where it goes: over it, or in the page's
// one bar while it's the document being edited.
export function placed(bar: ReactNode, props: Pick<EditorProps, "toolbarIn" | "toolbarOn">): ReactNode {
  if (props.toolbarIn === undefined) return bar;
  return props.toolbarIn && props.toolbarOn ? createPortal(bar, props.toolbarIn) : null;
}
