import type { ReactNode } from "react";
import { Link } from "react-router";
import { HoverCardHead, hoverCardLinkClass, HoverCardSaid, HoverCardTitle } from "../ui/HoverCard";
import { Icon } from "../ui/Icon";
import { textClass } from "../ui/Text";

// MissionTalkCard is what a hover card says of a conversation of a
// mission's: what it's about, where it stands (a Status), its summary so
// far, and the way to it.
export function MissionTalkCard({ title, status, summary, to }: { title: ReactNode; status: ReactNode; summary?: ReactNode; to: string }) {
  return (
    <>
      <HoverCardHead>
        <Icon name="message" size={13} className={textClass("dim")} />
        <HoverCardTitle>{title}</HoverCardTitle>
      </HoverCardHead>
      {status}
      {summary && <HoverCardSaid>{summary}</HoverCardSaid>}
      <Link to={to} className={hoverCardLinkClass()}>
        Open the conversation <Icon name="arrow-right" size={11} />
      </Link>
    </>
  );
}
