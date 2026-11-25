import type { Transcript, TranscriptEntry } from "../../api";
import type { Call } from "../../ui/Calls";

// A turn of the transcript: what crew was sent, what it said, or the steps
// between. Each is keyed by where it is in the whole session, so a fold
// stays open as the window of the latest entries slides.
export type Group =
  | { kind: "brief"; at: number; text: string; time?: string }
  | { kind: "user"; at: number; text: string; time?: string }
  | { kind: "notice"; at: number; text: string }
  | { kind: "agent"; at: number; text: string; time?: string }
  | { kind: "calls"; at: number; calls: TranscriptEntry[] };

// transcriptGroups is a transcript as its turns: a run of steps is one,
// and what crew's sent is ws's brief to it (its opening prompt), a
// harness's notice, or you.
export function transcriptGroups(t: Transcript): Group[] {
  const out: Group[] = [];
  const base = t.earlier ?? 0;
  t.entries.forEach((e, i) => {
    const at = base + i;
    const last = out[out.length - 1];
    if (e.by === "call") {
      if (last?.kind === "calls") last.calls.push(e);
      else out.push({ kind: "calls", at, calls: [e] });
      return;
    }
    const text = e.text ?? "";
    if (e.by === "agent") return out.push({ kind: "agent", at, text, time: e.at });
    if (/^You are crew on the mission|^You are the briefing agent|^You are an? .{0,40}(agent|crew)\b/.test(text) && at < 4) return out.push({ kind: "brief", at, text, time: e.at });
    if (/^\[Request interrupted|^\[.*\]$|^<command-|^Caveat:/.test(text.trim())) return out.push({ kind: "notice", at, text: text.trim().replace(/^\[|\]$/g, "") });
    out.push({ kind: "user", at, text, time: e.at });
  });
  return out;
}

// toCall is a step of the transcript as the steps' part draws it.
export const toCall = (c: TranscriptEntry): Call => ({
  tool: c.tool ?? "tool",
  input: c.input ?? "",
  result: c.result,
  error: c.error,
  refused: c.refused,
  pending: c.pending,
  edit: c.edit,
  todos: c.todos,
  questions: c.questions,
});

// pendingCall is the step crew are on, or waiting for permission for: the
// last call with nothing back yet.
export const pendingCall = (t?: Transcript) => [...(t?.entries ?? [])].reverse().find((e) => e.by === "call" && e.pending);

// latestPlan is crew's latest plan: the todo list it last wrote.
export const latestPlan = (t: Transcript) => [...t.entries].reverse().find((e) => e.todos?.length)?.todos;
