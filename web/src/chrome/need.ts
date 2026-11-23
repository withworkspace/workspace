import { useEffect, useRef, useState } from "react";
import type { Headline, Need } from "../api";
import { plain } from "../ui/inline";
import { headlineStatus, type StatusInfo } from "../ui/status";

// A need: something that can't move without you. The daemon works the
// list out from where each thing stands now and never keeps it, so it
// can't go stale: when the thing moves on, the need is gone
// (internal/ops/needs.go; workspace-records:docs/chrome-and-notifications.md).
// The workspace view carries it, and the browser only draws it: the
// sidebar's list, the counts beside its places, the tab's title and what
// the desktop is told are all the same list.
export type { Need };

// needStatus is how a need stands, as an icon and a word: a mission's as
// its headline's kind has it, a conversation's as your turn.
export function needStatus(n: Need): StatusInfo {
  return headlineStatus({
    kind: n.kind,
    label: n.label ?? "",
    court: "you",
  } as Headline);
}

// needSays is what a need asks of you, in plain words: its sentence with
// the marks of its writing taken out.
export const needSays = (n: Need): string => plain(n.text ?? "");

// toReview is whether a need is a pull request asking for your review:
// someone else's work, listed apart from what's yours (Needs).
export const toReview = (n: Need) => n.of === "pull";

// waitingOf is the needs as what tells the desktop and counts the places
// has them (shell/notify.ts's Waiting, with which place counts each). A
// conversation about a mission is counted with the missions: it's in the
// mission, not among Conversations. A pull request counts at neither:
// Reviews has a count of its own, of everything on its page that needs
// you.
export function waitingOf(needs: Need[]) {
  return needs.map((n) => ({
    kind: toReview(n) ? ("review" as const) : n.of === "mission" || n.in ? ("mission" as const) : ("talk" as const),
    key: n.key,
    title: n.title,
    body: needSays(n),
    path: n.to,
  }));
}

// dealtWithAt is whether a page is where a need is dealt with: its own
// page, or one under it (a mission's review, its crew).
export const dealtWithAt = (need: Need, path: string) =>
  path === need.to || path.startsWith(`${need.to}/`);

// arrived is the needs that are new: those here now that weren't before.
// Before anything was known (the page just opened) nothing has arrived:
// what was already waiting isn't news.
export function arrived(before: string[] | undefined, now: string[]): string[] {
  if (!before) return [];
  const had = new Set(before);
  return now.filter((k) => !had.has(k));
}

// useArrived is the needs that have just arrived, for as long as their
// rows say so: each lights up once, where it is, and that's all a new
// need gets. It's no notification and no card.
//
// needs is undefined until they're known (the workspace hasn't answered):
// what's there when it first does was already waiting, and isn't news.
export function useArrived(needs: Need[] | undefined, ms = 1800): Set<string> {
  const before = useRef<string[] | undefined>(undefined);
  const [fresh, setFresh] = useState<Set<string>>(() => new Set());
  const keys = needs?.map((n) => n.key);
  const joined = keys?.join("\n");
  useEffect(() => {
    if (!keys) return;
    const came = arrived(before.current, keys);
    before.current = keys;
    if (!came.length) return;
    setFresh((f) => new Set([...f, ...came]));
    const t = setTimeout(
      () => setFresh((f) => new Set([...f].filter((k) => !came.includes(k)))),
      ms,
    );
    return () => clearTimeout(t);
    // The keys, joined, are what it follows.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [joined, ms]);
  return fresh;
}
