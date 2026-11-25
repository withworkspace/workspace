import type { ArtifactDetail, ArtifactVersion } from "../api";
import { ArtifactVersionLine } from "./ArtifactVersionLine";
import { From } from "./From";
import { byWords, filesWords, keptTitle } from "./shown";

// VersionLine is a version in a line: v2 of 3, based on v1 (when it isn't
// the one before), from where, by whom, and when.
export function VersionLine({ d, v }: { d: ArtifactDetail; v: ArtifactVersion }) {
  return (
    <ArtifactVersionLine
      n={v.n}
      of={d.all.length}
      basedOn={v.basedOn && v.basedOn !== v.n - 1 ? v.basedOn : undefined}
      from={v.from && <From from={v.from} title={v.fromTitle} />}
      proposal={v.proposal}
      by={v.from ? byWords(v) : v.by === "you" ? "by you, by hand" : byWords(v)}
      at={v.at}
      size={v.size ? filesWords(v.files, v.size) : undefined}
      kept={d.home?.version === v.n ? { repo: d.home.repo, title: keptTitle(d) } : { title: `On this machine: ${d.where}` }}
    />
  );
}
