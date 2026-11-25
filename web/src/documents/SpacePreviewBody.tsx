import type { ReactNode, Ref } from "react";
import { cx } from "../ui/cx";
import css from "./SpacePreviewBody.module.css";

// SpacePreviewBody is the document picked's text, as the reader shows it
// (artifact-body and prose: the looks a document's text has wherever
// it's read).
export function SpacePreviewBody({ bodyRef, children }: { bodyRef?: Ref<HTMLElement>; children: ReactNode }) {
  return (
    <article className={cx("artifact-body prose", css.body)} ref={bodyRef}>
      {children}
    </article>
  );
}
