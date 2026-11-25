import { Button } from "../../ui/Button";
import { Icon } from "../../ui/Icon";
import { Spinner } from "../../ui/Spinner";
import css from "./MissionBriefStart.module.css";

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
      <p className="dim">Its planner reads round what you asked for, asks what it needs to, and writes the brief and a plan with you. They're here to read, change and confirm.</p>
      <div className={css.act}>
        <Button variant="primary" type="button" onClick={onBrief} disabled={busy} aria-busy={busy}>
          {busy ? <Spinner /> : <Icon name="pencil" />} Brief it
        </Button>
        <Button variant="ghost" type="button" onClick={onOwn}>
          Write it yourself
        </Button>
      </div>
    </div>
  );
}
