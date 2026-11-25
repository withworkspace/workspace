import type { ReactNode } from "react";
import { Icon } from "../ui/Icon";
import css from "./TalkReply.module.css";

// TalkReply is the reply box at the conversation's foot, kept in reach
// as it scrolls; onNewReply, a reply's come below where you're reading,
// and a pill over it goes there.
export function TalkReply({ onNewReply, children }: { onNewReply?: () => void; children: ReactNode }) {
  return (
    <div className={css.reply}>
      {onNewReply && (
        <button className={css.pill} onClick={onNewReply}>
          <Icon name="arrow-down" size={12} /> New reply
        </button>
      )}
      {children}
    </div>
  );
}
