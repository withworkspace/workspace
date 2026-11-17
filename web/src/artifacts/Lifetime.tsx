import { useMemo, useState } from "react";
import { type ArtifactDone, type ArtifactInfo, clearExpiredArtifacts, discardArtifact, getArtifactDetail, promoteArtifact, restoreArtifact, type Workspace } from "../api";
import { useLoaded } from "../load";
import { Icon } from "../ui/Icon";
import { Callout, confirm, Dialog, humanError, Keys, toast } from "../ui/kit";
import { Option, Select } from "../ui/Select";
import { clearable, type IntoKind, promoteBlocked, promoteVersions, promotion } from "./shown";

// An artifact's lifetime, in the web UI: Promote, with the version chosen
// and what it went into, and Promote and move to the repo for one on this
// machine; Discard, asking why; Restore; and Clear expired on the list.
// Each in a repo is a commit landed on its default branch, so each says
// so before it's done.

const isLocal = (a: Pick<ArtifactInfo, "place">) => a.place === ".ws" || !a.place;

// doneWords says what promoting, discarding or restoring did, for a toast.
function doneWords(what: string, d: ArtifactDone): string {
  let s = `${what} ${d.artifact.id}`;
  if (d.into) s += `, into ${d.into}`;
  if (d.moved) s += `, and moved to ${d.artifact.place}`;
  if (d.landed) s += `: ${d.landed}`;
  if (d.started) s += `. Mission ${d.started} waits to be briefed.`;
  return s;
}

