import type { Artifact, Bundle, CapsuleDetail, ChangedFile, CrewDetail, MissionDetail, Stage } from "../api";
import type { IconName } from "../ui/Icon";
import type { Tone } from "../ui/tone";

// What the mission's Work tab works out from the mission: its capsules in
// the order they need you, each with the steps that ran in it, its crew
// and what was made there, and its outputs.

export const capsuleId = (c: Pick<CapsuleDetail, "repo" | "name">) => `${c.repo}/${c.name}`;
export const capsuleKey = (c: Pick<CapsuleDetail, "repo" | "name">) => `c:${capsuleId(c)}`;
export const outputKey = (a: Pick<Artifact, "ref">) => `a:${a.ref}`;

// Where a capsule is in the list: in hand (something's happening in it, or
// it holds work that hasn't landed), up next (lifted, and nothing's been
// done in it yet), or done (landed, merged or burned).
export type Where = "hand" | "next" | "done";

// WorkState is where a capsule stands, in a chip.
export interface WorkState {
  text: string;
  tone?: Tone;
  icon?: IconName;
  live?: boolean; // crew are at work in it now
  url?: string; // its pull request, when that's what it says
}

export interface WorkCapsule {
  c: CapsuleDetail;
  id: string; // repo/name
  key: string; // c:repo/name
  where: Where;
  state: WorkState;
  steps: Stage[]; // the plan's steps that run in it, in the plan's order
  now?: Stage; // the step the mission's on, when it's one of them
  crew: CrewDetail[]; // who's been in it, first first
  live?: CrewDetail; // who's in it now
  made: Artifact[]; // what was made in it
  bundles: Bundle[]; // the verification bundles of it
}

export interface Work {
  capsules: WorkCapsule[]; // in the order the list has them
  hand: WorkCapsule[];
  next: WorkCapsule[];
  done: WorkCapsule[];
  outputs: Artifact[]; // documents first, then handoffs, newest first
  keys: string[]; // every row's key, in the list's order
  opensOn?: string; // the row it shows before one's picked
}

// A step that says nothing of where it runs, and has no crew, is in the
// capsule of the step before when it's one of these: they look at, verify,
// land or open a pull request for what a step before made.
const follows = new Set<Stage["type"]>(["Review", "Verify", "Land", "Open Pull Request"]);

// stepCapsules is which capsules each step runs in, by its number. The
// crew who took it say exactly; else where the plan says it works (a
// capsule, or a repo: then that repo's capsule that's still here, or its
// latest); else, for a step that follows on, where the step before ran.
export function stepCapsules(d: MissionDetail): Map<number, string[]> {
  const ids = new Set(d.capsules.map(capsuleId));
  const inRepo = (repo: string) => {
    const list = d.capsules.filter((c) => c.repo === repo);
    return (list.find((c) => c.exists && !c.landed) ?? list[list.length - 1]) as CapsuleDetail | undefined;
  };
  const out = new Map<number, string[]>();
  let last: string[] = [];
  for (const s of d.stages) {
    let at = (s.crew ?? []).map((id) => d.crew.find((c) => c.id === id)?.capsule).filter((x): x is string => !!x && ids.has(x));
    if (!at.length && s.runs?.length) at = s.runs.map((r) => (r.capsule && ids.has(r.capsule) ? r.capsule : inRepo(r.repo) && capsuleId(inRepo(r.repo)!))).filter((x): x is string => !!x);
    if (!at.length && s.in) {
      const named = ids.has(s.in) ? s.in : inRepo(s.in) && capsuleId(inRepo(s.in)!);
      if (named) at = [named];
    }
    if (!at.length && follows.has(s.type)) at = last;
    at = [...new Set(at)];
    if (at.length) {
      out.set(s.n, at);
      last = at;
    }
  }
  return out;
}

const day = (iso: string) => new Date(iso).toLocaleDateString([], { day: "numeric", month: "short" });

const asks = (c?: CrewDetail) => c?.activity === "permission" || c?.activity === "question";
// At work now, as the crew's status takes it: one with no activity to go
// by (a harness with no hooks) is working until it says it's done.
const working = (c?: CrewDetail) => !!c && !c.ended && !asks(c) && (!!c.starting || !!c.again || c.activity === "working" || (!c.activity && c.status !== "done"));

