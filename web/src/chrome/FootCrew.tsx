import { CrewBadge } from "../mission/parts";
import css from "./FootCrew.module.css";
import { FootItem } from "./FootItem";

type FootCrewProps = {
  // Which crew, and the mission it's on (a short name for it).
  id: string;
  mission: string;
  // Where it's watched at work: its own page.
  to: string;
};

// FootCrew is a crew at work, in the foot: who, and on what. It's a
// window on to what's going on, there to be looked through when you're
// curious, not something to act on: a press goes to the crew at work. It
// gives way when the foot is short of room: the mission is cut short,
// and its badge stays.
export function FootCrew({ id, mission, to }: FootCrewProps) {
  return (
    <FootItem shrink to={to} className={css.crew} title={`${id} is working on ${mission}: go and watch it`}>
      <span className={css.who}>
        <CrewBadge id={id} live />
      </span>
      <span className={css.mission}>{mission}</span>
    </FootItem>
  );
}
