import { filesKeys } from "./keys";
import { useNavigate, useParams } from "react-router";
import { capsuleAPI, docRef } from "../api";
import { Crumbs } from "../ui/Crumbs";
import { useWorkspace } from "../workspace/store";
import { FilesCapsule } from "./Browser/FilesCapsule";
import { filesBarCrumbsClass } from "./Browser/FilesBar";
import { TabbedBrowser } from "./WorktreeTabs";

// A mission's capsule's files: the explorer (Browser/FileBrowser.tsx),
// with the way back to the mission and which capsule it is on its bar.

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
      escape={filesKeys.backToMission}
      empty="The capsule is empty."
      head={
        <>
          <Crumbs className={filesBarCrumbsClass} steps={[{ to: `/missions/${slug}?tab=work`, label: mission?.title ?? slug, keys: filesKeys.backToMission }]} />
          <FilesCapsule repo={repo} name={name} />
        </>
      }
    />
  );
}
