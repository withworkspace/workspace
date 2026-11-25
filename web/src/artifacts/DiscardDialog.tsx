import { useState } from "react";
import { type ArtifactDone, type ArtifactInfo, discardArtifact, restoreArtifact } from "../api";
import { humanError } from "../ui/text";
import { toast } from "../ui/toast";
import { useWorkspace } from "../workspace/store";
import { ArtifactDiscard } from "./ArtifactDiscard";
import { doneWords } from "./doneWords";

// DiscardDialog discards an artifact, asking why (which can be left
// empty).
export function DiscardDialog({ a, onClose, onDone }: { a: ArtifactInfo; onClose: () => void; onDone?: (d: ArtifactDone) => void }) {
  const defaultBranch = useWorkspace((w) => w.defaultBranch);
  const [why, setWhy] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
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
  return <ArtifactDiscard title={a.title} id={a.id} home={a.home} branch={defaultBranch} why={why} onWhy={setWhy} error={error} busy={busy} onGo={go} onClose={onClose} />;
}

async function restoreFromToast(id: string) {
  try {
    toast(doneWords("Restored", await restoreArtifact(id)), { tone: "done" });
  } catch (e) {
    toast(humanError((e as Error).message), { tone: "danger" });
  }
}
