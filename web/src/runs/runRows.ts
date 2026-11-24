import type { Mission, RecipeInfo, RunDetail, Talk } from "../api";
import type { StatusInfo } from "../ui/Status";

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

// runStatus is where a run stands, as a Status draws it: a service up,
// coming up, or ended (and how); a task running, or what it came to.
export function runStatus(r: RunDetail): StatusInfo {
  if (r.type === "task") {
    switch (r.result) {
      case "running":
        return { icon: "spinner", tone: "progress", words: "Running", live: true };
      case "passed":
        return { icon: "check-circle", tone: "done", words: "Passed" };
      case "failed":
        return { icon: "x-circle", tone: "danger", words: "Failed" };
    }
    return { icon: "circle-dashed", tone: "quiet", words: "Stopped" };
  }
  if (r.state === "running") return r.up ? { icon: "circle-dot", tone: "done", words: "Up", live: true } : { icon: "spinner", tone: "progress", words: "Starting", live: true };
  if (r.state === "exited") return r.status ? { icon: "x-circle", tone: "danger", words: `Exited ${r.status}` } : { icon: "circle", tone: "quiet", words: "Exited" };
  return { icon: "circle-dashed", tone: "quiet", words: "Gone" };
}

// runId is how a run is picked on a page: a task's run by its number
// (ref#n), as its earlier runs are picked too; a service by its ref.
export const runId = (r: RunDetail) => r.nref ?? r.ref;

// startedBy is who started a run, and how, in a few words: "you, from a
// terminal", "crew c2", "ws, for check 1 of step 2".
export function startedBy(r: RunDetail): string {
  if (r.check) return `ws, for check ${r.check.n} of step ${r.check.step}`;
  if (r.owner === "run") return r.neededBy?.length ? `ws, as ${listed(r.neededBy.map(nameOfRef))} ${r.neededBy.length === 1 ? "needs" : "need"} it` : "ws, for a run that needed it";
  const who = !r.by || r.by === "you" ? "you" : r.by === "ws" ? "ws" : `crew ${r.by}`;
  switch (r.owner) {
    case "pid":
      return `${who}, from a terminal`;
    case "looking":
      if (r.app) return `${who}, by opening its ${r.app} tab`;
      return r.mission ? `${who}, from the mission's page` : `${who}, from its files`;
    case "conversation":
      return `${who}, in a conversation`;
  }
  return who;
}

// nameOfRef is a run's name, from its ref (name.worktree.repo): what a
// line about the runs of one place names another by.
export const nameOfRef = (ref: string) => ref.split(".")[0];

// A place's runs, as its Runs tab lists them: its services, a row for
// each, apps apart from tools; and its tasks' runs.
export interface PlaceRuns {
  apps: ServiceRow[];
  tools: ServiceRow[];
  tasks: RunDetail[];
}

// ServiceRow is one of a place's services: its recipe (none for a command
// run as it is), and its run, running or the last to end (kept an hour),
// if it has one. name is the recipe's, or the run's.
export interface ServiceRow {
  name: string;
  recipe?: RecipeInfo;
  run?: RunDetail;
}

// placeRuns is a place's runs and recipes as its Runs tab lists them.
// Each service recipe is a row whether it's running or not, with its
// run, in the order its file's names have; after them, services run as
// commands, with no recipe here (apps, as nothing says otherwise). Its
// tasks' runs are what's running, then the rest, the last to end first,
// the runs of a task kept behind its latest (past) among them.
export function placeRuns(runs: RunDetail[], past: RunDetail[], recipes: RecipeInfo[]): PlaceRuns {
  const services = runs.filter((r) => r.type !== "task");
  const rows: ServiceRow[] = recipes.filter((rc) => rc.type !== "task").map((rc) => ({ name: rc.name, recipe: rc, run: services.find((r) => r.recipe === rc.name) }));
  const named = new Set(rows.map((s) => s.name));
  for (const r of services) if (!r.recipe || !named.has(r.recipe)) rows.push({ name: r.name, run: r });
  const { running, results } = split([...runs.filter((r) => r.type === "task"), ...past]);
  return {
    apps: rows.filter((s) => s.recipe?.kind !== "tool"),
    tools: rows.filter((s) => s.recipe?.kind === "tool"),
    tasks: [...running, ...results],
  };
}

// Picked is what a place's Runs tab shows of its runs: a service, or one
// run of a task.
export type Picked = { service: ServiceRow; task?: undefined } | { task: RunDetail; service?: undefined };

// pickedIn is what's shown of a place's runs: the service asked for by
// its name, or the task's run by its id (runId, or a task's ref alone for
// its latest); or else the first worth a look: a service that's up, then
// one running, then a task running, then the last task to end, then the
// first app.
export function pickedIn(p: PlaceRuns, ask: { service?: string; run?: string }): Picked | undefined {
  const services = [...p.apps, ...p.tools];
  const service = ask.service ? services.find((s) => s.name === ask.service) : undefined;
  if (service) return { service };
  if (ask.run) {
    const task = p.tasks.find((r) => runId(r) === ask.run) ?? p.tasks.filter((r) => r.ref === ask.run).sort((a, b) => (b.n ?? 0) - (a.n ?? 0))[0];
    if (task) return { task };
  }
  const live = services.find((s) => s.run?.state === "running" && s.run.up) ?? services.find((s) => s.run?.state === "running");
  if (live) return { service: live };
  const task = p.tasks[0];
  if (task) return { task };
  return services[0] && { service: services[0] };
}
