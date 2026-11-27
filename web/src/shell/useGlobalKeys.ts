import { useCallback } from "react";
import { useNavigate } from "react-router";
import { useChromeStore } from "../chrome/store";
import { codeSearchKey } from "../keys";
import { useCommands } from "../keymap/useCommands";
import { scopePrefix } from "./find";
import { everywhereKeys } from "./keys";
import { goes } from "./places";

// useGlobalKeys: ⌘K finds anything, ⌘P a file, ⌘⇧F text in the files, ?
// shows the keys, and g then m, c, d, a, r, t, f or o goes to Missions,
// Conversations, Documents, Artifacts, Reviews, Team, Files or the
// Orchestrator (each unless you've set another key: shell/keys.ts).
// They're read before a page's own keys, and before what has the focus.
export function useGlobalKeys() {
  const navigate = useNavigate();
  const openPalette = useChromeStore((c) => c.openFind);
  const openHelp = useCallback(() => useChromeStore.getState().setHelp(true), []);
  const overlay = () => !!document.querySelector("[data-overlay]");
  // Going somewhere: not while something's open over the page.
  const go = (to: string) => () => {
    if (overlay()) return false;
    navigate(to);
  };
  const g = everywhereKeys;
  const to = (c: (typeof g)[keyof typeof g]) => Object.keys(goes).find((path) => goes[path] === c) ?? "/";
  useCommands(
    g,
    {
      find: (e) => {
        // In a document being edited, ⌘K with words selected is a link, as
        // in any editor. With nothing selected, it's Find.
        if (e.target instanceof HTMLElement && e.target.closest(".pm") && !window.getSelection()?.isCollapsed) return false;
        // With Find open, ⌘K is Find's own: it acts on what the cursor's
        // on there (shell/Palette/Palette.tsx).
        if (useChromeStore.getState().find !== undefined) return false;
        openPalette();
      },
      findCode: (e) => {
        // ⌘⇧F is Find, for what the files say: except where a page has a
        // search box of its own (a capsule's files), which is nearer.
        const does = codeSearchKey(overlay());
        if (does.to === "nothing") {
          // Something's open: the key's its, or nobody's (not the browser's).
          e.preventDefault();
          return false;
        }
        if (does.to === "own") does.open();
        else openPalette(scopePrefix.code);
      },
      findFile: (e) => {
        // ⌘P is Find, for files: except where a page has a file finder of
        // its own (a capsule's files, a review's), which is nearer.
        if (ownsFileFinder(window.location.pathname)) return false;
        if (overlay()) {
          // Something's open: not the browser's print, either.
          e.preventDefault();
          return false;
        }
        openPalette("/");
      },
      help: () => {
        if (overlay()) return false;
        openHelp();
      },
      // g then a letter goes somewhere, on every page: the sidebar's
      // places say so. A page's key of the same letter is the page's
      // alone, not straight after g: on a list, g (the first) then o goes
      // to the Orchestrator, and ⏎ opens the first.
      goMissions: go(to(g.goMissions)),
      goConversations: go(to(g.goConversations)),
      goDocuments: go(to(g.goDocuments)),
      goArtifacts: go(to(g.goArtifacts)),
      goReviews: go(to(g.goReviews)),
      goTeam: go(to(g.goTeam)),
      goFiles: go(to(g.goFiles)),
      goOrchestrator: go(to(g.goOrchestrator)),
    },
    { first: true },
  );
}

// ownsFileFinder is whether the page at path finds files itself on ⌘P: a
// capsule's or a repo's files.
const ownsFileFinder = (path: string) => /^\/files\/[^/]+/.test(path) || path.includes("/files/");
