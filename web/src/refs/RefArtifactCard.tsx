import { artifactStatus, byWords } from "../artifacts/shown";
import { useLoaded } from "../load";
import { HoverCardLine } from "../ui/HoverCard";
import { Inline } from "../ui/Inline";
import { useWorkspace } from "../workspace/store";
import { artifactOf } from "./cards";
import { type PageRef, refHref } from "./refs";
import { RefCardHead } from "./RefCardHead";
import { RefChip } from "./RefChip";
import { RefGo } from "./RefGo";

// RefArtifactCard is an artifact's card, or a version's: its title, which
// version of how many and where it stands, the version's note, and who
// made it and when.
export function RefArtifactCard({ r }: { r: Extract<PageRef, { kind: "artifact" }> }) {
  const artifacts = useWorkspace((w) => w.artifacts);
  const brief = artifacts?.find((a) => a.id === r.id);
  const { value: d, error } = useLoaded(() => (brief ? artifactOf(r.id, brief.versions) : Promise.reject(new Error("not here"))), [r.id, brief?.versions]);
  const n = r.version || brief?.latest?.n || 0;
  const v = d?.all.find((x) => x.n === n) ?? (n === brief?.latest?.n ? brief?.latest : undefined);
  const st = brief && artifactStatus(brief);
  return (
    <>
      <RefCardHead icon="artifact" title={brief?.title ?? r.id} />
      {!brief && <HoverCardLine dim>This machine has no artifact {r.id}.</HoverCardLine>}
      {brief && (
        <HoverCardLine dim>
          {r.version ? `${r.id}.v${r.version}, ` : `${r.id}, `}
          {v ? `v${v.n} of ${brief.versions}` : d ? `it has no v${n}: ${d.all.map((x) => `v${x.n}`).join(", ")}` : `${brief.versions} ${brief.versions === 1 ? "version" : "versions"}`}
          {st && <RefChip tone={st.tone}>{st.words}</RefChip>}
        </HoverCardLine>
      )}
      {v?.note && (
        <HoverCardLine>
          <Inline text={v.note} lines={4} />
        </HoverCardLine>
      )}
      {v && (
        <HoverCardLine dim>
          {byWords(v)}
          {v.from ? `, from ${v.from}` : ""}
          {v.fromTitle ? ` (${v.fromTitle})` : ""}
        </HoverCardLine>
      )}
      {error && brief && <HoverCardLine dim>It can't be read: {error}</HoverCardLine>}
      {brief && <RefGo to={refHref(r)}>Open the artifact</RefGo>}
    </>
  );
}
