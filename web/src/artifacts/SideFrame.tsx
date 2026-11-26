import { useMemo } from "react";
import { type ArtifactDetail, frameURL } from "../api";
import { Button } from "../ui/Button";
import { ArtifactEmpty } from "./ArtifactEmpty";
import { ArtifactFrame } from "./ArtifactFrame";
import type { Side } from "./pair";
import type { FrameTweaks } from "./useTweaks";

// SideFrame is one side's version, running: in its frame, at the width
// both are drawn at. It opens in the state the address gives its side
// when it's made, and is told of changes after (as an artifact's page
// does).
export function SideFrame({
  side,
  d,
  px,
  run,
  stopped,
  onRun,
  tweaks,
  opensAs,
}: {
  side: Side;
  d: ArtifactDetail;
  px: number;
  run: number;
  stopped: boolean;
  onRun: () => void;
  tweaks: FrameTweaks;
  opensAs: string;
}) {
  // eslint-disable-next-line react-hooks/exhaustive-deps
  const src = useMemo(() => frameURL(d.id, side.n, opensAs), [d.id, side.n, run, stopped]);
  const v = d.all.find((x) => x.n === side.n);
  if (!v) {
    return (
      <ArtifactEmpty icon="search" title={`${d.id} has no v${side.n}`}>
        Versions: {d.all.map((x) => `v${x.n}`).join(", ")}.
      </ArtifactEmpty>
    );
  }
  if (d.gone || !v.size) {
    return (
      <ArtifactEmpty icon="alert" title="No file to run">
        {d.gone ? `Folder gone from ${d.where.replace(/[^/]+\/$/, "")}.` : `v${v.n}'s files are missing from ${d.where}.`}
      </ArtifactEmpty>
    );
  }
  if (stopped) {
    return (
      <ArtifactEmpty icon="stop" title="Stopped" action={<Button onClick={onRun} keys="x">Run</Button>}>
        Frames and scripts unloaded.
      </ArtifactEmpty>
    );
  }
  return <ArtifactFrame src={src} title={`${d.title}, v${v.n}`} px={px} frameKey={`${d.id}.v${v.n}#${run}`} frameRef={tweaks.frame} />;
}
