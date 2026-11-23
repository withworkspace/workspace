import type { ReactNode } from "react";
import { Button } from "../ui/Button";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import css from "./MissionPlanFold.module.css";

type MissionPlanFoldProps = {
  // Which steps it stands for: those before the ones shown, or after.
  side: "earlier" | "later";
  // How many.
  count: number;
  // They're showing, and this folds them again.
  open?: boolean;
  onToggle: () => void;
  // What's worth knowing of them while they're folded: in amber where
  // something's amiss (warn), in the accent where one's yours.
  note?: { text: string; tone?: "warn" | "yours" };
  // Each of them, as a MissionPlanPip.
  children?: ReactNode;
};

// MissionPlanFold is the steps of a long plan that aren't shown, as a row
// where they are in it: how many, what's worth knowing of them, and each
// as a pip to point at. The plan's line goes on through it, dotted: steps
// are there. Pressed, it shows them, and then folds them again.
export function MissionPlanFold({ side, count, open, onToggle, note, children }: MissionPlanFoldProps) {
  const up = (side === "earlier") !== !!open;
  return (
    <li>
      <Button
        variant="bare"
        className={cx(css.fold, side === "earlier" ? css.earlier : css.end)}
        onClick={onToggle}
        aria-expanded={!!open}
        aria-label={open ? undefined : `Show the ${count} ${side} steps`}
      >
        <span className={css.words}>
          {open ? `Fold ${count} ${side}` : `${count} ${side}`}
          {!open && note && <span className={cx(css.note, note.tone === "warn" && css.warn, note.tone === "yours" && css.yours)}>{note.text}</span>}
        </span>
        <Icon name={up ? "chevron-up" : "chevron-down"} size={12} className={css.chevron} />
        {!open && <span className={css.pips}>{children}</span>}
      </Button>
    </li>
  );
}