// capsuleState is where a capsule stands, the most pressing thing first:
// crew asking or at work, a step of yours, a pull request to open, then
// how it landed or what it holds.
export function capsuleState(c: CapsuleDetail, steps: Stage[], live?: CrewDetail): WorkState {
  const now = steps.find((s) => s.state === "current");
  if (live && asks(live)) return { text: `${live.id} asks`, tone: "attention" };
  if (live && working(live)) return { text: `${live.id} working`, tone: "progress", live: true };
  if (now?.who === "you") {
    const words: Partial<Record<NonNullable<Stage["type"]>, string>> = { Review: "Yours to check", Land: "Yours to land", Decide: "Yours to decide" };
    return { text: (now.type && words[now.type]) || "Yours", tone: "attention", icon: now.type === "Review" ? "eye" : undefined };
  }
  if (now?.opensPR && !c.pr && c.exists) return { text: "Pull request to open", tone: "attention", icon: "pr" };
  if (c.landed) return { text: `Landed ${day(c.landed)}`, tone: "done", icon: "check" };
  if (c.pr?.merged) return { text: `#${c.pr.number} merged`, tone: "done", icon: "pr", url: c.pr.url };
  if (!c.exists) return { text: "Burned", tone: "quiet" };
  if (c.pr) {
    if (/fail/i.test(c.pr.checks ?? "")) return { text: `#${c.pr.number} checks failing`, tone: "danger", icon: "pr", url: c.pr.url };
    return { text: `#${c.pr.number} open`, tone: "waiting", icon: "pr", url: c.pr.url };
  }
  if (c.toLand) return { text: `${c.toLand} to land`, tone: "waiting" };
  if (now) return { text: `Step ${now.n} now`, tone: "progress" };
  if (c.dirty) return { text: "Not committed", tone: "attention" };
  if (c.ahead) return { text: `${c.ahead} ahead`, tone: "quiet" };
  const next = steps.find((s) => s.state === "todo");
  return next ? { text: `From step ${next.n}`, tone: "quiet" } : { text: "Nothing yet", tone: "quiet" };
}

export function work(d: MissionDetail): Work {
  const at = stepCapsules(d);
  const capsules = d.capsules.map((c): WorkCapsule => {
    const id = capsuleId(c);
    const steps = d.stages.filter((s) => at.get(s.n)?.includes(id));
    const crew = d.crew.filter((x) => x.capsule === id);
    const live = [...crew].reverse().find((x) => !x.ended);
    const handoffs = new Set(crew.map((x) => x.handoff).filter(Boolean));
    const made = d.artifacts.filter((a) => handoffs.has(a.ref) || a.ref.startsWith(`${id}:`));
    const now = steps.find((s) => s.state === "current");
    const done = !!c.landed || !c.exists || !!c.pr?.merged;
    const touched = crew.length > 0 || !!c.dirty || !!c.ahead || !!c.toLand || !!c.pr || steps.some((s) => s.state !== "todo");
    const where: Where = now || working(live) || asks(live) ? "hand" : done ? "done" : touched ? "hand" : "next";
    return { c, id, key: capsuleKey(c), where, state: capsuleState(c, steps, live), steps, now, crew, live, made, bundles: d.bundles.filter((b) => b.capsule === id) };
  });
  const group = (w: Where) => capsules.filter((x) => x.where === w);
  const hand = group("hand"), next = group("next"), done = group("done");
  const handoff = (a: Artifact) => a.kind === "handoff";
  const outputs = [...d.artifacts.filter((a) => !handoff(a)), ...d.artifacts.filter(handoff).sort((a, b) => b.added.localeCompare(a.added))];
  const listed = [...hand, ...next, ...done];
  const first = listed.find((x) => x.now) ?? listed[0];
  return { capsules: listed, hand, next, done, outputs, keys: [...listed.map((x) => x.key), ...outputs.map(outputKey)], opensOn: first?.key ?? (outputs[0] && outputKey(outputs[0])) };
}

// Home is where an output came from: the crew who left it (a handoff's),
// the capsule it was made in, and the step that was for.
export interface Home {
  crew?: CrewDetail;
  capsule?: WorkCapsule;
  step?: Stage;
}

export function outputHome(d: MissionDetail, w: Work, a: Artifact): Home {
  const crew = d.crew.find((c) => c.handoff === a.ref);
  const capsule = w.capsules.find((x) => x.made.some((m) => m.ref === a.ref));
  const steps = crew ? d.stages.filter((s) => s.crew?.includes(crew.id)) : [];
  return { crew, capsule, step: steps[steps.length - 1] };
}

// crewElsewhere is the crew the Work tab doesn't show by capsule: those
// that worked in none of the mission's (a briefing agent, or crew in a
// capsule that ended alone), which the log keeps.
export function crewElsewhere(d: Pick<MissionDetail, "crew" | "capsules">): CrewDetail[] {
  const ids = new Set(d.capsules.map(capsuleId));
  return d.crew.filter((c) => !c.capsule || !ids.has(c.capsule));
}

// FileGroup is a capsule's changed files under one top-level folder ("" for
// those at the top), in the order they came, with their lines counted.
export interface FileGroup {
  dir: string;
  files: ChangedFile[];
  additions: number;
  deletions: number;
}

export function fileGroups(files: ChangedFile[], filter = ""): FileGroup[] {
  const q = filter.trim().toLowerCase();
  const groups = new Map<string, FileGroup>();
  for (const f of files) {
    if (q && !f.path.toLowerCase().includes(q)) continue;
    const i = f.path.indexOf("/");
    const dir = i < 0 ? "" : f.path.slice(0, i);
    const g = groups.get(dir) ?? { dir, files: [], additions: 0, deletions: 0 };
    g.files.push(f);
    g.additions += f.additions;
    g.deletions += f.deletions;
    groups.set(dir, g);
  }
  return [...groups.values()];
}
