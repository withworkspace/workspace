import { Button } from "../../ui/Button";
import { Icon } from "../../ui/Icon";
import css from "./MissionBriefUnfold.module.css";

type MissionBriefUnfoldProps = {
  // The planner has something for you while it's folded away.
  waits?: boolean;
  onUnfold: () => void;
};

// MissionBriefUnfold brings the planner's conversation back beside the
// documents, from the tab's bar, with a dot when it has something for you.
export function MissionBriefUnfold({ waits, onUnfold }: MissionBriefUnfoldProps) {
  return (
    <Button variant="ghost" square type="button" className={css.unfold} onClick={onUnfold} title="Show the planner's conversation (p)" aria-label="Show the planner's conversation">
      <Icon name="panel" size={14} />
      {waits && <span className={css.dot} aria-label="It has something for you" />}
    </Button>
  );
}
