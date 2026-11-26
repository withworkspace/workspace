import { CrewBadge } from "../mission/CrewBadge";
import css from "./FootCrew.module.css";
import { FootItem } from "./FootItem";

type FootCrewProps = {
  // Which crew, and the mission it's on (a short name for it).
  id: string;
  mission: string;
  // What it's doing this moment, in a few words ("Editing Foot.tsx"),
  // once that's known.
  doing?: string;
  // Where it's watched at work: its own page.
  to: string;
};

// FootCrew is a crew at work, in the foot: who, on what, and what it's
// doing this moment. It's a window on to what's going on, there to be
// looked through when you're curious, not something to act on: a press
// goes to the crew at work. It gives way when the foot is short of room:
// what it's doing and the mission are cut short, and its badge stays.
export function FootCrew({ id, mission, doing, to }: FootCrewProps) {
  return (
    <FootItem
      shrink
      to={to}
      className={css.crew}
      title={`${id} is working on ${mission}${doing ? `: ${doing}` : ""}`}
    >
      <span className={css.who}>
        <CrewBadge id={id} live />
      </span>
      <span className={css.mission}>{mission}</span>
      {doing && (
        // Keyed by what it says, so a new step comes in as new.
        <span key={doing} className={css.doing} aria-live="off">
          {doing}…
        </span>
      )}
    </FootItem>
  );
}
