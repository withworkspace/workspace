import type { ReactNode } from "react";
import { Link } from "react-router";
import { HoverCardHead, HoverCardLine, hoverCardLinkClass, HoverCardTitle } from "../ui/HoverCard";
import { Icon } from "../ui/Icon";

type MissionDocCardProps = {
  // An artifact (made to look at), not a document to read.
  artifact?: boolean;
  title: ReactNode;
  // What kind of thing it is, and how many versions an artifact has.
  kind: string;
  versions?: number;
  // Where it stands (its state's chip).
  state?: ReactNode;
  // What it's said to be, a few lines.
  desc?: ReactNode;
  // The way to it, where there's one.
  to?: string;
};

// MissionDocCard is what a hover card says of something a mission's
// made: a document or an artifact, what kind it is and where it stands,
// what it's said to be, and the way to it.
export function MissionDocCard({ artifact, title, kind, versions, state, desc, to }: MissionDocCardProps) {
  return (
    <>
      <HoverCardHead>
        <Icon name={artifact ? "artifact" : "file"} size={13} className="dim" />
        <HoverCardTitle>{title}</HoverCardTitle>
      </HoverCardHead>
      <HoverCardLine>
        <span className="dim">{kind}</span>
        {!!versions && <span className="dim">, {versions === 1 ? "1 version" : `${versions} versions`}</span>} {state}
      </HoverCardLine>
      {desc && <span className="dim">{desc}</span>}
      {to && (
        <Link to={to} className={hoverCardLinkClass()}>
          {artifact ? "Open the artifact" : "Read it"} <Icon name="arrow-right" size={11} />
        </Link>
      )}
    </>
  );
}
