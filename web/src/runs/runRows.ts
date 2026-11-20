import type { Mission, RunDetail, Talk } from "../api";

// What the Runs page says of a run: who it's for, where it runs, each
// with somewhere to go when there's a page for it, and the order runs
// come in.

export interface Said {
  text: string;
  to?: string; // a page of the web UI's
}

// missionNamed is a mission's title, by its slug: the slug itself for one
// the workspace no longer lists.
function missionNamed(missions: Mission[], slug: string): Said {
  const m = missions.find((x) => x.slug === slug);
  return { text: m?.title || slug, to: `/missions/${slug}` };
}

// forWhom is who a run is for: what it ends with. Its words read after
// "For".
export function forWhom(r: RunDetail, missions: Mission[], talks: Talk[] = []): Said {
  const mission = r.mission ? missionNamed(missions, r.mission) : undefined;
  switch (r.owner) {
    case "pid":
      return { text: r.ownerCommand ? `${r.ownerCommand} (pid ${r.ownerPid})` : `pid ${r.ownerPid}` };
    case "capsule":
      return { text: `capsule ${r.worktree}`, to: placeOf(r, missions).to };
    case "mission":
      return mission ?? { text: "a mission" };
    case "step":
      return mission ? { text: `step ${r.step} of ${mission.text}`, to: mission.to } : { text: `step ${r.step}` };
    case "crew":
      return mission ? { text: `crew ${r.crew} of ${mission.text}`, to: `/missions/${r.mission}/crew/${r.crew}` } : { text: `crew ${r.crew}` };
    case "conversation": {
      const t = talks.find((x) => x.id === r.conversation);
      const text = t?.title ? `the conversation “${t.title}”` : "a conversation";
      return { text, to: r.mission ? `/missions/${r.mission}/conversations/${r.conversation}` : `/conversations/${r.conversation}` };
    }
    case "looking":
      return mission ? { text: `whoever has ${mission.text} open`, to: mission.to } : { text: "whoever has its page open" };
    case "none":
      return { text: "nobody: it lasts until it's stopped, or its time's up" };
  }
  return { text: r.owner };
}

// placeOf is where a run runs: its repo and worktree, going to the
// mission whose capsule it is, or else to the worktree's files.
export function placeOf(r: RunDetail, missions: Mission[]): Said {
  const text = `${r.repo} / ${r.worktree}`;
  if (r.worktree === "ground") return { text, to: `/files/${r.repo}` };
  if (r.worktree === "silo") return { text };
  const m = missions.find((x) => x.capsules.some((c) => c.repo === r.repo && c.name === r.worktree));
  return { text, to: m ? `/missions/${m.slug}` : `/files/${r.repo}/${r.worktree}` };
}

// running and ended split runs, each newest first.
export function split(runs: RunDetail[]): { running: RunDetail[]; ended: RunDetail[] } {
  const newest = (a: RunDetail, b: RunDetail) => b.started.localeCompare(a.started);
  return {
    running: runs.filter((r) => r.state === "running").sort(newest),
    ended: runs.filter((r) => r.state !== "running").sort((a, b) => (b.ended ?? b.started).localeCompare(a.ended ?? a.started)),
  };
}

// bare is a link as it's read: no scheme, no trailing slash.
export const bare = (link: string) => link.replace(/^https?:\/\//, "").replace(/\/$/, "");

// plain is a log's text without the terminal's colours and cursor moves
// (ANSI escapes), which a dev server writes even to a file.
// eslint-disable-next-line no-control-regex
const escapes = /\x1b\[[0-?]*[ -/]*[@-~]|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)|\x1b[@-Z\\-_]/g;
export const plain = (log: string) =>
  log
    .replace(escapes, "")
    .replace(/\r(?!\n)/g, "\n")
    .replace(/^\s*\n/, "");
