import type { KeyboardEvent } from "react";
import { Banner } from "../ui/Banner";
import { Button } from "../ui/Button";
import { Callout } from "../ui/Callout";
import { Dialog, DialogActions, DialogText } from "../ui/Dialog";
import { Input } from "../ui/Input";
import { Keys } from "../ui/Keys";
import { Option, Select } from "../ui/Select";
import type { IntoKind, Lives } from "./shown";
import { Text, textClass } from "../ui/Text";
import css from "./ArtifactPromote.module.css";
import { cx } from "../ui/cx";

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
      <DialogText>Keeps the artifact permanently, recording the chosen version and what it went into. New versions can still be added.</DialogText>
      <div className="file-as-fields" onKeyDown={keys}>
        <label>
          <span>Version</span>
          <Select fill value={String(n)} onChange={(v) => onVersion(Number(v))} aria-label="Version">
            {versions.map(({ n: v, note }) => (
              <Option key={v} value={String(v)} title={note}>
                {`v${v}${v === latest ? " (latest)" : ""}${home?.version === v ? ` (in ${home.repo})` : ""}${note ? `: ${note}` : ""}`}
              </Option>
            ))}
          </Select>
        </label>
        <label>
          <span>Into</span>
          <Select fill value={kind} onChange={(k) => onKind(k as IntoKind)} aria-label="Into">
            <Option value="nothing">Nothing</Option>
            <Option value="mission">A mission</Option>
            <Option value="start">A new mission</Option>
            <Option value="document">A document</Option>
          </Select>
        </label>
        {kind === "mission" && (
          <label>
            <span>Mission</span>
            <Select fill value={text} onChange={onText} aria-label="Mission">
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
            <Input autoFocus value={text} placeholder="What the mission is: build the glow" onChange={(e) => onText(e.target.value)} aria-label="New mission" />
          </label>
        )}
        {kind === "document" && (
          <label>
            <span>Document</span>
            <Input autoFocus className={cx(textClass("mono"), css.code)} value={text} placeholder="repo:path/to/plan.md" onChange={(e) => onText(e.target.value)} aria-label="Document (repo:path)" />
          </label>
        )}
        {!home && ready && (
          <label>
            <span>Location</span>
            <Select fill value={lives} onChange={(l) => onLives(l as Lives)} aria-label="Location">
              {offered ? (
                <Option value="offered" note="shared">
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
            <Input autoFocus className={cx(textClass("mono"), css.code)} value={other} placeholder="repo:folder/" onChange={(e) => onOther(e.target.value)} aria-label="Folder (repo:folder/)" />
          </label>
        )}
      </div>
      {kind === "mission" || kind === "start" ? (
        <Text as="p" dim className="file-as-what">Added to the mission's artifacts, for its brief and crew.</Text>
      ) : null}
      {home ? (
        <Callout tone="attention" icon="branch" title={`Lives in ${home.where}`}>
          {home.version === n ? `v${n} already there: record updated` : `v${n} replaces v${home.version}`}, in
          one commit straight to {home.repo}'s {branch}. All versions stay on this machine.
        </Callout>
      ) : lives === "here" ? (
        <Callout tone="info" icon="monitor" title="Stays on this machine">
          Not shared. Promote again to put it in a repo.
        </Callout>
      ) : ready ? (
        <Callout tone="attention" icon="branch" title="Location is chosen once">
          v{n} goes in {folder.trim() || "that folder"}, in one commit straight to the repo's {branch}. Later
          promotions use the same folder. All versions stay on this machine.
        </Callout>
      ) : null}
      {blocked && <Text as="p" dim className="file-as-what">{blocked}</Text>}
      {error && <Banner>{error}</Banner>}
      <DialogActions>
        <Button variant="ghost" onClick={onClose} disabled={busy} keys="esc">
          Cancel
        </Button>
        <Button variant="primary" onClick={onGo} disabled={busy || !n || !ready || !!blocked} keys="mod+⏎">
          {busy ? (lands ? "Landing…" : "Promoting…") : lands ? `Promote, and land on ${branch}` : "Promote"} <Keys k="mod+⏎" />
        </Button>
      </DialogActions>
    </Dialog>
  );
}
