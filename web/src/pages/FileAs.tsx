import { useEffect, useState } from "react";
import { type DocKind, type DocSaved, FileChanged, filePage, type PageFiling, type PageTied, savePageToFile, type SpaceRef } from "../api";
import { Icon } from "../ui/Icon";
import { Callout, humanError, Keys } from "../ui/kit";
import { Dialog, DialogActions } from "../ui/Dialog";
import { Option, Select } from "../ui/Select";
import { Button } from "../ui/Button";

// FileAs files a page as a document of one of the workspace's kinds: a new file where the kind
// says, named from its title, in a commit of yours on its repo's default branch: landed, in a repo
// with no pull requests, or left in a capsule lifted for it. Or of no kind, in a folder of a repo
// (a space of documents', most likely), named from its title. What's filed is the page as it reads:
// what its cells hold is kept as text, its questions are left out. The page stays, and says where
// it was filed.
export function FileAs({
  id,
  kinds,
  kind,
  to,
  spaces,
  title,
  beforeFile,
  onFiled,
  onClose,
}: {
  id: string;
  kinds: DocKind[];
  kind?: string; // the kind the page was started as
  to?: string; // where it was started to be filed, of no kind: repo:folder/
  spaces?: SpaceRef[]; // where a document of no kind may go: one of their folders
  title: string;
  // beforeFile saves the page as it is now: that's what's filed.
  beforeFile: () => Promise<unknown>;
  onFiled: (done: PageFiling) => void;
  onClose: () => void;
}) {
  // A kind's name, or "" for none: a folder (where).
  const [k, setK] = useState(kind && kinds.some((x) => x.name === kind) ? kind : to ? "" : (kinds[0]?.name ?? ""));
  const [where, setWhere] = useState(to ?? (spaces?.[0] ? `${spaces[0].repo}:${spaces[0].folder ?? ""}` : ""));
  const [name, setName] = useState(title === "Untitled" ? "" : title);
  // The commit's first line: the kind's, until you change it.
  const [message, setMessage] = useState<string>();
  const [plan, setPlan] = useState<PageFiling>();
  const [error, setError] = useState<string>();
  const [busy, setBusy] = useState(false);

  // What it would do, as the kind and title stand.
  // (An answer that comes once they've changed again is dropped.)
  useEffect(() => {
    let stale = false;
    const t = setTimeout(() => {
      filePage(id, { kind: k, ...(k ? {} : { to: where }), title: name, dry: true }).then(
        (p) => {
          if (stale) return;
          setPlan(p);
          setError(undefined);
        },
        (e: Error) => {
          if (stale) return;
          setPlan(undefined);
          setError(humanError(e.message));
        },
      );
    }, 250);
    return () => {
      stale = true;
      clearTimeout(t);
    };
  }, [id, k, name, where]);

  const subject = message ?? plan?.subject ?? "";
  const ready = !!plan && !plan.taken && !busy && subject.trim() !== "";
  const file = async () => {
    if (!ready) return;
    setBusy(true);
    setError(undefined);
    try {
      await beforeFile();
      // Only a message you've changed is sent: the kind's is made again,
      // with the number it takes when it's filed.
      onFiled(await filePage(id, { kind: k, ...(k ? {} : { to: where }), title: name, message }));
    } catch (e) {
      setError(humanError((e as Error).message));
    } finally {
      setBusy(false);
    }
  };
  const keys = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && (e.metaKey || e.ctrlKey || (e.target as HTMLElement).tagName === "INPUT")) {
      e.preventDefault();
      file();
    }
  };
  const lands = plan?.how === "land";

  return (
    <Dialog title="File as a document" onClose={onClose} className="file-as" wide>
      <div className="file-as-fields" onKeyDown={keys}>
        <label>
          <span>Kind</span>
          <Select fill value={k} onChange={setK} aria-label="Kind of document">
            {kinds.map((x) => (
              <Option key={x.name} value={x.name}>
                {x.title}
              </Option>
            ))}
            <Option value="">No kind: in a folder…</Option>
          </Select>
        </label>
        {!k && (
          <label>
            <span>Folder</span>
            <input className="mono" list="file-as-places" value={where} placeholder="repo:folder/" onChange={(e) => setWhere(e.target.value)} aria-label="The folder it's filed in, as repo:folder/" />
            <datalist id="file-as-places">
              {(spaces ?? []).map((s) => (
                <option key={s.repo} value={`${s.repo}:${s.folder ?? ""}`}>
                  {s.label}
                </option>
              ))}
            </datalist>
          </label>
        )}
        <label>
          <span>Title</span>
          <input autoFocus value={name} placeholder="What it's named from" onChange={(e) => setName(e.target.value)} aria-label="Title" />
        </label>
        <label>
          <span>Commit</span>
          <input className="mono" value={subject} onChange={(e) => setMessage(e.target.value)} aria-label="The commit's message" />
        </label>
      </div>
      {plan && (
        <p className={`file-as-where${plan.taken ? " taken" : ""}`}>
          <Icon name="file" size={12} /> <span className="mono">{plan.ref.replace(":", " · ")}</span>
          {plan.taken && <span>: there's one there already. Give it another title.</span>}
        </p>
      )}
      {plan && (
        <Callout tone={lands ? "attention" : "info"} icon={lands ? "alert" : "info"} title={lands ? `Filing lands it on ${plan.branch}` : "Filing lifts a capsule for it"}>
          {lands ? (
            <>
              This repo has no pull requests: it's one commit, straight to {plan.branch}
              {plan.verify ? (
                <>
                  , once <span className="mono">{plan.verify}</span> passes
                </>
              ) : null}
              .
            </>
          ) : (
            <>It's committed in a new capsule, and left there: yours, to add to or open a pull request from.</>
          )}
        </Callout>
      )}
      <p className="dim file-as-what">What's filed is the page as it reads: what its cells hold is kept as text, its questions are left out. The page stays, and says where it was filed.</p>
      {error && <div className="banner">{error}</div>}
      <DialogActions>
        <Button variant="ghost" onClick={onClose} disabled={busy}>
          Cancel
        </Button>
        <Button variant="primary" onClick={file} disabled={!ready}>
          {busy ? (lands ? "Landing…" : "Filing…") : lands ? `File, and land on ${plan?.branch}` : "File it"} <Keys k="mod+⏎" />
        </Button>
      </DialogActions>
    </Dialog>
  );
}

