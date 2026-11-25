import { useState } from "react";
import { act, writePR, type CapsuleDetail, type MissionDetail } from "../../api";
import type { Reload } from "../../load";
import { Button } from "../../ui/Button";
import { Icon } from "../../ui/Icon";
import { toast } from "../../ui/toast";
import { MissionOpenPR } from "./MissionOpenPR";

// A pull request is opened through ws, so the process's rules about it are
// held where ws is the one acting: it pushes the capsule's branch and opens
// it, or refuses, saying which rule isn't met (your review first, say). In
// a mission with a plan, it's the step that makes one that opens it; you
// may open one whenever you like. Its title and description are ws's PR
// writer's to write, from the step it's for and PR.md: "Write it" has it
// write them here, for you to change before you open it, and opening it
// with no title leaves them to it, and ws opens it once they're written.

// OpenPR is the button that opens a pull request from a capsule, and what
// it asks first (MissionOpenPR).
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
        <MissionOpenPR
          name={name}
          step={step?.n}
          title={title}
          onTitle={setTitle}
          body={body}
          onBody={setBody}
          draft={draft}
          onDraft={setDraft}
          writing={writing}
          onWrite={write}
          busy={busy}
          error={error}
          onClose={close}
          onOpen={submit}
        />
      )}
    </>
  );
}
