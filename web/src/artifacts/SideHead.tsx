import { type ArtifactDetail, frameURL, type Workspace } from "../api";
import { Inline } from "../ui/Inline";
import { ArtifactCompareSide } from "./ArtifactCompareSide";
import { ArtifactNote } from "./ArtifactNote";
import { type Pair, withSide } from "./pair";
import { TweaksButton } from "./TweaksButton";
import type { FrameTweaks } from "./useTweaks";
import { VersionLine } from "./VersionLine";
import { VersionStrip } from "./VersionStrip";

// SideHead is one side's head: which artifact (any of the list), its
// versions, the version's line and note, and Open alone.
export function SideHead({
  i,
  pair,
  d,
  choices,
  onPick,
  go,
  keys,
  tweaks,
  opensAs,
  framed,
}: {
  i: 0 | 1;
  pair: Pair;
  d: ArtifactDetail;
  choices: Workspace["artifacts"];
  onPick: (id: string) => void;
  go: (p: Pair) => void;
  keys: [string, string]; // that step its versions
  tweaks: FrameTweaks;
  opensAs: string; // the state it's in, as a frame's address says it
  framed: boolean; // its frame's on the page: not the diff, not stopped
}) {
  const side = pair[i];
  const v = d.all.find((x) => x.n === side.n);
  const runnable = !d.gone && !!v?.size;
  return (
    <ArtifactCompareSide
      label={i ? "Right artifact" : "Left artifact"}
      picked={side.id}
      choices={choices ?? []}
      onPick={onPick}
      versions={<VersionStrip d={d} n={side.n} go={(n) => go(withSide(pair, i, { id: side.id, n }))} keys={keys} />}
      tweaks={framed && <TweaksButton told={tweaks.told} set={tweaks.set} label={i ? "Right tweaks" : "Left tweaks"} />}
      alone={runnable ? frameURL(d.id, side.n, opensAs) : undefined}
    >
      {v && <VersionLine d={d} v={v} />}
      {v?.note && (
        <ArtifactNote dim>
          <Inline text={v.note} lines={2} />
        </ArtifactNote>
      )}
    </ArtifactCompareSide>
  );
}
