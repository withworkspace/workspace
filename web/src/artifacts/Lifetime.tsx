import { useMemo, useState } from "react";
import { type ArtifactDone, type ArtifactInfo, clearExpiredArtifacts, discardArtifact, getArtifactDetail, promoteArtifact, restoreArtifact } from "../api";
import { useLoaded } from "../load";
import { Banner } from "../ui/Banner";
import { Icon } from "../ui/Icon";
import { toast } from "../ui/toast";
import { humanError } from "../ui/text";
import { Callout } from "../ui/Callout";
import { Keys } from "../ui/Keys";
import { confirm } from "../ui/Confirm";
import { Dialog, DialogActions, DialogText } from "../ui/Dialog";
import { Option, Select } from "../ui/Select";
import { clearable, type IntoKind, type Lives, livesAt, promoteBlocked, promoteVersions, promotion } from "./shown";
import { Button } from "../ui/Button";
import { Input } from "../ui/Input";
import { TextArea } from "../ui/TextArea";
import { useWorkspace } from "../workspace/store";

// An artifact's lifetime, in the web UI: Promote, with the version chosen,
// what it went into, and, the first time, where it's to live (a folder in
// a repo, for everyone with it, or this machine only); Discard, asking
// why; Restore; and Clear expired on the list. For one that lives in a
// repo, promoting, discarding and restoring it are each a commit landed on
// that repo's default branch, so each says so before it's done.

// doneWords says what promoting, discarding or restoring did, for a toast.
function doneWords(what: string, d: ArtifactDone): string {
  let s = `${what} ${d.artifact.id}`;
  if (d.into) s += `, into ${d.into}`;
  if (d.home) s += what === "Discarded" ? `, and took it out of ${d.home}` : what === "Restored" ? `, and put it back in ${d.home}` : `, and put v${d.artifact.promoted?.version} in ${d.home}`;
  if (d.landed) s += `: ${d.landed}`;
  if (d.started) s += `. Mission ${d.started} waits to be briefed.`;
  return s;
}

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
  const lands = lives !== "here";
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
  const keys = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && (e.metaKey || e.ctrlKey || (e.target as HTMLElement).tagName === "INPUT")) {
      e.preventDefault();
      go();
    }
  };
  const latest = versions[0];
  const branch = workspace.defaultBranch ?? "default branch";

  return (
    <Dialog title={`Promote “${a.title}”`} onClose={onClose} className="artifact-promote" wide>
      <DialogText>Promoting keeps it for good: it expires no more, and says which version was chosen and what it went into. It still takes new versions.</DialogText>
      <div className="file-as-fields" onKeyDown={keys}>
        <label>
          <span>Version</span>
          <Select fill value={String(n)} onChange={(v) => setChosen(Number(v))} aria-label="The version chosen">
            {versions.map((v) => {
              const note = d?.all.find((x) => x.n === v)?.note;
              return (
                <Option key={v} value={String(v)} title={note}>
                  {`v${v}${v === latest ? " (the latest)" : ""}${home?.version === v ? ` (in ${home.repo})` : ""}${note ? `: ${note}` : ""}`}
                </Option>
              );
            })}
          </Select>
        </label>
        <label>
          <span>Into</span>
          <Select fill value={kind} onChange={(k) => setKind(k as IntoKind)} aria-label="What it went into">
            <Option value="nothing">Nothing: it's kept</Option>
            <Option value="mission">A mission</Option>
            <Option value="start">A new mission, started from it</Option>
            <Option value="document">A document, which cites it</Option>
          </Select>
        </label>
        {kind === "mission" && (
          <label>
            <span>Mission</span>
            <Select fill value={text} onChange={setText} aria-label="The mission">
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
            <Input autoFocus value={text} placeholder="What the mission is: build the glow" onChange={(e) => setText(e.target.value)} aria-label="What the mission is" />
          </label>
        )}
        {kind === "document" && (
          <label>
            <span>Document</span>
            <Input autoFocus className="mono" value={text} placeholder="repo:path/to/plan.md" onChange={(e) => setText(e.target.value)} aria-label="The document, as repo:path" />
          </label>
        )}
        {!home && d && (
          <label>
            <span>Lives</span>
            <Select fill value={lives} onChange={(l) => setPicked(l as Lives)} aria-label="Where it's to live">
              {offered ? (
                <Option value="offered" note="for everyone with it">
                  {offered}
                </Option>
              ) : null}
              <Option value="here">This machine only</Option>
              <Option value="other">Another folder in a repo…</Option>
            </Select>
          </label>
        )}
        {!home && lives === "other" && (
          <label>
            <span>Folder</span>
            <Input autoFocus className="mono" value={other} placeholder="repo:folder/" onChange={(e) => setOther(e.target.value)} aria-label="The folder it's to live in, as repo:folder/" />
          </label>
        )}
      </div>
      {kind === "mission" || kind === "start" ? (
        <p className="dim file-as-what">It's recorded on the mission, among its artifacts, so whoever briefs it and its crew read it.</p>
      ) : null}
      {home ? (
        <Callout tone="attention" icon="branch" title={`It lives in ${home.where}`}>
          {home.version === n ? `v${n} is there already: its record there is brought up to what's said here` : `v${home.version} is there now, and v${n} takes its place`}, in one commit straight to {home.repo}'s {branch}.
          Every version stays on this machine.
        </Callout>
      ) : lives === "here" ? (
        <Callout tone="info" icon="monitor" title="It stays on this machine">
          Nobody else has it. Promote it again to give it a folder in a repo, for everyone with the repo.
        </Callout>
      ) : d ? (
        <Callout tone="attention" icon="branch" title="Where it lives is chosen once">
          v{n} is put in {folder.trim() || "that folder"}, with a record of it, in one commit straight to its repo's {branch}. Promote it again, versions later, and the one chosen then goes to the same folder, with
          nothing asked. Every version stays on this machine.
        </Callout>
      ) : null}
      {blocked && <p className="dim file-as-what">{blocked}</p>}
      {(error || loadError) && <Banner>{error ?? loadError}</Banner>}
      <DialogActions>
        <Button variant="ghost" onClick={onClose} disabled={busy}>
          Cancel
        </Button>
        <Button variant="primary" onClick={go} disabled={busy || !n || !d || !!blocked}>
          {busy ? (lands ? "Landing…" : "Promoting…") : lands ? `Promote, and land on ${branch}` : "Promote"} <Keys k="mod+⏎" />
        </Button>
      </DialogActions>
    </Dialog>
  );
}

