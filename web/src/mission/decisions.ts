import type { Decision, Stage } from "../api";

// A mission's decisions as its page lays them out: yours apart from the
// rest, since yours are few and stand over what the plan asks, and the
// rest (crew's, the orchestrator's) are many and mostly read when a step
// is; those by the step each was made at.

// decisionsPath is where a mission's decisions are on its page, and one
// of them (d4) there.
export const decisionsPath = (slug: string, id?: string) => `/missions/${encodeURIComponent(slug)}?tab=decisions${id ? `&d=${encodeURIComponent(id)}` : ""}`;

// DecisionStep is the decisions made at a step, in the order they were
// made: n is the step's number (0: at none), step the plan's own, where
// the plan still has one by that number.
export interface DecisionStep {
  n: number;
  step?: Stage;
  list: Decision[];
}

export interface DecisionGroups {
  // Those of yours that stand, the latest first: the ones you recorded or
  // confirmed as decisions.
  yours: Decision[];
  // Yours by answering: what you said when you were asked, the latest
  // first, each with its question.
  answered: Decision[];
  // The rest that stand, by step in the plan's order, then those at none.
  work: DecisionStep[];
  // How many work holds.
  worked: number;
  // Those replaced or withdrawn, the latest first.
  gone: Decision[];
}

// stepOf is the step a decision was made at: as ws kept it, or (one from
// before it did) the latest step its crew was on; 0 for none.
export function stepOf(d: Decision, stages: Stage[]): number {
  if (d.step) return d.step;
  for (let i = stages.length - 1; i >= 0; i--) if (stages[i].crew?.includes(d.by)) return stages[i].n;
  return 0;
}

const made = (a: Decision, b: Decision) => a.at.localeCompare(b.at) || number(a.id) - number(b.id);
const number = (id: string) => Number(id.slice(1)) || 0;

export function groupDecisions(list: Decision[], stages: Stage[]): DecisionGroups {
  const standing = list.filter((d) => d.state === "current");
  const mine = standing.filter((d) => d.by === "you").sort((a, b) => made(b, a));
  const yours = mine.filter((d) => !d.asked);
  const answered = mine.filter((d) => !!d.asked);
  const rest = standing.filter((d) => d.by !== "you");
  const at = new Map<number, Decision[]>();
  for (const d of rest) {
    const n = stepOf(d, stages);
    at.set(n, [...(at.get(n) ?? []), d]);
  }
  const work = [...at.entries()]
    // In the plan's order, with those at no step last.
    .sort(([a], [b]) => (a || Infinity) - (b || Infinity))
    .map(([n, l]): DecisionStep => ({ n, step: stages.find((s) => s.n === n), list: l.sort(made) }));
  return { yours, answered, work, worked: rest.length, gone: list.filter((d) => d.state !== "current").sort((a, b) => made(b, a)) };
}

// whoDecided is who made a decision, in words: you, the orchestrator, ws,
// or crew by their id.
export function whoDecided(by: string): string {
  switch (by) {
    case "you":
      return "You";
    case "orchestrator":
      return "The orchestrator";
    case "ws":
      return "ws";
  }
  return by;
}

// decisionsWords says how many decisions stand and whose, for the way to
// them from Summary: "2 of yours and 11 from the work", "11 from the
// work"; "" for none.
export function decisionsWords(g: Pick<DecisionGroups, "yours" | "answered" | "worked">): string {
  const mine = g.yours.length + g.answered.length;
  const parts = [mine ? `${mine} of yours` : "", g.worked ? `${g.worked} from the work` : ""].filter(Boolean);
  return parts.join(" and ");
}

// whoAsked is who asked the question a decision answers, in words: the
// orchestrator, ws, or crew by their id.
export function whoAsked(by: string): string {
  switch (by) {
    case "orchestrator":
      return "The orchestrator";
    case "ws":
      return "ws";
  }
  return by;
}

// DecisionEffect is somewhere a decision was reckoned with: a check of a
// step whose latest verdict names it (with what the verdict was), or a
// step passed for a reason that names it.
export interface DecisionEffect {
  step: number;
  // The check, counted from 1 as the step lists them; none for the step
  // itself, passed.
  check?: number;
  result?: string;
  // The check's words, or the step's name.
  text: string;
}

// named is whether text names a decision (d4) as a word of its own: not
// in d41, a path or another name.
const named = (text: string | undefined, id: string) => !!text && new RegExp(`(?<![\\w/.:-])${id}(?![\\w-])`).test(text);

// effectsOf is where a decision was reckoned with, in the plan's order.
// The judge is told to name the decision a verdict rests on, so a check
// whose verdict names one was judged with it: met as the decision leaves
// it, or unmet all the same.
export function effectsOf(id: string, stages: Stage[]): DecisionEffect[] {
  const out: DecisionEffect[] = [];
  for (const s of stages) {
    (s.checks ?? []).forEach((c, i) => {
      if (c.result && named(c.evidence, id)) out.push({ step: s.n, check: i + 1, result: c.result, text: c.text });
    });
    if (s.state === "accepted" && named(s.why, id)) out.push({ step: s.n, text: s.title });
  }
  return out;
}
