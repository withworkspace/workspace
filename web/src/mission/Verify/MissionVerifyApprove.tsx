import type { ReactNode } from "react";
import { Button } from "../../ui/Button";
import css from "./MissionVerifyApprove.module.css";

type MissionVerifyApproveProps = {
  // What its button says: Approve b2, or Send back to c1 (2).
  children: ReactNode;
  // What pressing it does, said beside it.
  says: ReactNode;
  // Checks are still left for you: it still approves, as it is, but
  // isn't the one to press yet.
  plain?: boolean;
  disabled?: boolean;
  title?: string;
  // The key that settles it, said after the title and hinted at.
  keys?: string;
  onSettle: () => void;
};

// MissionVerifyApprove is the bundle's approval, under what's left for
// you: what you look at, then what you say of it. It's the one to press
// once nothing's left, or once you've said something isn't right (then
// it sends the bundle back).
export function MissionVerifyApprove({ children, says, plain, disabled, title, keys, onSettle }: MissionVerifyApproveProps) {
  return (
    <div className={css.approve}>
      <Button variant={plain ? undefined : "primary"} className={plain ? css.plain : undefined} disabled={disabled} onClick={onSettle} title={title} keys={keys}>
        {children}
      </Button>
      <span role="status" className={css.says}>
        {says}
      </span>
    </div>
  );
}
