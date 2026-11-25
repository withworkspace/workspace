import { HoverCard, HoverCardHead, HoverCardLine, HoverCardTitle } from "../ui/HoverCard";
import { Keys } from "../ui/Keys";
import { segmentClass, segmentedClass } from "../ui/Segmented";
import { Time } from "../ui/Time";
import { cx } from "../ui/cx";
import css from "./ArtifactVersions.module.css";

// A version, as the strip says it: who added it ("by you"), when, its
// note, and, for the one that's in the repo the artifact lives in, where.
export type ArtifactVersionsItem = { n: number; by: string; at: string; note?: string; home?: { repo: string; where: string } };

type ArtifactVersionsProps = {
  versions: ArtifactVersionsItem[];
  // The one shown.
  current: number;
  onPick: (n: number) => void;
  // The keys that step through them, said at the strip's end.
  keys: [string, string];
};

// ArtifactVersions is the versions, v1 v2 v3, the one shown raised, each
// with its note on hover, and the keys that step through them. The one in
// the repo is underlined. It's a segmented control's track, its segments
// each inside a hover card.
export function ArtifactVersions({ versions, current, onPick, keys }: ArtifactVersionsProps) {
  return (
    <div className={segmentedClass({}, css.versions)} role="group" aria-label="Versions">
      {versions.map((x) => (
        <HoverCard
          key={x.n}
          className={css.host}
          trigger={
            <button data-ui="" aria-pressed={x.n === current} data-selected={x.n === current || undefined} onClick={() => onPick(x.n)} tabIndex={-1} className={segmentClass(x.home ? css.atHome : undefined)}>
              v{x.n}
            </button>
          }
        >
          <HoverCardHead>
            <HoverCardTitle>v{x.n}</HoverCardTitle>
            <span className="dim">
              {x.by}, <Time iso={x.at} as="relative" />
            </span>
          </HoverCardHead>
          <HoverCardLine>{x.note || <span className="dim">No note.</span>}</HoverCardLine>
          {x.home && (
            <HoverCardLine dim>
              It's the version in {x.home.repo}: {x.home.where}
            </HoverCardLine>
          )}
        </HoverCard>
      ))}
      <span className={cx(css.keys, "dim")}>
        <Keys k={keys[0]} /> <Keys k={keys[1]} />
      </span>
    </div>
  );
}
