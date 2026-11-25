import { create } from "zustand";
import { createJSONStorage, persist } from "zustand/middleware";

// How you've left the web UI, in this browser: which panes are open, how
// wide you dragged a list, what you've put away. One store, kept as one
// entry (ws:prefs), so a page asks for what it wants by name and neither
// reads nor writes the browser's storage itself. With nowhere to keep
// them (storage off), they hold while the page is open.
//
// Not here, each for its reason: the theme (ui/theme.ts: public/theme.js
// reads its own key before the page is drawn), how the window's arranged
// and where you've been (chrome/store.ts, shell/recent.ts: kept for each
// workspace), and what's typed in a composer (ui/useDraft.ts: one
// each).

export interface Prefs {
  // Desktop notifications of what newly needs you, once the browser
  // allows them (shell/notify.ts).
  notify: boolean;
  // How many of the file explorer's panes are shown while a file's open:
  // 3 is the repos, the tree and the file; 2 drops the repos; 1 is the
  // file alone.
  filePanes: 1 | 2 | 3;
  // A mission's details, beside its Summary.
  missionSide: boolean;
  // A mission's Work tab: what's done, unfolded.
  workDone: boolean;
  // The brief's planner, beside the brief.
  briefPlanner: boolean;
  // A review's list of files: its width as dragged (none: as laid out),
  // and the diff side by side.
  reviewFilesWidth?: number;
  reviewSplit: boolean;
  // A pull request's review: its files in the guide's order, and the
  // passes whose findings are hidden, by name.
  guideOrder: boolean;
  passHide: Record<string, boolean>;
  // A page being edited: its map, and the passes whose findings are drawn
  // on the page, by name (one not said is drawn).
  pageMap: boolean;
  passNotes: Record<string, boolean>;
  // A space's list of documents: its width as dragged.
  spaceListWidth?: number;
  // What's come of a conversation, beside it.
  talkSide: boolean;
  // The folders folded in the list of artifacts found in the repos, by
  // repo and path.
  foundFolded: string[];
  // The harness's warnings you've dismissed (the latest 50), by how each
  // starts.
  dismissed: string[];
  // The key that sends what you've written in a box (ui/sendKey.ts): ⌘⏎,
  // where ⏎ is a new line, or ⏎, where ⇧⏎ is.
  sendKey: "mod+enter" | "enter";
  // Being told a thing has a key when you press it with the pointer
  // (ui/keyHint.ts). Off unless you turn it on.
  keyHints: boolean;
}

const defaults: Prefs = {
  notify: false,
  filePanes: 3,
  missionSide: true,
  workDone: false,
  briefPlanner: true,
  reviewSplit: false,
  guideOrder: true,
  passHide: {},
  pageMap: true,
  passNotes: {},
  talkSide: true,
  foundFolded: [],
  dismissed: [],
  sendKey: "mod+enter",
  keyHints: false,
};

// Where each was kept before they were one entry, and how it was written
// there: read once, the first time this ws is opened in a browser that
// has them, and removed. One that's only ever been in the one entry has
// none.
const before: { [K in keyof Prefs]?: [key: string, read: (said: string) => Prefs[K] | undefined] } = {
  notify: ["ws-notify", (s) => s === "on"],
  filePanes: ["ws-file-panes", (s) => (s === "1" ? 1 : s === "2" ? 2 : undefined)],
  missionSide: ["ws:mission-side", (s) => s !== "0"],
  workDone: ["ws:work-done", (s) => s === "1"],
  briefPlanner: ["ws:brief-planner", (s) => s !== "0"],
  reviewFilesWidth: ["ws:files-w", (s) => Number(s) || undefined],
  reviewSplit: ["ws:split", (s) => s === "1"],
  guideOrder: ["ws:guide-order", (s) => s !== "0"],
  passHide: ["ws:pass-hide", (s) => flags(s)],
  pageMap: ["ws:page-map", (s) => s !== "off"],
  passNotes: ["ws:pass-notes", (s) => flags(s)],
  spaceListWidth: ["ws:space-list-w", (s) => Number(s) || undefined],
  talkSide: ["ws:talk-side", (s) => s !== "off"],
  foundFolded: ["ws.artifacts.found.folded", (s) => strings(s)],
  dismissed: ["ws-dismissed", (s) => strings(s)],
};

function parsed(s: string): unknown {
  try {
    return JSON.parse(s);
  } catch {
    return undefined;
  }
}
const flags = (s: string): Record<string, boolean> | undefined => {
  const v = parsed(s);
  return v && typeof v === "object" && !Array.isArray(v) ? (v as Record<string, boolean>) : undefined;
};
const strings = (s: string): string[] | undefined => {
  const v = parsed(s);
  return Array.isArray(v) ? v.filter((x): x is string => typeof x === "string") : undefined;
};

// keptBefore is what the browser has under the old keys, as prefs.
export function keptBefore(storage: Pick<Storage, "getItem">): Partial<Prefs> {
  const got: Record<string, unknown> = {};
  for (const [name, [key, read]] of Object.entries(before)) {
    const said = storage.getItem(key);
    const v = said === null ? undefined : read(said);
    if (v !== undefined) got[name] = v;
  }
  return got as Partial<Prefs>;
}

const entry = "ws:prefs";

// What was kept before, where there's storage to have kept it, and
// whether any was.
function old(): Partial<Prefs> {
  try {
    return keptBefore(localStorage);
  } catch {
    return {};
  }
}
const was = old();

export const usePrefs = create<Prefs>()(persist(() => ({ ...defaults, ...was }), { name: entry, version: 1, storage: createJSONStorage(() => localStorage) }));

// The old keys go once what they said is in the one entry (which, if
// it's already there, has what's newer, and has been read over them).
if (Object.keys(was).length > 0) {
  try {
    usePrefs.setState({});
    if (localStorage.getItem(entry) !== null) for (const [key] of Object.values(before)) localStorage.removeItem(key);
  } catch {
    // Nowhere to keep them: they're as they were.
  }
}

// setPref sets one: to a value, or to what a function makes of how it was.
export function setPref<K extends keyof Prefs>(name: K, to: Prefs[K] | ((was: Prefs[K]) => Prefs[K])) {
  usePrefs.setState((s) => ({ [name]: typeof to === "function" ? to(s[name]) : to }) as Pick<Prefs, K>);
}

// usePref is one of them, and how it's set: as useState is, but yours,
// kept, and the same wherever it's read.
export function usePref<K extends keyof Prefs>(name: K): [Prefs[K], (to: Prefs[K] | ((was: Prefs[K]) => Prefs[K])) => void] {
  const value = usePrefs((s) => s[name]);
  return [value, (to) => setPref(name, to)];
}
