import { readsAs } from "../../ask/ask";

// What the Brief tab says in words, and what it works out of them.

export const briefKeys: [string, string][] = [
  ["j / k", "On Brief: next / previous change"],
  ["a / d", "On Brief: accept / decline change; with text selected, a asks the planner"],
  ["/", "On Brief: ask the planner"],
  ["p", "On Brief: show or hide planner"],
  ["mod+⏎", "On Brief: confirm, accept or save (in the editor too)"],
];

// quoted is a passage as a message quotes it, over what's said of it.
export const quoted = (passage: string, body: string) =>
  `${passage
    .split("\n")
    .map((l) => `> ${l}`)
    .join("\n")}\n\n${body}`;

// yourWords is a turn of yours taken apart again: the passage it quotes
// (as it reads), if it quotes one, and what you said of it.
export function yourWords(body: string): { passage?: string; said: string } {
  const quote = /^((?:> .*\n)+)\n([\s\S]*)$/.exec(body);
  if (!quote) return { said: body };
  return { passage: readsAs(quote[1].replace(/^> /gm, "")), said: quote[2] };
}

// suggestions is what's offered to ask the planner: about a passage in
// hand, or of the brief as it's being written, or of a mission under way.
// With no planner to ask and no passage, nothing.
export function suggestions(o: { held: boolean; planner: boolean; briefing: boolean }): string[] {
  if (o.held) return ["Why does it say this?", "Make this clearer", "Is this needed?"];
  if (!o.planner) return [];
  return o.briefing ? ["What's still open?", "Can Z be checked as it's written?", "What would you leave out?"] : ["Is the plan still right?", "What's changed since it started?"];
}

// What's proposed for the documents: a draft of the brief or its plan
// while it's being briefed, an edit or a revision once it's under way.
interface Proposed {
  briefing: boolean;
  kind: string;
  // Which draft of the brief it is.
  draft: number;
  // It comes with a plan.
  plan?: boolean;
  // The brief isn't written yet.
  unwritten?: boolean;
}

// proposedName is what a proposal is, in the bar: a couple of words.
export function proposedName(p: Proposed): string {
  if (p.briefing) return p.kind === "plan" ? "Plan" : `Draft ${Math.max(p.draft, 1)}`;
  return p.kind === "plan" ? "Plan revision" : "Brief edit";
}

// proposedTitle is the same, said in full.
export function proposedTitle(p: Proposed): string {
  if (p.briefing) return p.kind === "plan" ? "Proposed plan" : `Draft ${Math.max(p.draft, 1)} of the brief${p.plan ? ", with its plan" : ""}`;
  return p.kind === "plan" ? "Proposed plan revision" : "Proposed brief edit";
}

// acceptLabel is what taking a proposal is called: briefing, a brief (or
// a plan for one that's written) confirmed starts the mission.
export function acceptLabel(p: Pick<Proposed, "briefing" | "kind" | "unwritten">): string {
  if (p.briefing) return p.kind === "plan" && p.unwritten ? "Accept the plan" : "Confirm and start";
  return p.kind === "plan" ? "Accept the plan" : "Accept the edit";
}

// unplaced says of changes that couldn't be marked where they are.
export const unplaced = (n: number) => (n === 1 ? "1 change not shown in place: included as proposed." : `${n} changes not shown in place: included as proposed.`);
