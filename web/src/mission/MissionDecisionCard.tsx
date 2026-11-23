import type { ReactNode } from "react";
import { Chip } from "../ui/Chip";
import { Link } from "react-router";
import { HoverCardHead, HoverCardLine, hoverCardLinkClass, HoverCardStrong, HoverCardTitle } from "../ui/HoverCard";
import { Icon } from "../ui/Icon";

// MissionDecisionCard is what a hover card says of a decision of a
// mission's: which it is, whether it still stands, what was decided and
// why, who decided it, when, and the way to it among the mission's
// decisions.
export function MissionDecisionCard({ id, stands, what, why, by, at, to }: { id: string; stands?: "Replaced" | "Withdrawn"; what: ReactNode; why?: ReactNode; by: string; at: ReactNode; to?: string }) {
  return (
    <>
      <HoverCardHead>
        <Icon name="gavel" size={13} className="dim" />
        <HoverCardTitle>Decision {id}</HoverCardTitle>
        {stands && <Chip tone="quiet">{stands}</Chip>}
      </HoverCardHead>
      <HoverCardStrong>{what}</HoverCardStrong>
      {why && <span className="dim">{why}</span>}
      <HoverCardLine dim>
        {by}, {at}
      </HoverCardLine>
      {to && (
        <Link to={to} className={hoverCardLinkClass()}>
          See it in Decisions <Icon name="arrow-right" size={11} />
        </Link>
      )}
    </>
  );
}
