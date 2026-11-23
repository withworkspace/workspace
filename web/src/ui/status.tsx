import type { Crew, Headline, MissionState } from "../api";
import { Icon, type IconName } from "./Icon";
import { capital } from "./text";

// Status is one look for where something stands, used wherever a status
// shows: an icon and a word, coloured by what it asks of you, not by hue.
//
//   attention  it needs you
//   progress   an agent's on it
//   waiting    waiting on something outside (checks, a reviewer)
//   done       finished, or ready to be
//   danger     failed, or stopped to ask a permission
//   quiet      nothing's happening
export type Tone = "attention" | "progress" | "waiting" | "done" | "danger" | "quiet";

export interface StatusInfo {
  icon: IconName;
  tone: Tone;
  words: string;
  live?: boolean; // something's happening now: the icon pulses
}

// stripGlyph drops the glyph a headline's label starts with
// ("? asks", "↻ re-review"): the web shows an icon instead.
export const stripGlyph = (label: string) => label.replace(/^[^\p{L}\p{N}]+\s+/u, "");

const byKind: Record<string, Omit<StatusInfo, "words"> & { words?: string }> = {
  // your court
  "crew-asks": { icon: "question", tone: "attention", words: "Asks you" },
  permission: { icon: "lock", tone: "danger", words: "Needs permission" },
  flag: { icon: "question", tone: "attention", words: "Asks you" },
  "re-review": { icon: "rotate", tone: "attention", words: "Re-review" },
  "step-review": { icon: "eye", tone: "attention", words: "Review" },
  "to-do": { icon: "user", tone: "attention", words: "To do" },
  replies: { icon: "reply", tone: "attention", words: "Replies" },
  "brief-talk": { icon: "reply", tone: "attention" },
  queued: { icon: "play", tone: "attention", words: "Next crew" },
  resume: { icon: "rotate", tone: "attention", words: "Resume" },
  replied: { icon: "reply", tone: "attention", words: "Replied" },
  // a conversation whose agent has replied (a need's kind, not a headline's)
  "your-turn": { icon: "reply", tone: "attention", words: "Your turn" },
  // a pull request that asks for your review by name (a need's kind too)
  "your-review": { icon: "pr", tone: "attention", words: "Your review" },
  "checks-failing": { icon: "x-circle", tone: "danger", words: "Checks failing" },
  "changes-requested": { icon: "alert", tone: "attention", words: "Changes requested" },
  approve: { icon: "check-circle", tone: "attention", words: "Approve" },
  accept: { icon: "check-circle", tone: "attention", words: "Accept" },
  merge: { icon: "pr", tone: "attention", words: "Merge" },
  "wrap-up": { icon: "check-circle", tone: "done", words: "Wrap up" },
  stalled: { icon: "pause", tone: "quiet", words: "Idle" },
  // an agent's
  working: { icon: "circle-dot", tone: "progress", words: "Working", live: true },
  "own-work": { icon: "clock", tone: "progress", words: "Background work" },
  answering: { icon: "spinner", tone: "progress", words: "Answering", live: true },
  briefing: { icon: "pencil", tone: "progress", words: "Briefing", live: true },
  launching: { icon: "spinner", tone: "progress", words: "Starting", live: true },
  deciding: { icon: "sparkle", tone: "progress", words: "Deciding", live: true },
  "moving-on": { icon: "spinner", tone: "progress", words: "Moving on", live: true },
  "writing-pr": { icon: "pencil", tone: "progress", words: "Writing PR", live: true },
  // outside
  "checks-running": { icon: "spinner", tone: "waiting", words: "Checks running", live: true },
  "awaiting-review": { icon: "pr", tone: "waiting", words: "Awaiting review" },
  "waiting-on": { icon: "clock", tone: "waiting", words: "Waiting" },
  // up next and past
  landed: { icon: "check-circle", tone: "done", words: "Landed" },
  ended: { icon: "check-circle", tone: "done", words: "Ended" },
  "to-brief": { icon: "pencil", tone: "quiet", words: "To brief" },
  ready: { icon: "circle", tone: "quiet", words: "Ready" },
  parked: { icon: "pause", tone: "quiet", words: "Parked" },
  archived: { icon: "archive", tone: "quiet", words: "Archived" },
};

