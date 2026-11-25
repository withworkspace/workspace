import type { CellSource, PageCell, TalkCall, TalkTurn } from "../api";
import type { CellState } from "../editor/cells";
import { clip } from "../ui/text";

// A cell, as its conversation says it stands.

// versions are what a cell's agent has written, oldest first.
export function versions(cell?: PageCell): TalkTurn[] {
  return cell?.turns.filter((t) => t.by === "agent" && !t.answer) ?? [];
}

// askingNow is whether what a cell's agent has last been asked is a
// question, to be answered beside the page.
export const askingNow = (cell: PageCell) => !![...cell.turns].reverse().find((t) => t.by === "you")?.ask;

const shorten = (s: string, root: string) => s.split(root + "/").join("");

// doing is what a step is, in a few words: "Reading api/routes.go".
export function doing(c: TalkCall, root: string): string {
  const input = shorten(c.input, root).replace(/^repos\//, "");
  const tail = input.length > 48 ? "…" + input.slice(-47) : input;
  if (/^(read|view|cat)/i.test(c.tool)) return `Reading ${tail}`;
  if (/^(grep|glob|search|find|ls)/i.test(c.tool)) return `Searching for ${tail}`;
  return `Running ${clip(input, 48)}`;
}

export function cellState(cell: PageCell, root: string): CellState {
  const last = cell.turns.at(-1);
  // A block of an outline its agent hasn't come to yet.
  if (cell.queued) return { status: "starting", note: "To be written" };
  if (cell.answering) {
    const step = cell.run?.calls.at(-1);
    return { status: "filling", note: step ? doing(step, root) : askingNow(cell) ? "Answering…" : versions(cell).length ? "Writing it again…" : "Starting…" };
  }
  if (last?.by === "ws") return { status: "failed", note: askingNow(cell) ? "Couldn't answer this" : "Couldn't fill this" };
  if (last?.by === "you") return { status: "starting", note: "Not started" };
  // A question, answered: it holds nothing, and says there's an answer.
  if (!versions(cell).length) return { status: "filled", note: "Answered" };
  if (last?.answer) return { status: "filled", note: "With an answer" };
  const moved = changedSources(cell);
  if (moved.length) return { status: "stale", note: `${sourcesChanged(moved)} since` };
  if (cell.moved) return { status: "stale", note: `${besideChanged(cell)} since` };
  // What its agent had to say of it is beside it: its head says there's
  // something.
  return { status: "filled", note: versions(cell).at(-1)?.notes ? "With a note from its agent" : undefined };
}

// besideChanged says what a cell was written beside that's changed.
export const besideChanged = (cell: PageCell) => (cell.moved === "page" ? "The page has changed" : "A cell above has changed");

// outOfDate is why a cell's out of date, in a sentence's start, or "".
export function outOfDate(cell: PageCell): string {
  const moved = changedSources(cell);
  if (moved.length) return sourcesChanged(moved);
  return cell.moved ? besideChanged(cell) : "";
}

// changedSources are the files a cell was written from that aren't as
// they were.
export const changedSources = (cell: PageCell) => (cell.sources ?? []).filter((s) => s.state);

// sourceName is a source as it's said: its repo and file (demo/main.go),
// or in a capsule, with the capsule.
export function sourceName(s: CellSource): string {
  if (!s.repo || !s.file) return s.path;
  return s.worktree && s.worktree !== ".ground" ? `${s.repo}/${s.worktree}: ${s.file}` : `${s.repo}/${s.file}`;
}

export const baseName = (s: CellSource) => (s.file ?? s.path).split("/").pop() ?? s.path;

// sourcesChanged says what's changed, in a few words: "main.go has
// changed", "3 files have changed".
export function sourcesChanged(moved: CellSource[]): string {
  if (moved.length > 1) return `${moved.length} files have changed`;
  return `${baseName(moved[0])} ${moved[0].state === "gone" ? "has gone" : "has changed"}`;
}

// sourceHref is where a source can be read: the Files page, for one in a
// repo.
export function sourceHref(s: CellSource): string | undefined {
  if (!s.repo || !s.file || s.state === "gone") return undefined;
  const tree = s.worktree && s.worktree !== ".ground" ? `/${encodeURIComponent(s.worktree)}` : "";
  return `/files/${encodeURIComponent(s.repo)}${tree}?${new URLSearchParams({ path: s.file })}`;
}

// liveCalls are a run's steps as they go: the last, with no result yet,
// still going.
export const liveCalls = (run?: { calls: TalkCall[] }) => (run?.calls ?? []).map((c, i, list) => (i === list.length - 1 && !c.result ? { ...c, pending: true } : c));
