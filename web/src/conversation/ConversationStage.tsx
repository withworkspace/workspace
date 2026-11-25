import type { ReactNode } from "react";
import css from "./ConversationStage.module.css";

// ConversationStage is a page with its conversation round it: the page
// (children), and over its foot what asks about it (foot), which lets
// clicks beside it through to the page.
export function ConversationStage({ children, foot }: { children: ReactNode; foot?: ReactNode }) {
  return (
    <div className={css.stage}>
      <div className={css.main}>
        {children}
        {foot && <div className={css.foot}>{foot}</div>}
      </div>
    </div>
  );
}