// PromoteDialog promotes an artifact: the version chosen (its latest that's
// landed, at first), and what it went into, if anything.
export function PromoteDialog({ a, version, workspace, onClose, onDone }: { a: ArtifactInfo; version?: number; workspace: Workspace; onClose: () => void; onDone?: (d: ArtifactDone) => void }) {
  const { value: d, error: loadError } = useLoaded(() => getArtifactDetail(a.id), [a.id]);
  const versions = useMemo(() => (d ? promoteVersions(d.all) : []), [d]);
  const [chosen, setChosen] = useState<number>();
  const n = chosen ?? (version && versions.includes(version) ? version : versions[0]) ?? 0;
  const [kind, setKind] = useState<IntoKind>(a.promoted?.into ? (workspace.missions.some((m) => m.slug === a.promoted?.into) ? "mission" : "document") : "nothing");
  const [text, setText] = useState(a.promoted?.into ?? "");
  const [busy, setBusy] = useState<"" | "here" | "move">("");
  const [error, setError] = useState<string>();
  const local = isLocal(a);
  const repo = workspace.artifactsRepo;
  // Missions still going: an ended one has nothing left to build it.
  const missions = workspace.missions.filter((m) => !["landed", "answered", "handed-on", "dropped", "archived"].includes(m.state));

  const blockedHere = promoteBlocked(kind, text, local, false, repo);
  const blockedMove = promoteBlocked(kind, text, local, true, repo);
  const go = async (move: boolean) => {
    if (busy || !n || (move ? blockedMove : blockedHere)) return;
    setBusy(move ? "move" : "here");
    setError(undefined);
    try {
      const done = await promoteArtifact(a.id, promotion(n, kind, text, move));
      toast(doneWords("Promoted", done), { tone: "done", ms: 6000 });
      onDone?.(done);
      onClose();
    } catch (e) {
      setError(humanError((e as Error).message));
    } finally {
      setBusy("");
    }
  };
  // ⌘⏎ promotes as the first button would: moving, where only that can.
  const keys = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && (e.metaKey || e.ctrlKey || (e.target as HTMLElement).tagName === "INPUT")) {
      e.preventDefault();
      go(local && !!repo && !!blockedHere && !blockedMove);
    }
  };
  const latest = versions[0];

  return (
    <Dialog title={`Promote “${a.title}”`} onClose={onClose} className="artifact-promote" wide>
      <p className="dialog-text">Promoting keeps it for good: it expires no more, and says which version was chosen and what it went into. It still takes new versions.</p>
      <div className="file-as-fields" onKeyDown={keys}>
        <label>
          <span>Version</span>
          <Select value={String(n)} onChange={(v) => setChosen(Number(v))} aria-label="The version chosen">
            {versions.map((v) => {
              const note = d?.all.find((x) => x.n === v)?.note;
              return (
                <Option key={v} value={String(v)} title={note}>
                  {`v${v}${v === latest ? " (the latest)" : ""}${note ? `: ${note}` : ""}`}
                </Option>
              );
            })}
          </Select>
        </label>
        <label>
          <span>Into</span>
          <Select value={kind} onChange={(k) => setKind(k as IntoKind)} aria-label="What it went into">
            <Option value="nothing">Nothing: it's kept</Option>
            <Option value="mission">A mission</Option>
            <Option value="start">A new mission, started from it</Option>
            <Option value="document">A document, which cites it</Option>
          </Select>
        </label>
        {kind === "mission" && (
          <label>
            <span>Mission</span>
            <Select value={text} onChange={setText} aria-label="The mission">
              <Option value="">Choose one…</Option>
              {missions.map((m) => (
                <Option key={m.slug} value={m.slug} title={m.title}>
                  {`${m.title} (${m.slug})`}
                </Option>
              ))}
            </Select>
          </label>
        )}
        {kind === "start" && (
          <label>
            <span>Mission</span>
            <input autoFocus value={text} placeholder="What the mission is: build the glow" onChange={(e) => setText(e.target.value)} aria-label="What the mission is" />
          </label>
        )}
        {kind === "document" && (
          <label>
            <span>Document</span>
            <input autoFocus className="mono" value={text} placeholder="repo:path/to/plan.md" onChange={(e) => setText(e.target.value)} aria-label="The document, as repo:path" />
          </label>
        )}
      </div>
      {kind === "mission" || kind === "start" ? (
        <p className="dim file-as-what">It's recorded on the mission, among its artifacts, so whoever briefs it and its crew read it.</p>
      ) : null}
      {local ? (
        repo ? (
          <Callout tone="info" icon="monitor" title="It's on this machine only">
            <em>Promote and move to {repo}</em> moves it into {repo}, with all its versions and where each came from, in one commit landed on its {workspace.defaultBranch ?? "default branch"}, and takes it
            from <span className="mono">.ws/artifacts</span>. Its ref stays {a.id}.
          </Callout>
        ) : null
      ) : (
        <Callout tone="attention" icon="branch" title={`Promoting lands on ${a.place}`}>
          It's one commit, marking it promoted in its <span className="mono">artifact.toml</span>, straight to {a.place}'s {workspace.defaultBranch ?? "default branch"}.
        </Callout>
      )}
      {blockedHere && <p className="dim file-as-what">{blockedHere}</p>}
      {(error || loadError) && <div className="banner">{error ?? loadError}</div>}
      <div className="dialog-actions">
        <button className="ghost" onClick={onClose} disabled={!!busy}>
          Cancel
        </button>
        <button className={local && repo && blockedHere && !blockedMove ? "" : "primary"} onClick={() => go(false)} disabled={!!busy || !n || !!blockedHere}>
          {busy === "here" ? (local ? "Promoting…" : "Landing…") : "Promote"}
          {!(local && repo && blockedHere && !blockedMove) && <Keys k="mod+⏎" />}
        </button>
        {local && repo && (
          <button className={blockedHere && !blockedMove ? "primary" : ""} onClick={() => go(true)} disabled={!!busy || !n || !!blockedMove}>
            {busy === "move" ? "Landing…" : `Promote and move to ${repo}`}
            {blockedHere && !blockedMove && <Keys k="mod+⏎" />}
          </button>
        )}
      </div>
    </Dialog>
  );
}

// DiscardDialog discards an artifact, asking why (which can be left
// empty).
export function DiscardDialog({ a, workspace, onClose, onDone }: { a: ArtifactInfo; workspace: Workspace; onClose: () => void; onDone?: (d: ArtifactDone) => void }) {
  const [why, setWhy] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  const local = isLocal(a);
  const go = async () => {
    if (busy) return;
    setBusy(true);
    setError(undefined);
    try {
      const done = await discardArtifact(a.id, why);
      toast(doneWords("Discarded", done), { tone: "done", ms: 6000, action: done.artifact.restorable ? { label: "Restore", run: () => restoreFromToast(a.id) } : undefined });
      onDone?.(done);
      onClose();
    } catch (e) {
      setError(humanError((e as Error).message));
    } finally {
      setBusy(false);
    }
  };
  return (
    <Dialog title={`Discard “${a.title}”`} onClose={onClose}>
      <p className="dialog-text">
        {local ? (
          <>
            It's taken off the list, with its versions. Its folder moves to <span className="mono">.ws/artifacts/.discarded</span>, and is deleted for good in 30 days: until then, <em>Restore</em> puts it back.
          </>
        ) : (
          <>
            It's taken off the list, with its versions, in one commit taking its folder from {a.place}'s {workspace.defaultBranch ?? "default branch"}, landed. History keeps it, and <em>Restore</em> puts it back.
          </>
        )}{" "}
        {a.id} still says what it was.
      </p>
      <textarea
        className="artifact-discard-why"
        autoFocus
        rows={3}
        value={why}
        placeholder="Why? (you can leave it empty)"
        onChange={(e) => setWhy(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
            e.preventDefault();
            go();
          }
        }}
        aria-label="Why it's discarded"
      />
      {error && <div className="banner">{error}</div>}
      <div className="dialog-actions">
        <button className="ghost" onClick={onClose} disabled={busy}>
          Cancel
        </button>
        <button className="danger" onClick={go} disabled={busy}>
          {busy ? (local ? "Discarding…" : "Landing…") : local ? "Discard" : `Discard, and land on ${workspace.defaultBranch ?? "it"}`} <Keys k="mod+⏎" />
        </button>
      </div>
    </Dialog>
  );
}

