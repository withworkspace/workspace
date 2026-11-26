import type { ReactNode } from "react";
import { Button } from "../../ui/Button";
import { Chip } from "../../ui/Chip";
import { Icon } from "../../ui/Icon";
import css from "./MissionBriefNav.module.css";

type MissionBriefNavProps = {
  // How many there are, or which is in hand.
  children: ReactNode;
  onBefore: () => void;
  onNext: () => void;
};

// MissionBriefNav is the changes still to read, stepped through from the
// tab's bar: where the outline, which has them on a wide window, isn't.
export function MissionBriefNav({ children, onBefore, onNext }: MissionBriefNavProps) {
  return (
    <span className={css.nav}>
      <Chip tone="attention" className={css.chip}>{children}</Chip>
      <Button variant="ghost" square type="button" onClick={onBefore} title="Previous change" keys="k" aria-label="Previous change">
        <Icon name="chevron-up" size={13} />
      </Button>
      <Button variant="ghost" square type="button" onClick={onNext} title="Next change" keys="j" aria-label="Next change">
        <Icon name="chevron-down" size={13} />
      </Button>
    </span>
  );
}
