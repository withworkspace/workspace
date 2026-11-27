import { useMemo, useState } from "react";
import { useNavigate } from "react-router";
import type { ArtifactInfo } from "../api";
import { useCommands } from "../keymap/useCommands";
import { artifactsKeys } from "./keys";
import { Rows } from "../ui/Rows";
import { useTitle } from "../ui/useTitle";
import { useWorkspace } from "../workspace/store";
import { ArtifactsHead } from "./ArtifactsHead";
import { ArtifactsNone } from "./ArtifactsNone";
import { ClearExpired } from "./ClearExpired";
import { FoundArtifacts } from "./FoundArtifacts";
import { artifactHref } from "./hrefs";
import { ListRow } from "./ListRow";
import { PromoteDialog } from "./PromoteDialog";
import { ANYWHERE, listed, type Shown, sources } from "./shown";

// Artifacts: versioned HTML prototypes, listed, and each run in a
// sandboxed frame (ArtifactPage.tsx). A version is its page and the files
// it loads beside it.

// ArtifactsPage is the list: this machine's artifacts, by where they
// stand and where they came from, and under them those found in the
// workspace's repos.
export function ArtifactsPage() {
  const artifacts = useWorkspace((w) => w.artifacts);
  useTitle("Artifacts");
  const navigate = useNavigate();
  const all = useMemo(() => artifacts ?? [], [artifacts]);
  const [shown, setShown] = useState<Shown>("all");
  const [from, setFrom] = useState(ANYWHERE);
  const list = useMemo(() => listed(all, shown, from), [all, shown, from]);
  const froms = useMemo(() => sources(all), [all]);
  const discarded = all.filter((a) => a.status === "discarded").length;
  const [index, setIndex] = useState(0);
  const [promoting, setPromoting] = useState<ArtifactInfo>();
  const move = (i: number) => {
    const n = Math.min(Math.max(i, 0), list.length - 1);
    setIndex(n);
    document.getElementById(`artifact-${list[n]?.id}`)?.scrollIntoView({ block: "nearest" });
  };
  const open = () => list[index] && navigate(artifactHref(list[index].id));
  useCommands(artifactsKeys, { next: () => move(index + 1), prev: () => move(index - 1), open });

  return (
    <div className="page narrow">
      <ArtifactsHead filters={all.length > 0} shown={shown} onShown={setShown} discarded={discarded} froms={froms} from={from} onFrom={setFrom}>
        <ClearExpired all={all} />
      </ArtifactsHead>
      {all.length === 0 ? (
        <ArtifactsNone />
      ) : list.length === 0 ? (
        <ArtifactsNone shown={shown} />
      ) : (
        <Rows role="listbox" aria-label="Artifacts">
          {list.map((a, i) => (
            <ListRow key={a.id} a={a} selected={i === index} onHover={() => i !== index && setIndex(i)} onPromote={() => setPromoting(a)} />
          ))}
        </Rows>
      )}
      <FoundArtifacts />
      {promoting && <PromoteDialog a={promoting} onClose={() => setPromoting(undefined)} />}
    </div>
  );
}
