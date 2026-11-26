import { useEffect, useRef } from "react";
import { useLocation } from "react-router";
import { useKeys } from "../keys";
import { toast } from "../ui/toast";
import { besideFront, besideShown } from "./chromeState";
import { peekAt } from "./panes";
import { useChromeStore } from "./store";

// What z peeks at: the link the keys' cursor is on while you're moving by
// keys (a list's row says data-cursor: ui/Rows.tsx), else the link under
// the pointer. What isn't a link but goes somewhere says where with
// data-peek. Nothing, where neither's on something with a pane.
function peeked(pointer: { x: number; y: number } | undefined): string | undefined {
  const at = (el: Element | null | undefined) => {
    const href = el?.getAttribute("data-peek") ?? (el instanceof HTMLAnchorElement ? el.href : undefined);
    return href ? peekAt(href, window.location.origin) : undefined;
  };
  if (document.body.classList.contains("keyboarding")) {
    const row = document.querySelector("[data-cursor]");
    const to = at(row?.matches("a[href], [data-peek]") ? row : row?.querySelector("a[href], [data-peek]"));
    if (to) return to;
  }
  return pointer ? at(document.elementFromPoint(pointer.x, pointer.y)?.closest("a[href], [data-peek]")) : undefined;
}

// usePeekKey is z, anywhere: a peek at what the pointer's on (or the row
// the keys' cursor is on), beside the page. It opens in the place of what
// was peeked at before; z on the same thing, or on nothing, closes it. A
// peek isn't kept beside its own page: going to what it shows closes it.
export function usePeekKey() {
  // Where the pointer is, as it was last heard: a key says nothing of it.
  const pointer = useRef<{ x: number; y: number }>(undefined);
  useEffect(() => {
    const moved = (e: MouseEvent) => (pointer.current = { x: e.clientX, y: e.clientY });
    const left = () => (pointer.current = undefined);
    window.addEventListener("mousemove", moved, { passive: true });
    document.documentElement.addEventListener("mouseleave", left);
    return () => {
      window.removeEventListener("mousemove", moved);
      document.documentElement.removeEventListener("mouseleave", left);
    };
  }, []);

  useKeys({
    z: () => {
      const { chrome, here, peek, close } = useChromeStore.getState();
      const to = peeked(pointer.current);
      const open = chrome.beside.peek;
      const front = open !== undefined && besideShown(chrome, here) && besideFront(chrome, here) === open;
      if (!to) {
        // On nothing: the peek that's showing closes, as it would on its
        // own thing.
        if (front) close(open);
        else toast("Point at a mission, document, pull request or conversation, then press z to peek.");
        return;
      }
      // What the page itself is of has nothing more to show beside it.
      if (to === peekAt(window.location.href, window.location.origin) && to !== open) return void toast("Already on this page.");
      peek(to);
    },
  });

  // Gone to what the peek shows: it's the page now.
  const { pathname, search } = useLocation();
  const open = useChromeStore((s) => s.chrome.beside.peek);
  useEffect(() => {
    if (open !== undefined && open === peekAt(pathname + search, window.location.origin)) useChromeStore.getState().close(open);
  }, [pathname, search, open]);
}
