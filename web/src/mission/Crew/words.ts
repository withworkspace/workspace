import type { CrewPrompt, TranscriptEntry } from "../../api";

// What a crew's page says of what crew are doing and asking.

// noticeWords is a harness's notice as the transcript says it.
export const noticeWords = (text: string) => text.replace(/^Request interrupted by user.*$/, "Interrupted");

// workingOn is the step crew are on, in a line: what it's doing ("Running"
// a command, "Working on" anything else) and what to (the file it edits,
// or the start of what it was given).
export function workingOn(pending: TranscriptEntry): { verb: string; what?: string } {
  return { verb: pending.tool === "Bash" ? "Running" : "Working on", what: pending.edit?.path.split("/").pop() ?? pending.input?.slice(0, 80) };
}

// screenContext is what crew's screen says of what it asks leave for, but
// the harness's tips.
export const screenContext = (screen?: CrewPrompt) => screen?.context.filter((l) => !/^tip:/i.test(l));

// wantsTo is what crew want permission to do: edit or write a file, run a
// command, use a tool; read off its screen when its transcript hasn't the
// step yet ("make this edit to …"); or, knowing nothing, go on.
export function wantsTo(pending: TranscriptEntry | undefined, screen: CrewPrompt | undefined, about: string[] | undefined): string {
  if (pending) return pending.edit ? (pending.edit.old ? "edit" : "write") : pending.tool === "Bash" ? "run" : `use ${pending.tool}`;
  if (!about?.length) return "continue";
  return screen?.title.replace(/^.*?(Do you want to [^?]*)\?.*$/i, "$1").replace(/^Do you want to /i, "") || "continue";
}

// shortPath is a file's path by its folder and name.
export const shortPath = (path: string) => path.split("/").slice(-2).join("/");
