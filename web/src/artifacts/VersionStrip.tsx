import type { ArtifactDetail } from "../api";
import { ArtifactVersions } from "./ArtifactVersions";
import { byWords } from "./shown";

// VersionStrip is an artifact's versions, v1 v2 v3, each with its note on
// hover, and the keys that step through them.
export function VersionStrip({ d, n, go, keys = ["[", "]"] }: { d: ArtifactDetail; n: number; go: (n: number) => void; keys?: [string, string] }) {
  return (
    <ArtifactVersions
      versions={d.all.map((x) => ({ n: x.n, by: byWords(x), at: x.at, note: x.note, home: d.home?.version === x.n ? { repo: d.home.repo, where: d.home.where } : undefined }))}
      current={n}
      onPick={go}
      keys={keys}
    />
  );
}
