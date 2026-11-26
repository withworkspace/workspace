import type { FileTree, TreeFile } from "../../api";
import { Button } from "../../ui/Button";
import { Empty } from "../../ui/Empty";
import { Icon } from "../../ui/Icon";
import { Keys } from "../../ui/Keys";
import { Loading } from "../../ui/Loading";
import { FilesChanged } from "./FilesChanged";
import { FilesOverview } from "./FilesOverview";
import { branchName } from "./words";

// FileOverview is the worktree before a file's open: what's changed, a
// click each, and how to find the rest.
export function FileOverview({ tree, changed, onOpen, onFind }: { tree?: FileTree; changed: TreeFile[]; onOpen: (p: string) => void; onFind: () => void }) {
  if (!tree) return <Loading what="Loading files…" />;
  return (
    <FilesOverview>
      <Empty
        icon="file"
        title="Pick a file"
        action={
          <Button onClick={onFind} title="Find a file" keys="t">
            <Icon name="search" /> Find a file <Keys k="t" />
          </Button>
        }
      >
        {tree.files.length} files{tree.base ? `; ${changed.length} changed since ${branchName(tree.baseRef)}` : ""}.
      </Empty>
      {changed.length > 0 && <FilesChanged files={changed.slice(0, 40)} count={changed.length} onOpen={onOpen} />}
    </FilesOverview>
  );
}
