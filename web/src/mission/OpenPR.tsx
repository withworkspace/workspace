import { useState } from "react";
import { act, type CapsuleDetail, type MissionDetail } from "../api";
import { Icon } from "../ui/Icon";
import { Dialog, humanError, toast } from "../ui/kit";

// A pull request is opened through ws, so the process's rules about it are
// held where ws is the one acting: it pushes the capsule's branch and opens
// it, or refuses, saying which rule isn't met (your review first, say). In
// a mission with a plan, it's the step that makes one that opens it; you
// may open one whenever you like.

// canOpenPR is whether a capsule could have a pull request opened from it:
// a repo that takes them, commits to open, and none open already (once
// GitHub's been asked).
export function canOpenPR(d: MissionDetail, c: CapsuleDetail) {
  return d.github && c.exists && !c.direct && !c.pr && (c.ahead ?? 0) > 0;
}

export function OpenPR({ d, c, reload }: { d: MissionDetail; c: CapsuleDetail; reload: () => void }) {
  const [open, setOpen] = useState(false);
  const [title, setTitle] = useState(d.title);
  const [body, setBody] = useState("");
  const [draft, setDraft] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const name = `${c.repo}/${c.name}`;
  const step = d.stages?.find((s) => s.opensPR && s.state !== "accepted");
  const close = () => {
    setOpen(false);
    setError("");
  };
  const submit = () => {
    setBusy(true);
    act(d.slug, "open-pr", { capsule: name, title, text: body, draft })
      .then((r) => {
        close();
        toast(r.message, { tone: "done" });
        reload();
      })
      .catch((e: Error) => setError(e.message))
      .finally(() => setBusy(false));
  };
  return (
    <>
      <button className="ghost small" onClick={() => setOpen(true)} title="Push its branch and open a pull request, through ws">
        <Icon name="pr" size={12} /> Open pull request
      </button>
      {open && (
        <Dialog title={`Open a pull request from ${name}`} onClose={close} className="end-mission" wide>
          <p>
            ws pushes its branch and opens the pull request onto the default branch, holding the process's rules about pull requests: one that wants your
            review first refuses it until you've approved the capsule as it is.
            {step && ` Step ${step.n} of the plan opens it; its crew would, through ws.`}
          </p>
          <div className="open-pr-form">
            <input value={title} autoFocus aria-label="Title" placeholder="Its title" onChange={(e) => setTitle(e.target.value)} />
            <textarea value={body} rows={6} aria-label="Description" placeholder="What it does, and why" onChange={(e) => setBody(e.target.value)} />
          </div>
          <label className="setting-toggle open-pr-draft">
            <input type="checkbox" checked={draft} onChange={(e) => setDraft(e.target.checked)} />
            Open it as a draft
          </label>
          {error && <div className="banner">{humanError(error)}</div>}
          <div className="dialog-actions">
            <button className="ghost" onClick={close}>
              Cancel
            </button>
            <button className="primary" onClick={submit} disabled={busy || !title.trim()}>
              Open it
            </button>
          </div>
        </Dialog>
      )}
    </>
  );
}
