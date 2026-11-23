import { cx } from "../ui/cx";
import { crewHue } from "./crewHue";
import css from "./MissionCrewBadge.module.css";

// MissionCrewBadge is a crew's name in its own colour, the same wherever
// it shows in its mission: live, one at work now, with a ring; large, at
// the head of its own page.
export function MissionCrewBadge({ id, live, large, className }: { id: string; live?: boolean; large?: boolean; className?: string }) {
  return (
    <span className={cx(css.badge, live && css.live, large && css.large, className)} style={{ "--hue": crewHue(id) } as React.CSSProperties}>
      {id}
    </span>
  );
}
