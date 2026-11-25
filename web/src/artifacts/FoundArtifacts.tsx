import { getArtifactsFound } from "../api";
import { useLoaded } from "../load";
import { Rows } from "../ui/Rows";
import { useWorkspace } from "../workspace/store";
import { ArtifactFoundTree } from "./ArtifactFoundTree";
import { ArtifactsFound } from "./ArtifactsFound";
import { FoundFolder } from "./FoundFolder";
import { FoundRow } from "./FoundRow";
import { foundTree } from "./shown";

// Artifacts that live in the workspace's repos and aren't this machine's:
// someone else promoted them, or you did on another machine. ws finds each
// by the thing.toml in its folder (as it finds a plugin's things in a
// repo), and it's named by that folder, which is its name everywhere. One
// is listed under the artifacts on this machine, and opened on a page of
// its own (FoundArtifactPage.tsx), where it runs from the repo's files, in
// the same sandboxed frame as a version here does.

// FoundArtifacts lists them, under the Artifacts page's own, as a tree:
// each repo, and the folders they're grouped in there. Nothing's drawn
// while there are none.
export function FoundArtifacts() {
  const workspace = useWorkspace();
  // Read again when the artifacts here change: one promoted here is this
  // machine's, and leaves the list.
  const key = (workspace.artifacts ?? []).map((a) => `${a.id}:${a.status}:${a.home?.where ?? ""}`).join(" ");
  // And when a look behind a read has read the spaces again (spacesSeen):
  // the list is what was found when they were last read.
  const { value } = useLoaded(() => getArtifactsFound(), [key, workspace.spacesSeen], { keep: true });
  const found = value?.found ?? [];
  if (found.length === 0) return null;
  const tree = foundTree(found);
  return (
    <ArtifactsFound>
      {/* One repo with nothing grouped in it is a plain list: there's no tree to draw. */}
      {tree.length === 1 && tree[0]?.folders.length === 0 ? (
        <Rows>
          {found.map((a) => (
            <FoundRow key={a.name} a={a} />
          ))}
        </Rows>
      ) : (
        <ArtifactFoundTree>
          {tree.map((repo) => (
            <FoundFolder key={repo.repo} folder={repo} />
          ))}
        </ArtifactFoundTree>
      )}
    </ArtifactsFound>
  );
}
