import { useState } from "react";
import { type ArtifactDetail, promoteArtifact } from "../api";
import { humanError } from "../ui/text";
import { toast } from "../ui/toast";
import { ArtifactHome } from "./ArtifactHome";

// HomeLine says where a promoted artifact lives in a repo, and which
// version is there. Where the version on the page is another, it's one
// press to put it there: where it lives was chosen the first time, so
// nothing's asked.
export function HomeLine({ d, n, onDone }: { d: ArtifactDetail; n: number; onDone: () => void }) {
  const [busy, setBusy] = useState(false);
  const home = d.home;
  if (!home) return null;
  const put = async () => {
    if (busy) return;
    setBusy(true);
    try {
      const done = await promoteArtifact(d.id, { version: n, into: d.promoted?.into || undefined });
      toast(`Promoted ${d.id}: v${n} is in ${home.where}${done.landed ? ` (${done.landed})` : ""}`, { tone: "done", ms: 6000 });
      onDone();
    } catch (e) {
      toast(humanError((e as Error).message), { tone: "danger", ms: 8000 });
    } finally {
      setBusy(false);
    }
  };
  return <ArtifactHome where={home.where} repo={home.repo} version={home.version} commit={home.commit} put={n && n !== home.version && d.all.some((x) => x.n === n) ? { n, busy, onPress: put } : undefined} />;
}
