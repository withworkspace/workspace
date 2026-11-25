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
  return (
    <div className={css.nav}>
      <Button size="small" className={css.button} type="button" onClick={onBefore} title="The change before" keys="k" aria-label="The change before">
        <Icon name="chevron-up" size={12} /> <Keys k="k" />
      </Button>
      <Button size="small" className={css.button} type="button" onClick={onNext} title="The next change" keys="j" aria-label="The next change">
        <Icon name="chevron-down" size={12} /> <Keys k="j" />
      </Button>
      <span className={css.spacer} />
      <Button size="small" className={css.button} type="button" onClick={onAcceptAll} title="Take each as it's proposed">
        Accept all
      </Button>
    </div>
  );
}
