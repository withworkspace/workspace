import type { ReactNode } from "react";
import { Link } from "react-router";
import { HoverCardHead, HoverCardLine, hoverCardLinkClass, HoverCardLinks, HoverCardTitle } from "../ui/HoverCard";
import { Icon } from "../ui/Icon";
import { textClass } from "../ui/Text";

// MissionCapsuleCard is what a hover card says of a capsule of a
// mission's: its name, its branch, where it stands (its badges), and,
// while it's there, the ways to its changes and its files.
export function MissionCapsuleCard({ name, branch, badges, review, files }: { name: string; branch?: string; badges?: ReactNode; review?: string; files?: string }) {
  return (
    <>
      <HoverCardHead>
        <Icon name="box" size={13} className={textClass("dim")} />
        <HoverCardTitle className={textClass("mono")}>{name}</HoverCardTitle>
      </HoverCardHead>
      {branch && (
        <HoverCardLine dim>
          <Icon name="branch" size={11} /> {branch}
        </HoverCardLine>
      )}
      {badges}
      {(review || files) && (
        <HoverCardLinks>
          {review && (
            <Link to={review} className={hoverCardLinkClass()}>
              Review its changes <Icon name="arrow-right" size={11} />
            </Link>
          )}
          {files && (
            <Link to={files} className={hoverCardLinkClass()}>
              Its files <Icon name="arrow-right" size={11} />
            </Link>
          )}
        </HoverCardLinks>
      )}
    </>
  );
}
