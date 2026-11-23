import { useState } from "react";
import { act, writePR, type CapsuleDetail, type MissionDetail } from "../api";
import type { Reload } from "../load";
import { Icon } from "../ui/Icon";
import { Spinner } from "../ui/Spinner";
import { toast } from "../ui/toast";
import { humanError } from "../ui/text";
import { Dialog, DialogActions } from "../ui/Dialog";
import { Button } from "../ui/Button";
import { Checkbox } from "../ui/Checkbox";
import { Input } from "../ui/Input";
import { TextArea } from "../ui/TextArea";

// A pull request is opened through ws, so the process's rules about it are
// held where ws is the one acting: it pushes the capsule's branch and opens
// it, or refuses, saying which rule isn't met (your review first, say). In
// a mission with a plan, it's the step that makes one that opens it; you
// may open one whenever you like. Its title and description are ws's PR
// writer's to write, from the step it's for and PR.md: "Write it" has it
// write them here, for you to change before you open it, and opening it
// with no title leaves them to it, and ws opens it once they're written.

// canOpenPR is whether a capsule could have a pull request opened from it:
// a repo that takes them, commits to open, and none open already (once
// GitHub's been asked).
export function canOpenPR(d: MissionDetail, c: CapsuleDetail) {
  return d.github && c.exists && !c.direct && !c.pr && (c.ahead ?? 0) > 0;
}

export function OpenPR({ d, c, reload }: { d: MissionDetail; c: CapsuleDetail; reload: Reload }) {
  const [open, setOpen] = useState(false);
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [draft, setDraft] = useState(false);
  const [busy, setBusy] = useState(false);
  const [writing, setWriting] = useState(false);
  const [error, setError] = useState("");
  const name = `${c.repo}/${c.name}`;
  const step = d.stages?.find((s) => s.opensPR && s.state !== "accepted");
  const close = () => {
    setOpen(false);
    setError("");
  };
  const write = () => {
    setWriting(true);
    setError("");
    writePR(d.slug, name)
      .then((w) => {
        setTitle(w.title);
        setBody(w.body);
      })
      .catch((e: Error) => setError(e.message))
      .finally(() => setWriting(false));
  };
  const submit = () => {
    setBusy(true);
    act(d.slug, "open-pr", { capsule: name, title, text: body, draft })
      .then((r) => {
        close();
        toast(r.message, { tone: "done" });
        return reload();
      })
      .catch((e: Error) => setError(e.message))
      .finally(() => setBusy(false));
  };
  return (
    <>
      <Button variant="ghost" size="small" onClick={() => setOpen(true)} title="Push its branch and open a pull request, through ws">
        <Icon name="pr" size={12} /> Open pull request
      </Button>
      {open && (
        <Dialog title={`Open a pull request from ${name}`} onClose={close} className="end-mission" wide>
          <p>
            ws pushes its branch and opens the pull request onto the default branch, holding the process's rules about pull requests: one that wants your
            review first refuses it until you've approved the capsule as it is.
            {step && ` Step ${step.n} of the plan opens it.`} Its title and description are the PR writer's to write, from the step it's for and the workspace's PR.md, unless you give them.
          </p>
          <div className="open-pr-form">
            <div className="open-pr-title">
              <Input
                value={title}
                autoFocus
                aria-label="Title"
                placeholder="Its title, or leave it to the PR writer"
                disabled={writing}
                onChange={(e) => setTitle(e.target.value)}
              />
              <Button variant="ghost" size="small" onClick={write} disabled={writing || busy} title="Have the PR writer write its title and description, from the step it's for and PR.md">
                {writing ? <Spinner size={12} /> : <Icon name="pencil" size={12} />} {writing ? "Writing…" : "Write it"}
              </Button>
            </div>
            <TextArea
              value={body}
              rows={8}
              aria-label="Description"
              placeholder="What it does, and why"
              disabled={writing}
              onChange={(e) => setBody(e.target.value)}
            />
          </div>
          <Checkbox className="setting-toggle open-pr-draft" checked={draft} onChange={setDraft}>
            Open it as a draft
          </Checkbox>
          {error && <div className="banner">{humanError(error)}</div>}
          <DialogActions>
            <Button variant="ghost" onClick={close}>
              Cancel
            </Button>
            <Button
              variant="primary"
              onClick={submit}
              disabled={busy || writing}
              title={title.trim() ? undefined : "The PR writer writes its title and description, and ws opens it once they're written"}
            >
              {title.trim() ? "Open it" : "Write and open it"}
            </Button>
          </DialogActions>
        </Dialog>
      )}
    </>
  );
}
