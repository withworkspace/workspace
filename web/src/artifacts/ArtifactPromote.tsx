import type { KeyboardEvent } from "react";
import { Banner } from "../ui/Banner";
import { Button } from "../ui/Button";
import { Callout } from "../ui/Callout";
import { Dialog, DialogActions, DialogText } from "../ui/Dialog";
import { Input } from "../ui/Input";
import { Keys } from "../ui/Keys";
import { Option, Select } from "../ui/Select";
import type { IntoKind, Lives } from "./shown";

type ArtifactPromoteProps = {
  // The artifact's title.
  title: string;
  // ready is once the artifact's been read: until then there's nothing to
  // choose a version of, or to say of where it'll live.
  ready: boolean;
  // The versions it can be promoted at, newest first, each with its note,
  // and the one chosen.
  versions: { n: number; note?: string }[];
  version: number;
  onVersion: (n: number) => void;
  // What it went into, and what names that: a mission's slug, what a new
  // mission is, or a document as repo:path.
  kind: IntoKind;
  onKind: (kind: IntoKind) => void;
  text: string;
  onText: (text: string) => void;
  // The missions it can go into.
  missions: { slug: string; title: string }[];
  // Where it lives already, in a repo: then that's not asked.
  home?: { where: string; repo: string; version: number };
  // Where it's to live, asked the first time: the folder the workspace
  // offers, which is picked, another folder as it's typed, and the folder
  // that comes to.
  offered?: string;
  lives: Lives;
  onLives: (lives: Lives) => void;
  other: string;
  onOther: (other: string) => void;
  folder: string;
  // The branch a commit lands on, as it's said.
  branch: string;
  // Why it can't be promoted yet, as it's filled in ("" when it can), and
  // what went wrong.
  blocked: string;
  error?: string;
  busy: boolean;
  onGo: () => void;
  onClose: () => void;
};

// ArtifactPromote is the dialog that promotes an artifact: the version
// chosen, what it went into, and, the first time, where it's to live,
// with what that will do said before it's done. Its fields are laid out
// as Filing a page's are (file-as-fields, styles.css's, which the pages'
// and the documents' dialogs share).
export function ArtifactPromote({ title, ready, versions, version: n, onVersion, kind, onKind, text, onText, missions, home, offered, lives, onLives, other, onOther, folder, branch, blocked, error, busy, onGo, onClose }: ArtifactPromoteProps) {
  const keys = (e: KeyboardEvent) => {
    if (e.key === "Enter" && (e.metaKey || e.ctrlKey || (e.target as HTMLElement).tagName === "INPUT")) {
      e.preventDefault();
      onGo();
    }
  };
  const latest = versions[0]?.n;
  const lands = lives !== "here";

  return (
    <Dialog title={`Promote “${title}”`} onClose={onClose} wide>
      <DialogText>Promoting keeps it for good: it expires no more, and says which version was chosen and what it went into. It still takes new versions.</DialogText>
      <div className="file-as-fields" onKeyDown={keys}>
        <label>
          <span>Version</span>
          <Select fill value={String(n)} onChange={(v) => onVersion(Number(v))} aria-label="The version chosen">
            {versions.map(({ n: v, note }) => (
              <Option key={v} value={String(v)} title={note}>
                {`v${v}${v === latest ? " (the latest)" : ""}${home?.version === v ? ` (in ${home.repo})` : ""}${note ? `: ${note}` : ""}`}
              </Option>
            ))}
          </Select>
        </label>
        <label>
          <span>Into</span>
          <Select fill value={kind} onChange={(k) => onKind(k as IntoKind)} aria-label="What it went into">
            <Option value="nothing">Nothing: it's kept</Option>
            <Option value="mission">A mission</Option>
            <Option value="start">A new mission, started from it</Option>
            <Option value="document">A document, which cites it</Option>
          </Select>
        </label>
        {kind === "mission" && (
          <label>
            <span>Mission</span>
            <Select fill value={text} onChange={onText} aria-label="The mission">
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
            <Input autoFocus value={text} placeholder="What the mission is: build the glow" onChange={(e) => onText(e.target.value)} aria-label="What the mission is" />
          </label>
        )}
        {kind === "document" && (
          <label>
            <span>Document</span>
            <Input autoFocus className="mono" value={text} placeholder="repo:path/to/plan.md" onChange={(e) => onText(e.target.value)} aria-label="The document, as repo:path" />
          </label>
        )}
        {!home && ready && (
          <label>
            <span>Lives</span>
            <Select fill value={lives} onChange={(l) => onLives(l as Lives)} aria-label="Where it's to live">
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
            <Input autoFocus className="mono" value={other} placeholder="repo:folder/" onChange={(e) => onOther(e.target.value)} aria-label="The folder it's to live in, as repo:folder/" />
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
      ) : ready ? (
        <Callout tone="attention" icon="branch" title="Where it lives is chosen once">
          v{n} is put in {folder.trim() || "that folder"}, with a record of it, in one commit straight to its repo's {branch}. Promote it again, versions later, and the one chosen then goes to the same folder, with
          nothing asked. Every version stays on this machine.
        </Callout>
      ) : null}
      {blocked && <p className="dim file-as-what">{blocked}</p>}
      {error && <Banner>{error}</Banner>}
      <DialogActions>
        <Button variant="ghost" onClick={onClose} disabled={busy}>
          Cancel
        </Button>
        <Button variant="primary" onClick={onGo} disabled={busy || !n || !ready || !!blocked}>
          {busy ? (lands ? "Landing…" : "Promoting…") : lands ? `Promote, and land on ${branch}` : "Promote"} <Keys k="mod+⏎" />
        </Button>
      </DialogActions>
    </Dialog>
  );
}
