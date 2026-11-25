import type { ReactNode, Ref } from "react";
import { cx } from "../ui/cx";
import css from "./SpacePreview.module.css";

// SpacePreview is the document picked in a space, beside its list: a
// SpacePreviewHead, what's in flight, its sections, its text and the
// changes to it lately. With none picked (none), what it says instead.
export function SpacePreview({ label, paneRef, none, children }: { label?: string; paneRef?: Ref<HTMLElement>; none?: boolean; children: ReactNode }) {
  return (
    <aside className={cx(css.preview, none && css.none)} ref={paneRef} aria-label={label}>
      {children}
    </aside>
  );
}
