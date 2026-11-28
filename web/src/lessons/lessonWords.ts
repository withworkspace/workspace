import type { Lesson, LessonsInfo } from "../api";
import type { Tone } from "../ui/tone";

// What the Lessons page says of a lesson, and how it groups them: worked
// out of what ws answered (internal/lessons), for the parts to draw.

// recommendedAt is how many crew make a lesson a recommendation, and
// quietDays how long nobody running into one makes it quiet: as
// internal/lessons has them (Recommended, QuietAfter).
export const recommendedAt = 2;
export const quietDays = 30;

// fixWords is who or what a lesson's fix changes, in a word or two.
const fixWords: Record<string, string> = {
  ws: "Change Workspace",
  setup: "Capsule setup",
  repo: "Change the repo",
  docs: "Agent instructions",
  you: "Yours to do",
};

export function fixOf(fix: string): string {
  return fixWords[fix] ?? fixWords.docs;
}

// fixTone colours what takes the problem away apart from what only tells
// of it, and what's yours.
export function fixTone(fix: string): Tone {
  return fix === "you" ? "attention" : fix === "docs" ? "quiet" : "info";
}

// seenWords is who ran into it: "3 crew in 2 missions", "1 crew".
export function seenWords(l: Pick<Lesson, "crew" | "missions">): string {
  if (l.crew <= 1) return "1 crew";
  return `${l.crew} crew in ${l.missions} mission${l.missions === 1 ? "" : "s"}`;
}

export function isRecommended(l: Pick<Lesson, "crew">): boolean {
  return l.crew >= recommendedAt;
}

export function isQuiet(l: Pick<Lesson, "lastSeen">, now: number): boolean {
  return now - Date.parse(l.lastSeen) > quietDays * 24 * 60 * 60 * 1000;
}

export type LessonGroupId = "recommended" | "notes" | "quiet" | "settled";

export type LessonGroup = { id: LessonGroupId; title: string; lessons: Lesson[] };

// lessonGroups puts the open ones first, two crew or more apart from those
// one crew ran into, then those nobody's run into lately (fixed, maybe,
// without anyone saying so), then those fixed or dismissed. Within each,
// as ws sent them (the most crew, then the latest). Empty ones are left
// out.
export function lessonGroups(list: Lesson[], now: number): LessonGroup[] {
  const groups: LessonGroup[] = [
    { id: "recommended", title: "Recommended", lessons: [] },
    { id: "notes", title: "Seen once", lessons: [] },
    { id: "quiet", title: "Not seen lately", lessons: [] },
    { id: "settled", title: "Fixed and dismissed", lessons: [] },
  ];
  const at = (id: LessonGroupId) => groups.find((g) => g.id === id)!;
  for (const l of list) {
    if (l.state !== "open") at("settled").lessons.push(l);
    else if (isQuiet(l, now)) at("quiet").lessons.push(l);
    else if (isRecommended(l)) at("recommended").lessons.push(l);
    else at("notes").lessons.push(l);
  }
  return groups.filter((g) => g.lessons.length > 0);
}

// readingWords is how reading handoffs stands: "31 handoffs read · 4
// waiting", or that it's off.
export function readingWords(info: Pick<LessonsInfo, "on" | "read" | "failed" | "waiting">): string {
  if (!info.on) return info.read > 0 ? `Reading handoffs is off · ${info.read} read` : "Reading handoffs is off";
  const parts = [`${info.read} handoff${info.read === 1 ? "" : "s"} read`];
  if (info.failed > 0) parts.push(`${info.failed} unreadable`);
  if (info.waiting > 0) parts.push(`${info.waiting} waiting`);
  return parts.join(" · ");
}

// lessonAnchor is a lesson's id on the page: a notification opens
// /lessons#lesson-3.
export const lessonAnchor = (id: number) => `lesson-${id}`;

export function lessonOfAnchor(hash: string): number | undefined {
  const m = /^#?lesson-(\d+)$/.exec(hash);
  return m ? Number(m[1]) : undefined;
}
