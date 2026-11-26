import type { Mission, RunDetail } from "../api";
import { split } from "./runRows";

// What Below's Runs view knows of runs: the services that are running,
// those of the place the page is of (here), which of them are new, and
// where each one is gone to.

// BelowPlace is what the page is of, where runs are concerned: a mission
// (its capsules), or a repo's ground or one capsule on the Files page.
export type BelowPlace = { mission: string } | { repo: string; worktree: string };

// placeAt is the place of the page at pathname: any page of a mission,
// or a repo's page on Files (its ground, or one of its capsules). Other
// pages are of no place that runs.
export function placeAt(pathname: string): BelowPlace | undefined {
  const [a, b, c] = pathname.split("/").filter(Boolean).map(decodeURIComponent);
  if (a === "missions" && b) return { mission: b };
  if (a === "files" && b) return { repo: b, worktree: c ?? "ground" };
  return undefined;
}

// placeSaid is a place as its view's head says it.
export function placeSaid(p: BelowPlace, missions: Mission[]): string {
  if ("mission" in p) return missions.find((m) => m.slug === p.mission)?.title || p.mission;
  return `${p.repo} / ${p.worktree}`;
}

// inPlace is whether a run is the place's: one started for the mission,
// or in one of its capsules; or in the repo's worktree.
function inPlace(r: RunDetail, p: BelowPlace, missions: Mission[]): boolean {
  if ("repo" in p) return r.repo === p.repo && r.worktree === p.worktree;
  if (r.mission === p.mission) return true;
  const m = missions.find((x) => x.slug === p.mission);
  return !!m?.capsules.some((c) => c.repo === r.repo && c.name === r.worktree);
}

// ofPlace is the place's services, of those running: its own, and what
// they need, started where it's nearest (records' api, for a capsule's
// web), as what's here doesn't work without it.
export function ofPlace(services: RunDetail[], p: BelowPlace, missions: Mission[]): RunDetail[] {
  const own = new Set(services.filter((r) => inPlace(r, p, missions)).map((r) => r.ref));
  return services.filter((r) => own.has(r.ref) || r.neededBy?.some((ref) => own.has(ref)));
}

// services is the services that are running, the newest first.
export const services = (runs: RunDetail[]): RunDetail[] => split(runs).running.filter((r) => r.type !== "task");

// isNew is whether a service started after Below was last closed (seen):
// one you haven't had in front of you yet. Before anything's been seen,
// nothing is.
export const isNew = (r: RunDetail, seen?: number): boolean => seen !== undefined && Date.parse(r.started) > seen;

// jumpTo is where a service is gone to: its worktree's Runs tab, with it
// picked (its preview, how it was run, all it's printed); or, for one of
// the workspace's own or a silo's, which have no such tab, the Runs page
// with it open.
export function jumpTo(r: RunDetail): string {
  if (!r.repo || r.worktree === "silo") return `/runs?show=${encodeURIComponent(r.ref)}`;
  const at = r.worktree === "ground" ? `/files/${encodeURIComponent(r.repo)}` : `/files/${encodeURIComponent(r.repo)}/${encodeURIComponent(r.worktree)}`;
  return `${at}?view=runs&service=${encodeURIComponent(r.recipe ?? r.name)}`;
}
