import type { ReactNode } from "react";
import { type Anchor, FloatingCard } from "../ui/FloatingCard";
import css from "./RefFloat.module.css";
import { RefFoot } from "./RefFoot";
import { Text } from "../ui/Text";

type RefFloatProps = {
  anchor: Anchor;
  onEnter: () => void;
  onLeave: () => void;
  // hint says ⌘-click goes there: where words are written, a click is for
  // the cursor.
  hint?: boolean;
  // What's at the end of its foot: pinning it.
  pin?: ReactNode;
  children: ReactNode;
};

// RefFloat is a ref's card over the page, under the words that name it,
// as wide as what it says within its bounds, with its foot.
export function RefFloat({ anchor, onEnter, onLeave, hint, pin, children }: RefFloatProps) {
  return (
    <FloatingCard anchor={anchor} className={css.float} onEnter={onEnter} onLeave={onLeave}>
      {children}
      <RefFoot>
        {hint && <Text dim className={css.hint}>⌘-click to open</Text>}
        {pin}
      </RefFoot>
    </FloatingCard>
  );
}
