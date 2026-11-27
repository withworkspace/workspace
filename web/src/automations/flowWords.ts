import type { AutomationOutlineStep, AutomationStepCount } from "../api";
import type { IconName } from "../ui/Icon";

// What an automation's outline (AutomationFlow) says of each step: its
// icon and what it's called, and how it's gone over the runs counted on
// it (or a dry run's).

// stepLook is a step's icon and the word for what it does.
export function stepLook(kind: string): { icon: IconName; words: string } {
  switch (kind) {
    case "notify":
      return { icon: "bell", words: "Notify you" };
    case "converse":
      return { icon: "message", words: "Start a conversation" };
    case "wait":
      return { icon: "clock", words: "Wait for" };
    case "read":
      return { icon: "eye", words: "Read" };
    case "run":
      return { icon: "play", words: "Run" };
    case "output":
      return { icon: "file", words: "Write" };
    case "keep":
      return { icon: "archive", words: "Keep" };
    case "log":
      return { icon: "list", words: "Log" };
    case "if":
      return { icon: "split", words: "If" };
    case "for":
      return { icon: "rotate", words: "For each" };
    case "stop":
      return { icon: "stop", words: "Stop" };
  }
  return { icon: "code", words: "Code" };
}

// countsAt is the counts by where each step is: the builtins called there
// added together.
export function countsAt(steps: AutomationStepCount[] | undefined): Map<string, AutomationStepCount> {
  const out = new Map<string, AutomationStepCount>();
  for (const c of steps ?? []) {
    const had = out.get(c.at);
    if (!had) {
      out.set(c.at, { ...c });
      continue;
    }
    for (const k of ["done", "started", "waiting", "met", "timedOut"] as const) had[k] = (had[k] ?? 0) + (c[k] ?? 0);
  }
  return out;
}

// countWords is how a step's gone, a few words each: "12 sent", "3 met",
// "1 timed out", "1 waiting", "1 unsure". dry says it as what would have.
export function countWords(kind: string, c: AutomationStepCount | undefined, dry = false): string[] {
  if (!c) return [];
  const done = c.done ?? 0;
  const verb =
    kind === "notify"
      ? dry
        ? "would notify"
        : "sent"
      : kind === "converse"
        ? dry
          ? "would start"
          : "started"
        : kind === "run"
          ? dry
            ? "would run"
            : "asked for"
          : kind === "read"
            ? "read"
            : "done";
  const said = dry && (kind === "notify" || kind === "converse" || kind === "run") ? (n: number) => `${verb} ${n}×` : (n: number) => `${n} ${verb}`;
  return [
    done ? said(done) : "",
    c.met ? `${c.met} met` : "",
    c.timedOut ? `${c.timedOut} timed out` : "",
    c.waiting ? `${c.waiting} waiting` : "",
    c.started ? `${c.started} unsure` : "",
  ].filter(Boolean);
}

// failuresOn is how many runs failed at each step: where a failure is at
// a step's place, there; where it's at no step's, on the step on its line.
export function failuresOn(steps: AutomationOutlineStep[], failures: Record<string, number> | undefined): Map<string, number> {
  const out = new Map<string, number>();
  if (!failures) return out;
  const places = new Map<string, AutomationOutlineStep>();
  const byLine = new Map<number, AutomationOutlineStep>();
  const walk = (list: AutomationOutlineStep[]) => {
    for (const s of list) {
      places.set(s.at, s);
      if (!byLine.has(s.line)) byLine.set(s.line, s);
      for (const b of s.branches ?? []) walk(b.steps);
    }
  };
  walk(steps);
  for (const [at, n] of Object.entries(failures)) {
    const step = places.get(at) ?? byLine.get(Number(at.split(":")[0]));
    if (step) out.set(step.at, (out.get(step.at) ?? 0) + n);
  }
  return out;
}
