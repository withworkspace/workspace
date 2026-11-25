import { setPref, usePrefs } from "../prefs";
import { ArtifactFoundFolder } from "./ArtifactFoundTree";
import { FoundRow } from "./FoundRow";
import type { FoundFolder as Folder } from "./shown";

// FoundFolder is a repo, or a folder of its artifacts, in the list's tree:
// its name and how many are in it, and, unfolded, the artifacts straight
// in it and its folders.
export function FoundFolder({ folder }: { folder: Folder }) {
  const id = `${folder.repo}:${folder.path}`;
  // Which folders are folded is kept for the next time the page's opened
  // (prefs.ts): by repo and path.
  const open = usePrefs((s) => !s.foundFolded.includes(id));
  const toggled = (now: boolean) => {
    if (now !== open) setPref("foundFolded", (was) => (now ? was.filter((x) => x !== id) : [...was, id]));
  };
  return (
    <ArtifactFoundFolder
      name={folder.name}
      count={folder.count}
      title={folder.path ? `${folder.repo}: ${folder.path}/` : folder.repo}
      open={open}
      onToggle={toggled}
      rows={folder.found.length > 0 && folder.found.map((a) => <FoundRow key={a.name} a={a} />)}
    >
      {folder.folders.map((f) => (
        <FoundFolder key={f.path} folder={f} />
      ))}
    </ArtifactFoundFolder>
  );
}
