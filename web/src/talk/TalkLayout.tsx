import type { ReactNode, Ref } from "react";
import { cx } from "../ui/cx";
import css from "./TalkLayout.module.css";

// TalkLayout is a conversation's page: its bar, then the conversation,
// and, side, what's come of it in a column beside it.
export function TalkLayout({ side, ref, children }: { side?: boolean; ref?: Ref<HTMLDivElement>; children: ReactNode }) {
  return (
    <div className={cx("page", css.layout, side && css.side)} ref={ref}>
      {children}
    </div>
  );
}
