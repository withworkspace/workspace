import type { ReactNode } from "react";
import { Chip } from "../ui/Chip";
import { HoverCardHead, HoverCardLine, HoverCardStrong, HoverCardTitle } from "../ui/HoverCard";
import { Icon } from "../ui/Icon";

// MissionDecisionCard is what a hover card says of a decision of a
// mission's: which it is, whether it still stands, what was decided and
// why, and who decided it, when.
export function MissionDecisionCard({ id, stands, what, why, by, at }: { id: string; stands?: "Replaced" | "Withdrawn"; what: ReactNode; why?: ReactNode; by: string; at: ReactNode }) {
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
    </>
  );
}