// DiscardDialog discards an artifact, asking why (which can be left
// empty).
export function DiscardDialog({ a, onClose, onDone }: { a: ArtifactInfo; onClose: () => void; onDone?: (d: ArtifactDone) => void }) {
  const defaultBranch = useWorkspace((w) => w.defaultBranch);
  const [why, setWhy] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  const home = a.home;
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
      <DialogText>
        It's taken off the list, with its versions. Its folder moves to <span className="mono">.ws/artifacts/.discarded</span>, and is deleted for good in 30 days: until then, <em>Restore</em> puts it back.{" "}
        {home && (
          <>
            Its folder in {home.repo} (<span className="mono">{home.where}</span>) is taken away too, in one commit straight to {home.repo}'s {defaultBranch ?? "default branch"}.{" "}
          </>
        )}
        {a.id} still says what it was.
      </DialogText>
      <TextArea
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
      {error && <Banner>{error}</Banner>}
      <DialogActions>
        <Button variant="ghost" onClick={onClose} disabled={busy}>
          Cancel
        </Button>
        <Button variant="danger" onClick={go} disabled={busy}>
          {busy ? (home ? "Landing…" : "Discarding…") : home ? `Discard, and land on ${defaultBranch ?? "it"}` : "Discard"} <Keys k="mod+⏎" />
        </Button>
      </DialogActions>
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

// restore puts a discarded artifact back, once you've said so. (Where it
// lived in a repo, it's put back there too: the server says so in what
// it did, as a discarded one's home isn't sent.)
export async function restore(a: ArtifactInfo): Promise<ArtifactDone | undefined> {
  const yes = await confirm({
    title: `Restore “${a.title}”?`,
    body: <>Its folder comes back from .ws/artifacts/.discarded, as it was, and where it lived in a repo, the version that was there is put back, in one commit. Its expiry counts from now.</>,
    confirm: "Restore it",
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

// ClearExpired is the list's Clear expired: every expired artifact,
// discarded now, as the daemon would within the day.
export function ClearExpired({ all }: { all: ArtifactInfo[] }) {
  const [busy, setBusy] = useState(false);
  const expired = clearable(all);
  if (!expired.length) return null;
  const clear = async () => {
    const yes = await confirm({
      title: `Clear ${expired.length} expired ${expired.length === 1 ? "artifact" : "artifacts"}?`,
      body: (
        <>
          {expired.map((a) => a.id).join(", ")} {expired.length === 1 ? "is" : "are"} discarded, as the daemon would within the day. Each can be restored.
        </>
      ),
      confirm: "Clear them",
      danger: true,
    });
    if (!yes) return;
    setBusy(true);
    try {
      const c = await clearExpiredArtifacts();
      toast(c.local.length ? `Cleared ${c.local.join(", ")}` : "Nothing's expired now", { tone: "done", ms: 6000 });
    } catch (e) {
      toast(humanError((e as Error).message), { tone: "danger", ms: 8000 });
    } finally {
      setBusy(false);
    }
  };
  return (
    <Button variant="ghost" size="small" onClick={clear} disabled={busy} title="Discard every expired artifact now">
      <Icon name="clock" size={13} /> {busy ? "Clearing…" : `Clear expired ${expired.length}`}
    </Button>
  );
}
