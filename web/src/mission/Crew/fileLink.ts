import type { Crew, Workspace } from "../../api";

// fileLinkOf makes an edited file's path a link into the capsule's file
// explorer, when it's in the crew's capsule: a path inside the capsule's
// folder, or one relative to it. None when crew have no capsule of the
// mission's.
export function fileLinkOf(workspace: Workspace, slug: string, crew?: Pick<Crew, "capsule">): ((path: string) => string | undefined) | undefined {
  const m = workspace.missions.find((x) => x.slug === slug);
  const cap = m?.capsules.find((c) => `${c.repo}/${c.name}` === crew?.capsule);
  if (!cap) return undefined;
  return (path: string) => {
    const prefix = cap.path.replace(/\/$/, "") + "/";
    const rel = path.startsWith(prefix) ? path.slice(prefix.length) : !path.startsWith("/") ? path : undefined;
    return rel ? `/missions/${slug}/files/${cap.repo}/${cap.name}?${new URLSearchParams({ path: rel })}` : undefined;
  };
}
