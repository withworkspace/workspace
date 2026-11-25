import { setPref, usePrefs } from "../../prefs";

// Panes: how many of the explorer's are shown while a file's open. 3 is
// the repos (the Files page's), the tree and the file; 2 drops the repos;
// 1 is the file alone. Yours, in this browser, for every explorer: one
// without the repos shows 2 for 3.
export type Panes = 1 | 2 | 3;
export const setPanes = (n: Panes) => setPref("filePanes", n);
export const usePanes = (): Panes => usePrefs((s) => s.filePanes);

export interface PaneWords {
  label: string;
  sub: string;
}

export type PaneChoice = PaneWords & { n: Panes };

export const paneWords: Record<Panes, PaneWords> = {
  3: { label: "3 panes", sub: "Repos, tree and file" },
  2: { label: "2 panes", sub: "Tree and file" },
  1: { label: "1 pane", sub: "The file alone" },
};

// On a tab other than Files, the panes are the repos beside it, or not.
export const otherWords: Partial<Record<Panes, PaneWords>> = {
  3: { label: "With the repos", sub: "The repos, beside this tab" },
  2: { label: "This tab alone", sub: "The repos folded away" },
};

// What the panes menu offers, and which it's on: on a tab other than
// Files (other), whether the repos are beside it.
export function paneChoices(shown: Panes, most: Panes, other: boolean): { now: PaneChoice; choices: PaneChoice[] } {
  const words = (n: Panes): PaneChoice => ({ n, ...(other ? (otherWords[n] ?? paneWords[n]) : paneWords[n]) });
  const choices = ([3, 2, 1] as Panes[]).filter((n) => n <= most && (!other || n > 1)).map(words);
  return { now: words(other && shown === 1 ? 2 : shown), choices };
}