// SaveToFile writes a page tied to a file back to it, as the page reads:
// what its cells hold kept as text, its questions left out. It's saved as
// a document is: committed in its capsule, or on a default branch landed
// (a repo with no pull requests) or committed in a capsule lifted for it.
export function SaveToFile({
  id,
  tied,
  beforeSave,
  onSaved,
  onChanged,
  onClose,
}: {
  id: string;
  tied: PageTied;
  // beforeSave saves the page as it is now: that's what's written.
  beforeSave: () => Promise<unknown>;
  onSaved: (saved: DocSaved) => void;
  // onChanged: the file's moved on since the page took it in.
  onChanged: (why: string) => void;
  onClose: () => void;
}) {
  const saves = tied.saves;
  const [message, setMessage] = useState(saves?.subject ?? "");
  const [error, setError] = useState<string>();
  const [busy, setBusy] = useState(false);
  const lands = saves?.how === "land";
  const save = async () => {
    if (busy) return;
    setBusy(true);
    setError(undefined);
    try {
      await beforeSave();
      onSaved(await savePageToFile(id, message.trim() === saves?.subject ? undefined : message));
    } catch (e) {
      if (e instanceof FileChanged) onChanged(e.message);
      else setError(humanError((e as Error).message));
    } finally {
      setBusy(false);
    }
  };
  return (
    <Dialog title="Save to the repo" onClose={onClose} className="file-as" wide>
      <p className="file-as-where">
        <Icon name="file" size={12} /> <span className="mono">{tied.ref.replace(":", " · ")}</span>
      </p>
      <div
        className="file-as-fields"
        onKeyDown={(e) => {
          if (e.key === "Enter" && (e.metaKey || e.ctrlKey || (e.target as HTMLElement).tagName === "INPUT")) {
            e.preventDefault();
            save();
          }
        }}
      >
        <label>
          <span>Commit</span>
          <input autoFocus className="mono" value={message} onChange={(e) => setMessage(e.target.value)} aria-label="The commit's message" />
        </label>
      </div>
      {saves && saves.how !== "capsule" && (
        <Callout tone={lands ? "attention" : "info"} icon={lands ? "alert" : "info"} title={lands ? `Saving lands it on ${saves.branch}` : "Saving lifts a capsule for it"}>
          {lands ? (
            <>
              This repo has no pull requests: it's one commit, straight to {saves.branch}
              {saves.verify ? (
                <>
                  , once <span className="mono">{saves.verify}</span> passes
                </>
              ) : null}
              .
            </>
          ) : (
            <>It's on {saves.branch}, which isn't changed in place: it's committed in a new capsule, and left there, for you to open a pull request from. The page goes on with the file there.</>
          )}
        </Callout>
      )}
      <p className="dim file-as-what">What's saved is the page as it reads: what its cells hold is kept as text, its questions are left out. The page keeps them.</p>
      {error && <div className="banner">{error}</div>}
      <DialogActions>
        <Button variant="ghost" onClick={onClose} disabled={busy}>
          Cancel
        </Button>
        <Button variant="primary" onClick={save} disabled={busy || !message.trim()}>
          {busy ? (lands ? "Landing…" : "Saving…") : lands ? `Save, and land on ${saves?.branch}` : "Save"} <Keys k="mod+⏎" />
        </Button>
      </DialogActions>
    </Dialog>
  );
}
