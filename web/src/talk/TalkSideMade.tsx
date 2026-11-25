import type { ReactNode } from "react";
import { Button } from "../ui/Button";
import { Icon } from "../ui/Icon";
import css from "./TalkSideMade.module.css";

// TalkSideMade is something a conversation made, in its list beside it:
// the thing (a TalkSideItem, grow), and onWhere, a button to the card it
// was proposed in.
export function TalkSideMade({ onWhere, children }: { onWhere?: () => void; children: ReactNode }) {
  return (
    <li className={css.made}>
      {children}
      {onWhere && (
        <Button variant="ghost" square className={css.where} onClick={onWhere} title="Where it was proposed, in the conversation">
          <Icon name="arrow-up" size={12} />
          <span className="sr-only">Where it was proposed</span>
        </Button>
      )}
    </li>
  );
}
