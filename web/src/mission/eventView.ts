import type { MissionEvent } from "../api";
import { exitWords } from "../ui/statusOf";

// Who did something in the log.
export type ActorKind = "you" | "crew" | "orchestrator" | "agent" | "ws";

export interface EventView {
  actor: ActorKind;
  who: string;
  crew?: string;
  text: string; // the rest of the sentence
  title?: string; // a tooltip: the words replaced
}

// viewEvent splits an event's sentence into who and what they did, and
// says crew ending in words ("its terminal closed", not "exit 129").
export function viewEvent(e: MissionEvent): EventView {
  let t = e.text.trim();
  let title: string | undefined;
  // (One logged before ws said how it ended: stopped, or by itself.)
  t = t.replace(/ended \((stopped: )?(exit \d+)\)/, (_, stopped: string | undefined, code: string) => {
    title = code;
    return stopped ? "was stopped" : `ended: ${exitWords(code)}`;
  });
  t = t.replace(/^note: /, "noted ");
  // A message to crew (to resume with, or typed in): the crew it names is
  // whom it's for; by says who said it. One logged before ws said was the
  // orchestrator's (you were never logged).
  if (e.kind === "resume_with" || e.kind === "told") {
    const by = e.to ? e.by : undefined;
    if (by === "you" || (!by && e.kind === "told")) return { actor: "you", who: "You", text: t, title };
    if (by === "ws") return { actor: "ws", who: "ws", text: t, title };
    return { actor: "orchestrator", who: "Orchestrator", text: t, title };
  }
  if (e.by === "orchestrator" || /^orchestrator( failed)?: /.test(t)) {
    const failed = /^orchestrator failed: /.test(t);
    return { actor: "orchestrator", who: "Orchestrator", text: (failed ? "failed: " : "") + t.replace(/^orchestrator( failed)?: /, ""), title };
  }
  const crew = e.by && /^[a-z]+\d+$/.test(e.by) ? e.by : /^(c\d+) /.exec(t)?.[1];
  if (crew) return { actor: "crew", who: crew, crew, text: t.startsWith(crew + " ") ? t.slice(crew.length + 1) : t, title };
  if (/^you /.test(t)) return { actor: "you", who: "You", text: t.slice(4), title };
  if (/^agent /.test(t)) return { actor: "agent", who: "Agent", text: t.slice(6), title };
  return { actor: "ws", who: "", text: t, title };
}
