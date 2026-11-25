import { HoverCard, HoverCardHead, HoverCardLine, HoverCardTitle } from "../ui/HoverCard";
import { Keys } from "../ui/Keys";
import { hintOnClick } from "../ui/keyHint";
import { segmentClass, segmentedClass } from "../ui/Segmented";
import { Time } from "../ui/Time";
import css from "./ArtifactVersions.module.css";
import { Text } from "../ui/Text";

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
  // The version before the one shown, and the one after, are what the
  // keys step to: pressed with the pointer, each hints at its key.
  const at = versions.findIndex((x) => x.n === current);
  const keyOf = (n: number) => (at > 0 && versions[at - 1].n === n ? keys[0] : at >= 0 && versions[at + 1]?.n === n ? keys[1] : undefined);
  return (
    <div className={segmentedClass({}, css.versions)} role="group" aria-label="Versions">
      {versions.map((x) => (
        <HoverCard
          key={x.n}
          className={css.host}
          trigger={
            <button data-ui="" aria-pressed={x.n === current} data-selected={x.n === current || undefined} onClick={hintOnClick(keyOf(x.n), () => onPick(x.n))} tabIndex={-1} className={segmentClass(x.home ? css.atHome : undefined)}>
              v{x.n}
            </button>
          }
        >
          <HoverCardHead>
            <HoverCardTitle>v{x.n}</HoverCardTitle>
            <Text dim>
              {x.by}, <Time iso={x.at} as="relative" />
            </Text>
          </HoverCardHead>
          <HoverCardLine>{x.note || <Text dim>No note.</Text>}</HoverCardLine>
          {x.home && (
            <HoverCardLine dim>
              It's the version in {x.home.repo}: {x.home.where}
            </HoverCardLine>
          )}
        </HoverCard>
      ))}
      <Text dim className={css.keys}>
        <Keys k={keys[0]} /> <Keys k={keys[1]} />
      </Text>
    </div>
  );
}
