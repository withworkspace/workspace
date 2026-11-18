import type { Bundle, MissionEvent, Stage } from "../api";
import type { IconName } from "../ui/Icon";

// What the plan beside a mission works out before it draws: which steps
// show when there are too many to list, what a folded run says of itself,
// whether the step it's on is worth opening, and what the log and the
// bundles know of a step.

// A plan of up to planShort steps is listed whole. A longer one shows the
// step it's on with planBefore before it and planAfter after: what's just
// been done, and what's next, which matters more.
export const planShort = 9;
export const planBefore = 2;
export const planAfter = 3;

export interface PlanWindow {
  earlier: Stage[]; // folded away, before what's shown
  shown: Stage[];
  later: Stage[]; // folded away, after it
}

// planWindow is the steps to show, round the one the plan is on (the last,
// once they're all done; the first to come, when none is current), and
// those folded away on either side. A run of one isn't folded: its row
// would take the room the step does.
export function planWindow(stages: Stage[], before = planBefore, after = planAfter, short = planShort): PlanWindow {
  if (stages.length <= short) return { earlier: [], shown: stages, later: [] };
  let at = stages.findIndex((s) => s.state === "current");
  if (at < 0) at = stages.findIndex((s) => s.state === "todo");
  if (at < 0) at = stages.length - 1;
  let from = Math.max(0, at - before);
  let to = Math.min(stages.length - 1, at + after);
  if (from === 1) from = 0;
  if (to === stages.length - 2) to = stages.length - 1;
  return { earlier: stages.slice(0, from), shown: stages.slice(from, to + 1), later: stages.slice(to + 1) };
}

// stepYours is whether a step is yours alone to pass: a review, a
// decision, a landing.
export const stepYours = (s: Stage) => s.who === "you";

// checksUnmet is whether a step's checks have been run and didn't all
// come to met.
export const checksUnmet = (s: Stage) => !!s.round && (s.checks ?? []).some((c) => c.result !== "met");

// passedUnmet is a step that was passed with a check not met: worth
// knowing of a step that's folded away.
export const passedUnmet = (s: Stage) => s.state === "accepted" && checksUnmet(s);

// FoldNote is what's worth saying of the steps folded away, beside how
// many there are.
export interface FoldNote {
  text: string;
  tone?: "warn" | "yours";
}

// foldNote says, of those before, whether any was passed with a check
// unmet; of those to come, how many are yours and which is next.
export function foldNote(run: Stage[], side: "earlier" | "later"): FoldNote | undefined {
  if (side === "earlier") {
    const unmet = run.filter(passedUnmet).length;
    return unmet ? { text: `${unmet} passed unmet`, tone: "warn" } : { text: "all done" };
  }
  const yours = run.filter((s) => s.state === "todo" && stepYours(s));
  return yours.length ? { text: `${yours.length} yours, next ${yours[0].n}`, tone: "yours" } : undefined;
}

// worthOpening is whether the step the plan is on says more opened than
// its row does: what it makes, who's on it, its checks, its clock; and of
// a Wait, how it's passed, which is the thing to know while it waits. A
// step that's only its kind and whose it is (ws opening a pull request, a
// review that's yours) stays a row.
export const worthOpening = (s: Stage) => !!(s.makes || s.crew?.length || s.checks?.length || s.limit || s.carryOn || s.onlyIf || s.inEach || s.attempts || s.type === "Wait");

// WaitHow is how a Wait step is passed: whether anything's watching for
// what it waits for, in a word, and what that means, in a sentence.
export interface WaitHow {
  auto: boolean; // ws watches for it, and passes it
  word: string;
  says: string;
}

// waitHow says how a Wait is passed. Watched, ws runs its checks and
// passes it by itself; otherwise nothing does, and it waits for you (when
// it's yours to do) or for whoever can say it's happened.
export function waitHow(s: Stage): WaitHow | undefined {
  if (s.type !== "Wait") return undefined;
  const yours = s.on === "you";
  if (s.passes === "watched") {
    return { auto: true, word: "Watched", says: yours ? "ws runs its check every few minutes, and passes it; or mark it done, and ws looks then" : "ws runs its check every few minutes, and passes it by itself" };
  }
  if (yours) return { auto: false, word: "Manual", says: "nothing's watching for it: it moves when you mark it done" };
  return { auto: false, word: "Manual", says: "nothing's watching for it: it moves when someone marks it done" };
}

// watchWords is what ws's watch of a Wait step last came to: "ws last
// looked 14:02: not yet", or that it hasn't looked.
export function watchWords(s: Stage, at: (iso: string) => string): string {
  const w = s.watch;
  if (!w?.looked) return "ws hasn't looked yet";
  return `ws last looked ${at(w.looked)}${w.came ? `: ${w.came}` : ""}`;
}

// stepKind is a plan's step's kind as an icon and a word. A brief's stage
// has none.
const kinds: Record<NonNullable<Stage["type"]>, [IconName, string]> = {
  Work: ["tool", "Work"],
  Verify: ["list-checks", "Verify"],
  Review: ["eye", "Review"],
  Decide: ["gavel", "Decide"],
  "Open Pull Request": ["pr", "Open a pull request"],
  Land: ["arrow-down", "Land"],
  Wait: ["clock", "Wait"],
  Plan: ["layers", "Plan"],
};
export const stepKind = (s: Stage): [IconName, string] | undefined => (s.type ? kinds[s.type] : undefined);

// StepTime is when a step was last started and when it was accepted, as
// the mission's log has them.
export interface StepTime {
  from?: string;
  to?: string;
}

// stepTimes reads each step's times from the log ("on step 3: …", "step 3
// accepted: …"; a brief's stages say "stage"). A step started again is
// timed from its last start.
export function stepTimes(events: MissionEvent[]): Map<number, StepTime> {
  const times = new Map<number, StepTime>();
  for (const e of events) {
    if (e.kind !== "stage") continue;
    const on = /^on (?:step|stage) (\d+)\b/.exec(e.text);
    if (on) {
      times.set(Number(on[1]), { from: e.time });
      continue;
    }
    const done = /^(?:step|stage) (\d+) accepted\b/.exec(e.text);
    if (done) times.set(Number(done[1]), { ...times.get(Number(done[1])), to: e.time });
  }
  return times;
}

// stepBundle is the bundle that shows a step's work: the latest of its
// step that nothing has taken the place of.
export function stepBundle(bundles: Bundle[], n: number): Bundle | undefined {
  const mine = bundles.filter((b) => b.step === n);
  return mine.filter((b) => !b.replacedBy).pop() ?? mine.pop();
}

// stepText is a step's text without the title it begins with ("Build
// ui-lib. Build the package…"), which the card says above it.
export function stepText(s: Stage): string {
  const lead = `${s.title}.`;
  return s.text.startsWith(lead) ? s.text.slice(lead.length).trim() : s.text;
}
