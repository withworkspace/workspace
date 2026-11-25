import { useNavigate, useParams } from "react-router";
import { capsuleAPI, docRef } from "../api";
import { Crumbs } from "../ui/Crumbs";
import { useWorkspace } from "../workspace/store";
import { FilesCapsule } from "./Browser/FilesCapsule";
import { filesBarCrumbsClass } from "./Browser/FilesBar";
import { TabbedBrowser } from "./WorktreeTabs";

// A mission's capsule's files: the explorer (Browser/FileBrowser.tsx),
// with the way back to the mission and which capsule it is on its bar.

export const explorerKeys: [string, string][] = [
  ["t / mod+P", "Find a file"],
  ["mod+shift+F", "Search what the capsule's files say"],
  ["j / k", "Next / previous in the tree"],
  ["⏎ / o", "Open a file, or open or close a folder"],
  ["⏎", "On the markdown file that's open: open it as a document, to comment on"],
  ["h / l", "Close / open a folder"],
  ["c", "Only what's changed, or every file"],
  ["[ / ]", "Previous / next changed file"],
  ["⌘-click", "Go to a name's definition, or who uses it"],
  ["d", "Go to a definition on the picked line"],
  ["b", "Back from a definition"],
  ["m", "Markdown: rendered, or as text"],
  ["v", "Review the capsule's changes"],
  ["y", "Copy the file's path"],
  ["p", "Panes: the tree and the file, or the file alone"],
  ["r", "Reload"],
  ["esc", "Back to the mission"],
];

// Explorer is a mission's capsule's files.
export function Explorer() {
  const { slug = "", repo = "", name = "" } = useParams();
  const navigate = useNavigate();
  const mission = useWorkspace((w) => w.missions.find((m) => m.slug === slug));
  const readsSeen = useWorkspace((w) => w.readsSeen);
  const api = capsuleAPI(slug, repo, name);
  return (
    <TabbedBrowser
      at={{ repo, worktree: name }}
      key={api}
      api={api}
      label={`${repo}/${name}`}
      updated={`${mission?.updated ?? ""}:${readsSeen ?? ""}`}
      reviewTo={`/missions/${slug}/review/${repo}/${name}`}
      docOf={(path) => docRef(repo, name, path)}
      search={{ repo, worktree: name }}
      onEscape={() => navigate(`/missions/${slug}?tab=work`)}
      empty="The capsule is empty."
      head={
        <>
          <Crumbs className={filesBarCrumbsClass} steps={[{ to: `/missions/${slug}?tab=work`, label: mission?.title ?? slug }]} />
          <FilesCapsule repo={repo} name={name} />
        </>
      }
    />
  );
}
