import type { CrewPick, MissionAct, MissionCard, MissionDetail } from "../../api";

// What a mission's Summary works out of it, apart from drawing it.

// How many of its latest events Summary shows: more beside its details,
// where there's less else to show.
export const summaryRecent = 5;
export const summaryRecentWithSide = 7;

// isNotice is a harness's warning the orchestrator passed on as what it
// said ("this sign-in ends between …"): about the whole workspace, not the
// mission, so it's shown once, dismissably, and kept out of Recently.
export const isNotice = (text: string) => /^orchestrator: (warning|notice): /i.test(text);

// saidLately is the crew still there who've said something and aren't at
// work: their last word is shown with where the mission stands.
export const saidLately = (d: Pick<MissionDetail, "crew">) => d.crew.filter((c) => !c.ended && c.lastReply && c.activity !== "working");

// pickName is a harness and model as [crew] allowed writes it.
export const pickName = (p: CrewPick) => (p.model ? `${p.harness}:${p.model}` : p.harness);

// What putting a card aside does is done to the first question a mission
// has, whichever card says it: only the lead offers these.
const leadOnly = new Set(["dismiss", "ask-agent", "not-yet"]);

// actsOf is what a card offers, in the order it's given: the one meant to
// move it first, then the rest; a notice's without those only the lead
// offers.
export const actsOf = (c: Pick<MissionCard, "primary" | "more">, lead: boolean): MissionAct[] =>
  [...(c.primary ? [c.primary] : []), ...(c.more ?? [])].filter((a) => lead || !leadOnly.has(a.do));

// ActsShape is how a card's acts are drawn: what crew are stopped on, to
// answer in place (prompt); a reply to crew waiting (reply); a way to
// crew's page (crew); nothing (none); or the acts themselves (acts).
export type ActsShape = "prompt" | "reply" | "crew" | "none" | "acts";

// actsShape is how a card's acts are drawn. Crew stopped on something are
// answered where they are: what they're asking, here; a reply, written
// here.
export function actsShape(d: Pick<MissionDetail, "crew">, c: Pick<MissionCard, "primary" | "more">, lead: boolean): ActsShape {
  const acts = actsOf(c, lead);
  const first = c.primary && acts[0] === c.primary ? c.primary : undefined;
  if (!first || (first.do !== "reply" && first.do !== "permission")) return "acts";
  const crew = d.crew.find((x) => x.id === first.target && !x.ended);
  if (crew && (crew.activity === "permission" || crew.activity === "question")) return "prompt";
  if (crew && first.do === "reply" && crew.status !== "asking") return "reply";
  if (crew) return "crew";
  return "none";
}

// actsWide is whether a lead's acts take its width, under what it says,
// rather than sitting beside it: everything but a way to crew's page.
export const actsWide = (shape: ActsShape) => shape === "prompt" || shape === "reply" || shape === "acts";
