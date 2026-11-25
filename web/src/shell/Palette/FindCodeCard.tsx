import { HoverCardHead, HoverCardLine, HoverCardTitle } from "../../ui/HoverCard";
import { Icon } from "../../ui/Icon";
import { textClass } from "../../ui/Text";

// FindCodeCard says, in Find's pane, where a line a file says is: its
// path, its line, and the repo and capsule it's in (worktree: ".ground"
// is the repo's default branch).
export function FindCodeCard({ path, line, repo, worktree }: { path: string; line: number; repo: string; worktree: string }) {
  return (
    <>
      <HoverCardHead>
        <Icon name="file" size={13} className={textClass("dim")} />
        <HoverCardTitle className={textClass("mono")}>{path}</HoverCardTitle>
      </HoverCardHead>
      <HoverCardLine dim>
        Line {line} in {repo}
        {worktree === ".ground" ? ", its default branch" : `, capsule ${worktree}`}
      </HoverCardLine>
    </>
  );
}
