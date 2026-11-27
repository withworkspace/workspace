import type { Repo, Workspace } from "../api";
import { useWorkspace } from "../workspace/store";
import { useRefs } from "./known";
import { type Known, type PageRef, refHref } from "./refs";
import { RefArtifactCard } from "./RefArtifactCard";
import { RefCommitCard } from "./RefCommitCard";
import { RefDirCard } from "./RefDirCard";
import { RefFileCard } from "./RefFileCard";
import { RefIssueCard } from "./RefIssueCard";
import { RefMissionCard } from "./RefMissionCard";
import { RefPageCard } from "./RefPageCard";
import { RefPRCard } from "./RefPRCard";
import { RefRepoCard } from "./RefRepoCard";
import { RefTalkCard } from "./RefTalkCard";
import { ThingCard } from "./ThingCard";

// refBody is what a card says of a ref, by its kind: a file's lines, a
// pull request's title and where it stands, an issue (or the pull request
// GitHub says it is), a commit, a mission's headline, another
// page, a conversation, an artifact's version and its note, a plugin's
// thing as its plugin says it is. Each has somewhere to go. pane: it's
// shown in Find's pane.
export function refBody(r: PageRef, workspace: Workspace, known?: Known, repos?: Map<string, Repo>, pane?: boolean) {
  switch (r.kind) {
    case "file":
      return <RefFileCard r={r} known={known} pane={pane} />;
    case "dir":
      return <RefDirCard r={r} known={known} pane={pane} />;
    case "repo":
      return <RefRepoCard r={r} label={repos?.get(r.repo)?.label} />;
    case "pr":
      return <RefPRCard r={r} />;
    case "issue":
      return <RefIssueCard r={r} href={refHref(r, known)} />;
    case "commit":
      return <RefCommitCard r={r} href={refHref(r, known)} />;
    case "mission":
      return <RefMissionCard r={r} mission={workspace.missions.find((x) => x.slug === r.slug)} />;
    case "page":
      return <RefPageCard r={r} page={workspace.pages?.find((x) => x.id === r.id)} />;
    case "talk":
      return <RefTalkCard r={r} talk={workspace.conversations.find((x) => x.id === r.id)} />;
    case "artifact":
      return <RefArtifactCard r={r} />;
    case "thing":
      return <ThingCard r={r} known={known} />;
  }
}

// RefCardBody is what a ref's card says, for wherever a card is shown: on
// hover in a page, and beside the list in Find (pane).
export function RefCardBody({ r, known, pane }: { r: PageRef; known?: Known; pane?: boolean }) {
  const workspace = useWorkspace();
  // What there is to name says what a plugin's thing is called, and
  // where it opens: given, where the card's shown outside the pages (Find).
  const refs = useRefs();
  return <>{refBody(r, workspace, known ?? refs?.known, refs?.repos, pane)}</>;
}
