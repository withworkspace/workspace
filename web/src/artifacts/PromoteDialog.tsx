import { useMemo, useState } from "react";
import { type ArtifactDone, type ArtifactInfo, getArtifactDetail, promoteArtifact } from "../api";
import { useLoaded } from "../load";
import { humanError } from "../ui/strings";
import { toast } from "../ui/toast";
import { useWorkspace } from "../workspace/store";
import { ArtifactPromote } from "./ArtifactPromote";
import { doneWords } from "./doneWords";
import { type IntoKind, type Lives, livesAt, promoteBlocked, promoteVersions, promotion } from "./shown";

// PromoteDialog promotes an artifact: the version chosen (its latest, at
// first), what it went into, if anything, and where it's to live. That
// last is asked once: one that lives in a repo goes there.
export function PromoteDialog({ a, version, onClose, onDone }: { a: ArtifactInfo; version?: number; onClose: () => void; onDone?: (d: ArtifactDone) => void }) {
  const workspace = useWorkspace();
  const { value: d, error: loadError } = useLoaded(() => getArtifactDetail(a.id), [a.id]);
  const versions = useMemo(() => (d ? promoteVersions(d.all) : []), [d]);
  const [chosen, setChosen] = useState<number>();
  const n = chosen ?? (version && versions.includes(version) ? version : versions[0]) ?? 0;
  const [kind, setKind] = useState<IntoKind>(a.promoted?.into ? (workspace.missions.some((m) => m.slug === a.promoted?.into) ? "mission" : "document") : "nothing");
  const [text, setText] = useState(a.promoted?.into ?? "");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  // Where it's to live: what the workspace offers, once that's read,
  // unless another's picked.
  const home = d?.home ?? a.home;
  const offered = home ? undefined : d?.homeOffered;
  const [picked, setPicked] = useState<Lives>();
  const lives = picked ?? livesAt({ home, here: a.here }, offered);
  const [other, setOther] = useState(workspace.artifactsRepo ? `${workspace.artifactsRepo}:` : "");
  const folder = lives === "offered" ? (offered ?? "") : lives === "other" ? other : "";
  // Missions still going: an ended one has nothing left to build it.
  const missions = workspace.missions.filter((m) => !["landed", "answered", "handed-on", "dropped", "archived"].includes(m.state));

  const blocked = promoteBlocked(kind, text, lives, folder);
  const go = async () => {
    if (busy || !n || !d || blocked) return;
    setBusy(true);
    setError(undefined);
    try {
      const done = await promoteArtifact(a.id, promotion(n, kind, text, lives, folder));
      toast(doneWords("Promoted", done), { tone: "done", ms: 6000 });
      onDone?.(done);
      onClose();
    } catch (e) {
      setError(humanError((e as Error).message));
    } finally {
      setBusy(false);
    }
  };

  return (
    <ArtifactPromote
      title={a.title}
      ready={!!d}
      versions={versions.map((v) => ({ n: v, note: d?.all.find((x) => x.n === v)?.note }))}
      version={n}
      onVersion={setChosen}
      kind={kind}
      onKind={setKind}
      text={text}
      onText={setText}
      missions={missions}
      home={home}
      offered={offered}
      lives={lives}
      onLives={setPicked}
      other={other}
      onOther={setOther}
      folder={folder}
      branch={workspace.defaultBranch ?? "default branch"}
      blocked={blocked}
      error={error ?? loadError}
      busy={busy}
      onGo={go}
      onClose={onClose}
    />
  );
}
