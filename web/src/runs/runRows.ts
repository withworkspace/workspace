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
// "For". A run asked for again while it ran is the later askers' too
// (r.also of them): it ends once the last has gone.
export function forWhom(r: RunDetail, missions: Mission[], talks: Talk[] = []): Said {
  const first = firstFor(r, missions, talks);
  if (!r.also) return first;
  return { ...first, text: `${first.text}, and ${r.also === 1 ? "one other who" : `${r.also} others who`} asked for it since` };
}

function firstFor(r: RunDetail, missions: Mission[], talks: Talk[]): Said {
  const mission = r.mission ? missionNamed(missions, r.mission) : undefined;
  switch (r.owner) {
    case "run":
      // Started because another run needed it: it lasts as long as one does.
      return { text: r.neededBy?.length ? `${listed(r.neededBy)}, which ${r.neededBy.length === 1 ? "needs" : "need"} it` : "what needed it, which has ended" };
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
      // An app: the workspace's page for it, or its tab on a worktree's.
      if (r.app && !r.repo) return { text: "whoever has its page open", to: `/apps/${r.app}` };
      if (r.app) return { text: "whoever has its tab open", to: `${filesOf(r)}?view=${encodeURIComponent(r.app)}` };
      // Started from a mission's page, or with none, from the Files page.
      if (mission) return { text: `whoever has ${mission.text} open`, to: mission.to };
      return { text: "whoever has its files open", to: filesOf(r) };
    case "none":
      return { text: "nobody: it lasts until it's stopped, or its time's up" };
  }
  return { text: r.owner };
}

// filesOf is the Repos page of the worktree a run runs in.
const filesOf = (r: RunDetail) => (r.worktree === "ground" ? `/files/${r.repo}` : `/files/${r.repo}/${r.worktree}`);

// listed is names as a sentence has them: a, b and c.
export function listed(names: string[]): string {
  if (names.length < 2) return names.join("");
  return `${names.slice(0, -1).join(", ")} and ${names[names.length - 1]}`;
}

// placeOf is where a run runs: its repo and worktree, going to the
// mission whose capsule it is, or else to the worktree's files.
export function placeOf(r: RunDetail, missions: Mission[]): Said {
  // The workspace's own runs run at its root, and belong to no repo.
  if (!r.repo) return { text: "the workspace" };
  const text = `${r.repo} / ${r.worktree}`;
  if (r.worktree === "ground") return { text, to: `/files/${r.repo}` };
  if (r.worktree === "silo") return { text };
  const m = missions.find((x) => x.capsules.some((c) => c.repo === r.repo && c.name === r.worktree));
  return { text, to: m ? `/missions/${m.slug}` : `/files/${r.repo}/${r.worktree}` };
}

// split is runs as the page groups them, each newest first: what's
// running, services and tasks alike; the tasks that have ended, kept for
// what they came to; and the services that ended by themselves.
export function split(runs: RunDetail[]): { running: RunDetail[]; results: RunDetail[]; ended: RunDetail[] } {
  const newest = (a: RunDetail, b: RunDetail) => b.started.localeCompare(a.started);
  const lastEnded = (a: RunDetail, b: RunDetail) => (b.ended ?? b.started).localeCompare(a.ended ?? a.started);
  const over = runs.filter((r) => r.state !== "running");
  return {
    running: runs.filter((r) => r.state === "running").sort(newest),
    results: over.filter((r) => r.type === "task").sort(lastEnded),
    ended: over.filter((r) => r.type !== "task").sort(lastEnded),
  };
}

// took is how long a run ran, as it's said: 0.4s, 42s, 3m 05s, 1h 02m.
export function took(ms: number | undefined): string {
  if (!ms || ms < 0) return "";
  if (ms < 100) return "under 0.1s";
  if (ms < 10_000) return `${(ms / 1000).toFixed(1)}s`;
  const s = Math.round(ms / 1000);
  if (s < 60) return `${s}s`;
  const two = (n: number) => String(n).padStart(2, "0");
  if (s < 3600) return `${Math.floor(s / 60)}m ${two(s % 60)}s`;
  return `${Math.floor(s / 3600)}h ${two(Math.floor((s % 3600) / 60))}m`;
}

// came is what a task came to, in words: "passed in 42s", "failed (exit
// 2) in 3m 05s", "stopped: its time was up".
export function came(r: RunDetail): string {
  const t = took(r.took);
  switch (r.result) {
    case "running":
      return "running";
    case "passed":
      return t ? `passed in ${t}` : "passed";
    case "failed":
      return `failed (exit ${r.status ?? 1})${t ? ` in ${t}` : ""}`;
  }
  return r.why ? `stopped: ${r.why}` : "stopped before it ended";
}

// commitSaid is the commit a task ran at, short, and whether the
// worktree had changes that weren't committed.
export function commitSaid(r: RunDetail): string {
  if (!r.commit) return "";
  return r.commit.slice(0, 9) + (r.dirty ? ", with changes not committed" : "");
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
