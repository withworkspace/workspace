import type { ReactNode } from "react";
import { Link } from "react-router";
import { HoverCardHead, HoverCardLine, hoverCardLinkClass, HoverCardSaid } from "../ui/HoverCard";
import { Icon } from "../ui/Icon";

type MissionCrewCardProps = {
  // Its badge, and where it stands (a Status).
  badge: ReactNode;
  status: ReactNode;
  // What it is: its agent and model, and the capsule it's in.
  what: string;
  capsule?: string;
  // When it started, and ended (Times).
  started: ReactNode;
  ended?: ReactNode;
  // What it last said, or what's noted of it.
  said?: ReactNode;
  // Where to read more of it, and what that's called.
  to?: string;
  more?: string;
};

// MissionCrewCard is what a hover card says of a mission's crew: who it
// is, where it stands, when it ran, what it last said, and the way to its
// page.
export function MissionCrewCard({ badge, status, what, capsule, started, ended, said, to, more }: MissionCrewCardProps) {
  return (
    <>
      <HoverCardHead>
        {badge}
        {status}
      </HoverCardHead>
      <HoverCardLine>
        {what}
        {capsule && <span className="dim"> in {capsule}</span>}
      </HoverCardLine>
      <HoverCardLine dim>
        Started {started}
        {ended && <>, ended {ended}</>}
      </HoverCardLine>
      {said && <HoverCardSaid>{said}</HoverCardSaid>}
      {to && (
        <Link to={to} className={hoverCardLinkClass()}>
          {more} <Icon name="arrow-right" size={11} />
        </Link>
      )}
    </>
  );
}
