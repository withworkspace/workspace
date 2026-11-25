import { useMemo } from "react";
import type { Crew, Workspace } from "../../api";
import { fileLinkOf } from "./fileLink";

// useFileLink is where a file crew edited can be read (fileLinkOf), kept
// while the workspace, the mission and crew's capsule are.
export function useFileLink(workspace: Workspace, slug: string, crew?: Crew) {
  const capsule = crew?.capsule;
  return useMemo(() => fileLinkOf(workspace, slug, { capsule }), [workspace, slug, capsule]);
}
