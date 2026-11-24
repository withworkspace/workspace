import { create } from "zustand";

// What a tab beside is called, where only its pane knows: a document's
// title is in its text, a pull request's in what GitHub said of it. The
// pane says it once it's read; until then, and for a tab behind the one
// in front that hasn't been read here, the tab is called by its address
// (Beside.tsx). They're this page load's: a name is read again with its
// thing.

interface TabNames {
  names: Record<string, string>;
  // say is a pane saying what its thing is called, by its tab's address.
  say: (to: string, name: string) => void;
}

export const useTabNames = create<TabNames>()((set, get) => ({
  names: {},
  say: (to, name) => {
    if (name && get().names[to] !== name) set((s) => ({ names: { ...s.names, [to]: name } }));
  },
}));

// useTabName is what a tab's pane said its thing is called, if it has.
export const useTabName = (to: string): string | undefined => useTabNames((s) => s.names[to]);
