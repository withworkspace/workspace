import { useCallback, useEffect } from "react";
import { useNavigate } from "react-router";
import { useChromeStore } from "../chrome/store";
import { codeSearchKey, isCodeSearchKey, leaderWithin, pageBinds } from "../keys";
import { scopePrefix } from "./find";

// useGlobalKeys: ⌘K finds anything, ⌘P a file, ⌘⇧F text in the files, ?
// shows the keys, and g then m, c, p,
// a, r, t, f or o goes to Missions, Conversations, Pages, Artifacts,
// Reviews, Team, Files or the Orchestrator. They're read before a page's own keys.
export function useGlobalKeys() {
  const navigate = useNavigate();
  const openPalette = useChromeStore((c) => c.openFind);
  const openHelp = useCallback(() => useChromeStore.getState().setHelp(true), []);
  useEffect(() => {
    let g = 0;
    const typing = (t: EventTarget | null) => t instanceof HTMLElement && (t.isContentEditable || ["INPUT", "TEXTAREA", "SELECT"].includes(t.tagName));
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && !e.altKey && !e.shiftKey && e.key.toLowerCase() === "k") {
        // In a document being edited, ⌘K with words selected is a link, as
        // in any editor. With nothing selected, it's Find.
        if (e.target instanceof HTMLElement && e.target.closest(".pm") && !window.getSelection()?.isCollapsed) return;
        // With Find open, ⌘K is Find's own: it acts on what the cursor's
        // on there (shell/Palette/Palette.tsx).
        if (useChromeStore.getState().find !== undefined) return;
        e.preventDefault();
        e.stopPropagation();
        openPalette();
        return;
      }
      if (isCodeSearchKey(e)) {
        // ⌘⇧F is Find, for what the files say: except where a page has a
        // search box of its own (a capsule's files), which is nearer.
        e.preventDefault();
        const does = codeSearchKey(!!document.querySelector("[data-overlay]"));
        if (does.to === "nothing") return; // something's open: the key's its, or nobody's
        e.stopPropagation();
        if (does.to === "own") does.open();
        else openPalette(scopePrefix.code);
        return;
      }
      if ((e.metaKey || e.ctrlKey) && !e.altKey && !e.shiftKey && e.key.toLowerCase() === "p") {
        // ⌘P is Find, for files: except where a page has a file finder of
        // its own (a capsule's files, a review's), which is nearer.
        if (ownsFileFinder(window.location.pathname)) return;
        e.preventDefault();
        if (document.querySelector("[data-overlay]")) return; // something's open: not the browser's print, either
        e.stopPropagation();
        openPalette("/");
        return;
      }
      if (typing(e.target) || e.metaKey || e.ctrlKey || e.altKey || document.querySelector("[data-overlay]")) return;
      if (e.key === "?") {
        e.preventDefault();
        e.stopPropagation();
        openHelp();
        return;
      }
      // g then a key goes somewhere, unless the page uses that key itself
      // (g, first, then o, open it, on a list): the page's keys win.
      const to = { m: "/", c: "/conversations", p: "/documents", d: "/documents", a: "/artifacts", r: "/reviews", t: "/team", f: "/files", o: "/orchestrator" }[e.key];
      if (to && Date.now() - g < leaderWithin && !pageBinds(e.key)) {
        e.preventDefault();
        e.stopPropagation();
        g = 0;
        navigate(to);
        return;
      }
      g = e.key === "g" ? Date.now() : 0;
    };
    window.addEventListener("keydown", onKey, true);
    return () => window.removeEventListener("keydown", onKey, true);
  }, [navigate, openPalette, openHelp]);
}

// ownsFileFinder is whether the page at path finds files itself on ⌘P: a
// capsule's or a repo's files.
const ownsFileFinder = (path: string) => /^\/files\/[^/]+/.test(path) || path.includes("/files/");