async function restoreFromToast(id: string) {
  try {
    toast(doneWords("Restored", await restoreArtifact(id)), { tone: "done" });
  } catch (e) {
    toast(humanError((e as Error).message), { tone: "danger" });
  }
}

// restore puts a discarded artifact back, once you've said so.
export async function restore(a: ArtifactInfo, workspace: Workspace): Promise<ArtifactDone | undefined> {
  const local = isLocal(a);
  const yes = await confirm({
    title: `Restore “${a.title}”?`,
    body: local ? (
      <>Its folder comes back from .ws/artifacts/.discarded, as it was. Its expiry counts from now.</>
    ) : (
      <>
        Its folder comes back from history ({a.place}@{a.discarded?.commit?.slice(0, 7)}), in a new commit landed on {a.place}'s {workspace.defaultBranch ?? "default branch"}. Its expiry counts from now.
      </>
    ),
    confirm: local ? "Restore it" : "Restore, and land it",
  });
  if (!yes) return undefined;
  try {
    const done = await restoreArtifact(a.id);
    toast(doneWords("Restored", done), { tone: "done" });
    return done;
  } catch (e) {
    toast(humanError((e as Error).message), { tone: "danger", ms: 8000 });
    return undefined;
  }
}

// ClearExpired is the list's Clear expired: every expired artifact, those
// in the repo taken away in one commit, those here discarded.
export function ClearExpired({ all, workspace }: { all: ArtifactInfo[]; workspace: Workspace }) {
  const [busy, setBusy] = useState(false);
  const expired = clearable(all);
  if (!expired.length) return null;
  const inRepo = expired.filter((a) => !isLocal(a));
  const here = expired.length - inRepo.length;
  const clear = async () => {
    const yes = await confirm({
      title: `Clear ${expired.length} expired ${expired.length === 1 ? "artifact" : "artifacts"}?`,
      body: (
        <>
          {inRepo.length > 0 && (
            <>
              {inRepo.map((a) => a.id).join(", ")} {inRepo.length === 1 ? "is" : "are"} taken from {inRepo[0].place} in one commit, landed on its {workspace.defaultBranch ?? "default branch"}: history keeps{" "}
              {inRepo.length === 1 ? "it" : "them"}.{" "}
            </>
          )}
          {here > 0 && <>{here === 1 ? "The one" : `The ${here}`} on this machine {here === 1 ? "is" : "are"} discarded, as the daemon would. </>}
          Each can be restored.
        </>
      ),
      confirm: inRepo.length ? "Clear, and land it" : "Clear them",
      danger: true,
    });
    if (!yes) return;
    setBusy(true);
    try {
      const c = await clearExpiredArtifacts();
      const ids = [...c.repo, ...c.local];
      toast(ids.length ? `Cleared ${ids.join(", ")}${c.landed ? `: ${c.landed}` : ""}` : "Nothing's expired now", { tone: "done", ms: 6000 });
    } catch (e) {
      toast(humanError((e as Error).message), { tone: "danger", ms: 8000 });
    } finally {
      setBusy(false);
    }
  };
  return (
    <button className="ghost small" onClick={clear} disabled={busy} title="Take every expired artifact away: those in the repo in one commit">
      <Icon name="clock" size={13} /> {busy ? "Clearing…" : `Clear expired ${expired.length}`}
    </button>
  );
}
