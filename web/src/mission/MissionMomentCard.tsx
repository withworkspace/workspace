import { HoverCardHead, HoverCardStrong } from "../ui/HoverCard";
import { Icon, type IconName } from "../ui/Icon";
import { Inline } from "../ui/Inline";
import { cx } from "../ui/cx";
import css from "./MissionMomentCard.module.css";

// A moment in a mission's life that its timeline marks: a stage accepted
// (done), something you did (you), a decision (decided), something asked
// of you (ask); what it was, when, and what the log says of it.
export type Moment = { kind: "done" | "you" | "decided" | "ask"; what: string; when: string; text: string };

const icons: Record<Moment["kind"], IconName> = { done: "check", you: "eye", decided: "gavel", ask: "question" };
const kinds: Record<Moment["kind"], string> = { done: css.done, you: css.you, decided: css.decided, ask: css.ask };

// MissionMomentCard is what a hover card says of the moments at one tick
// of a mission's timeline: each, its kind, when, and what happened.
export function MissionMomentCard({ moments }: { moments: Moment[] }) {
  return (
    <>
      {moments.map((m, i) => (
        <div key={i} className={css.moment}>
          <HoverCardHead>
            <span className={cx(css.kind, kinds[m.kind])}>
              <Icon name={icons[m.kind]} size={9} />
            </span>
            <HoverCardStrong>{m.what}</HoverCardStrong>
            <span className={css.when}>{m.when}</span>
          </HoverCardHead>
          <Inline text={m.text} lines={3} className={css.what} />
        </div>
      ))}
    </>
  );
}
