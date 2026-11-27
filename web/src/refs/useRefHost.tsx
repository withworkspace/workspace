import { type ReactNode, useEffect, useMemo, useRef } from "react";
import { useNavigate } from "react-router";
import type { RefHost, RefMark } from "../editor/refs";
import { useHoverDelay } from "../ui/useHoverDelay";
import { useWorkspaceIfRead } from "../workspace/store";
import { useRefs } from "./known";
import { RefCard } from "./RefCard";
import { findRefs, isOutside, type Known, type PageRef, parseAddress, refHref, refKey, refName, refOf } from "./refs";
import { lookOf } from "./views";

export interface RefHosting {
  // What a ProseMirror view's refs plugin asks (editor/refs.ts), or none
  // outside the pages, where nothing's named.
  host?: RefHost;
  // The card of the ref the pointer's on, to draw beside the words.
  card: ReactNode;
  // What there is to name, as the host finds it.
  known?: Known;
  // Hides the card at once (what it's on has moved).
  clear: () => void;
}

// useRefHost is what's written somewhere, finding what it names as it's
// written: a page's document, a box a turn or a comment is written in. What
// there is to name is the web UI's (refs/known.tsx), less what except
// leaves out (a page isn't a ref in itself). A ref is drawn as its kind's
// Look says (refs/views.tsx), with its card a moment after the pointer's
// on it, and ⌘-click goes there. text is what's written: the paths it
// names are looked for on the repos' default branches.
export function useRefHost({ text, except, enabled = true }: { text?: string; except?: (known: Known) => Known; enabled?: boolean } = {}): RefHosting {
  const shared = useRefs();
  const workspace = useWorkspaceIfRead();
  const navigate = useNavigate();
  const on = enabled && !!shared && !!workspace;
  useEffect(() => {
    if (on && text) shared.want(text);
  }, [on, shared, text]);
  // except is the caller's to keep (useCallback): a new one finds again.
  const known = useMemo(() => (shared ? (except?.(shared.known) ?? shared.known) : undefined), [shared, except]);
  // The latest of each, for the host's callbacks: it's made once for what
  // there is to name.
  const now = useRef({ known, workspace, repos: shared?.repos ?? new Map() });
  now.current = { known, workspace, repos: shared?.repos ?? new Map() };
  const hovering = useHoverDelay<{ key: string; el: HTMLElement }>(160);
  const host = useMemo<RefHost | undefined>(() => {
    if (!on) return undefined;
    const markOf = (ref: PageRef, from: number, to: number): RefMark => {
      const { workspace, repos, known } = now.current;
      const look = lookOf(ref, { workspace: workspace!, repos, things: known?.things });
      return { from, to, key: refKey(ref), kind: ref.kind, shape: look.shape, accent: look.accent, text: look.text, label: look.after?.label, tone: look.after?.tone };
    };
    return {
      find: (text) => (now.current.known ? findRefs(text, now.current.known).map((f) => markOf(f.ref, f.from, f.to)) : []),
      // A link to a ref: by its address.
      at(href, words) {
        const ref = now.current.known && parseAddress(href, now.current.known);
        return ref ? { ...markOf(ref, 0, 0), words: words !== refName(ref) } : undefined;
      },
      // A moment before it shows, so passing over doesn't flash; and it
      // stays while the pointer's on the card.
      over: (key, el) => hovering.over({ key, el }),
      out: hovering.out,
      follow(key) {
        const r = refOf(key);
        if (!r) return;
        // A plugin's thing with nowhere to open in ws goes to its address
        // outside, in a tab of its own.
        const href = refHref(r, now.current.known);
        if (isOutside(href)) window.open(href, "_blank", "noopener,noreferrer");
        else navigate(href);
      },
    };
    // A new host for what there is to name: the view finds them again.
  }, [on, known, navigate, hovering.over, hovering.out]);
  const hover = hovering.shown;
  const hovered = hover && refOf(hover.key);
  const card = hover && hovered && hover.el.isConnected ? <RefCard r={hovered} el={hover.el} onEnter={hovering.stay} onLeave={hovering.out} /> : null;
  return { host, card, known, clear: hovering.clear };
}
