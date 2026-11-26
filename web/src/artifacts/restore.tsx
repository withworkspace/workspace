import { type ArtifactDone, type ArtifactInfo, restoreArtifact } from "../api";
import { confirm } from "../ui/Confirm";
import { humanError } from "../ui/strings";
import { toast } from "../ui/toast";
import { doneWords } from "./doneWords";

// restore puts a discarded artifact back, once you've said so. (Where it
// lived in a repo, it's put back there too: the server says so in what
// it did, as a discarded one's home isn't sent.)
export async function restore(a: ArtifactInfo): Promise<ArtifactDone | undefined> {
  const yes = await confirm({
    title: `Restore “${a.title}”?`,
    body: <>Folder comes back from .ws/artifacts/.discarded. If it lived in a repo, that version is put back in one commit. Expiry restarts from now.</>,
    confirm: "Restore",
  });
  if (!yes) return undefined;
  try {
    const done = await restoreArtifact(a.id);
    toast(doneWords("Restored", done), { tone: "done" });
    return done;
  } catch (e) {
    toast(humanError((e as Error).message), { tone: "danger", ms: 8000 });
    return undefined;
  }
}
