import { useState } from "react";
import { type ArtifactInfo, clearExpiredArtifacts } from "../api";
import { Button } from "../ui/Button";
import { confirm } from "../ui/Confirm";
import { Icon } from "../ui/Icon";
import { humanError } from "../ui/text";
import { toast } from "../ui/toast";
import { clearable } from "./shown";

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