// headlineStatus is where a mission stands, from its headline.
export function headlineStatus(h: Headline): StatusInfo {
  const known = byKind[h.kind];
  const words = capital(stripGlyph(h.label)) || known?.words || "";
  if (known) return { ...known, words };
  const tone: Tone = h.court === "you" ? "attention" : h.court === "agent" ? "progress" : h.court === "outside" ? "waiting" : "quiet";
  return { icon: tone === "attention" ? "alert" : "circle", tone, words };
}

// crewStatus is where crew stand. Starting is ws's mark on them, the one
// the mission's headline reads, so the two agree: crew are starting while
// it stands, whatever they last did (crew resumed did it in the session
// before), and not for having reported nothing. Crew who've reported no
// activity and aren't starting (a harness with no hooks) are taken as
// working, as the headline takes them.
export function crewStatus(c: Pick<Crew, "ended" | "starting" | "activity" | "status" | "waitingOn" | "again">): StatusInfo {
  if (c.starting && !c.ended) return { icon: "spinner", tone: "progress", words: "Starting", live: true };
  // Sent back after it reported done: what it's doing now is what counts.
  if (c.status === "done" && c.again && !c.ended) return { icon: "circle-dot", tone: "progress", words: "Working again", live: true };
  if (c.status === "done") return { icon: "check-circle", tone: "done", words: c.ended ? "Done" : "Reported done" };
  if (c.status === "asking") return { icon: "question", tone: "attention", words: "Asks you" };
  if (c.ended) return { icon: "circle", tone: "quiet", words: "Ended" };
  switch (c.activity) {
    case "working":
      return { icon: "circle-dot", tone: "progress", words: "Working", live: true };
    case "waiting":
      if (c.waitingOn) return { icon: "clock", tone: "progress", words: "Waiting on its work" };
      return { icon: "circle-half", tone: "attention", words: "Waiting for you" };
    case "permission":
      return { icon: "lock", tone: "danger", words: "Needs permission" };
    case "question":
      return { icon: "question", tone: "attention", words: "Asks you" };
  }
  return { icon: "circle-dot", tone: "progress", words: "Working", live: true };
}

const stateInfo: Record<MissionState, StatusInfo> = {
  briefing: { icon: "pencil", tone: "progress", words: "Briefing" },
  draft: { icon: "circle-dashed", tone: "quiet", words: "Draft" },
  active: { icon: "circle-half", tone: "progress", words: "Active" },
  review: { icon: "eye", tone: "waiting", words: "In review" },
  landed: { icon: "check-circle", tone: "done", words: "Landed" },
  answered: { icon: "check-circle", tone: "done", words: "Answered" },
  "handed-on": { icon: "arrow-right", tone: "quiet", words: "Handed on" },
  dropped: { icon: "x-circle", tone: "quiet", words: "Dropped" },
  parked: { icon: "pause", tone: "quiet", words: "Parked" },
  archived: { icon: "archive", tone: "quiet", words: "Archived" },
};

export const missionStateStatus = (s: MissionState): StatusInfo => stateInfo[s] ?? { icon: "circle", tone: "quiet", words: s };

// Status draws one: an icon, and its words unless only the icon's wanted
// (the words are then its tooltip).
export function Status({ s, iconOnly, title, size = 14, className }: { s: StatusInfo; iconOnly?: boolean; title?: string; size?: number; className?: string }) {
  return (
    <span className={`status tone-${s.tone}${s.live ? " live" : ""}${className ? ` ${className}` : ""}`} title={title ?? (iconOnly ? s.words : undefined)}>
      <Icon name={s.icon} size={size} />
      {iconOnly ? <span className="sr-only">{s.words}</span> : <span className="status-words">{s.words}</span>}
    </span>
  );
}

// exitWords turns how crew ended into words: "exit 129" is its terminal
// closing (a hang-up), 130 an interrupt, 137 killed, 143 asked to stop.
export function exitWords(exit?: string): string | undefined {
  if (!exit) return undefined;
  const m = /exit (\d+)/.exec(exit);
  if (!m) return exit;
  const code = Number(m[1]);
  const words: Record<number, string> = {
    0: "finished",
    1: "stopped with an error",
    129: "its terminal closed",
    130: "interrupted",
    137: "killed",
    143: "stopped",
  };
  return words[code] ?? `stopped (exit ${code})`;
}
