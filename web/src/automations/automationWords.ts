import type { AutomationItem, AutomationTally } from "../api";
import type { IconName } from "../ui/Icon";
import type { Tone } from "../ui/tone";

// What the Automations page says of a run's state, and of how one's runs
// stand: the words, and the tone and icon its mark is drawn in.

export interface RunLook {
  words: string;
  tone: Tone;
  icon: IconName;
}

// runLook is a run's state as the page says it.
export function runLook(state: string): RunLook {
  switch (state) {
    case "ran":
      return { words: "ran", tone: "done", icon: "check-circle" };
    case "failed":
      return { words: "failed", tone: "danger", icon: "x-circle" };
    case "unsure":
      return { words: "stopped part way", tone: "attention", icon: "alert" };
    case "waiting":
      return { words: "waiting", tone: "waiting", icon: "clock" };
    case "held":
      return { words: "held back", tone: "quiet", icon: "pause" };
    case "running":
      return { words: "running", tone: "progress", icon: "spinner" };
  }
  return { words: state, tone: "quiet", icon: "circle" };
}

// tallyWords is how its runs stand, a few words each: "12 runs", "2
// failed", "1 waiting", "1 settling", "3 held back". Nothing for none.
export function tallyWords(t: AutomationTally): string[] {
  return [
    t.runs ? `${t.runs} ${t.runs === 1 ? "run" : "runs"}` : "",
    t.failed ? `${t.failed} failed` : "",
    t.waiting ? `${t.waiting} waiting` : "",
    t.settling ? `${t.settling} settling` : "",
    t.held ? `${t.held} held back` : "",
  ].filter(Boolean);
}

// mayWords is what it may do, said: "notify you", "start a conversation".
export function mayWords(may: string[]): string {
  if (may.length === 0) return "notify you of the event";
  return may.map((m) => (m === "notify" ? "notify you" : m === "converse" ? "start a conversation" : m)).join(", ");
}

// standing is the one thing to say first of an automation, if any: why
// its file can't be read, or what it last failed with.
export function standing(a: AutomationItem): { tone: Tone; words: string } | undefined {
  if (a.broken) return { tone: "danger", words: `Its script can't be read: ${a.broken}` };
  if (a.failed) return { tone: "danger", words: `It couldn't run: ${a.failed}` };
  return undefined;
}
