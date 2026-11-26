import { Button } from "../../ui/Button";
import { Icon } from "../../ui/Icon";
import { Spinner } from "../../ui/Spinner";
import css from "./MissionBriefStart.module.css";
import { Text } from "../../ui/Text";

type MissionBriefStartProps = {
  // What was asked for, as it was said.
  request?: string;
  // It's being sent to be briefed.
  busy?: boolean;
  onBrief: () => void;
  onOwn: () => void;
};

// MissionBriefStart is a mission waiting to be briefed, with no planner
// yet: what was asked for, and the two ways on, having it briefed or
// writing the brief yourself.
export function MissionBriefStart({ request, busy, onBrief, onOwn }: MissionBriefStartProps) {
  return (
    <div className={css.start}>
      <span className={css.icon}>
        <Icon name="pencil" size={18} />
      </span>
      <h2>Waiting to be briefed</h2>
      {request && <blockquote className={css.request}>{request}</blockquote>}
      <Text as="p" dim>A planner asks what it needs and writes the brief and plan with you. Review, edit and confirm them here.</Text>
      <div className={css.act}>
        <Button variant="primary" type="button" onClick={onBrief} disabled={busy} aria-busy={busy}>
          {busy ? <Spinner /> : <Icon name="pencil" />} Brief mission
        </Button>
        <Button variant="ghost" type="button" onClick={onOwn}>
          Write the brief yourself
        </Button>
      </div>
    </div>
  );
}
