import { useSaid } from "../../keymap/keymap";
import { briefKeys } from "./keys";
import { Button } from "../../ui/Button";
import { Icon } from "../../ui/Icon";
import { Keys } from "../../ui/Keys";
import css from "./MissionBriefChangesNav.module.css";

type MissionBriefChangesNavProps = {
  onBefore: () => void;
  onNext: () => void;
  onAcceptAll: () => void;
};

// MissionBriefChangesNav is how the changes still to read are gone
// through from the outline: the one before, the next, or all taken.
export function MissionBriefChangesNav({ onBefore, onNext, onAcceptAll }: MissionBriefChangesNavProps) {
  const prevKey = useSaid(briefKeys.prev);
  const nextKey = useSaid(briefKeys.next);
  return (
    <div className={css.nav}>
      <Button size="small" className={css.button} type="button" onClick={onBefore} title="Previous change" keys={briefKeys.prev} aria-label="Previous change">
        <Icon name="chevron-up" size={12} /> {prevKey && <Keys k={prevKey} />}
      </Button>
      <Button size="small" className={css.button} type="button" onClick={onNext} title="Next change" keys={briefKeys.next} aria-label="Next change">
        <Icon name="chevron-down" size={12} /> {nextKey && <Keys k={nextKey} />}
      </Button>
      <span className={css.spacer} />
      <Button size="small" className={css.button} type="button" onClick={onAcceptAll} title="Accept all remaining changes">
        Accept all
      </Button>
    </div>
  );
}
