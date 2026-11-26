import { useState } from "react";
import { act, type CapsuleDetail } from "../../api";
import type { Reload } from "../../load";
import { Button } from "../../ui/Button";
import { Icon } from "../../ui/Icon";
import { toast } from "../../ui/toast";
import { MissionEndingDialog } from "./MissionEndingDialog";

// CapsuleEnds are what ends a capsule alone: handed on to a mission of its
// own (raised for it, or one there is), which takes it, or dropped. Each
// asks first, in a MissionEndingDialog.
export function CapsuleEnds({ slug, c, reload }: { slug: string; c: CapsuleDetail; reload: Reload }) {
  const [open, setOpen] = useState<"hand-on" | "drop">();
  const [text, setText] = useState("");
  const [to, setTo] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const name = `${c.repo}/${c.name}`;
  const close = () => {
    setOpen(undefined);
    setError("");
  };
  const submit = () => {
    if (!open) return;
    setBusy(true);
    act(slug, open, { capsule: name, text, to: to.trim() ? [to.trim()] : undefined })
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
      <Button variant="ghost" size="small" onClick={() => setOpen("hand-on")} title="Move to another mission">
        <Icon name="arrow-right" size={12} /> Hand on
      </Button>
      <Button variant="ghost" size="small" onClick={() => setOpen("drop")} title="Drop: work won't be part of the mission">
        <Icon name="x" size={12} /> Drop
      </Button>
      {open && <MissionEndingDialog ending={open} name={name} text={text} onText={setText} to={to} onTo={setTo} error={error} busy={busy} onClose={close} onEnd={submit} />}
    </>
  );
}
