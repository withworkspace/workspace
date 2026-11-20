import type { V1 } from "@withworkspace/ui-lib/protocol";
import type { Anchor } from "../ui/hover";

// A ref's card, for a ref in a plugin's page. The page can't draw one: a
// card says what ws knows of the thing (a file's lines, where a mission
// stands), and a frame can't draw past its own edge. So the page says
// which ref the pointer's on and where its words are in the page
// (ws:hover), and ws draws the card it draws in its own pages, by those
// words, over the frame. Whoever's looking can't tell the ref was in a
// frame: the card waits as long before it shows, stays as long after the
// pointer goes, and can be moved onto.

// Over is a ref in the frame's page, by ws's own word for it, and where
// its words are there.
export interface Over {
  ref: string;
  box: V1.Box;
}

// How long the pointer's on a ref before its card shows, and how long the
// card stays once the pointer's gone, so it can be moved onto: as for a
// ref in ws's own pages (refs/RefTag.tsx).
export const SHOW_AFTER = 350;
export const HIDE_AFTER = 160;

const same = (a: Over, b: Over) => a.ref === b.ref && a.box.x === b.box.x && a.box.y === b.box.y && a.box.width === b.box.width && a.box.height === b.box.height;

// FrameCard is the card over a plugin's frame: which ref's is shown, as
// the page says where the pointer is and ws says when it's on the card.
export interface FrameCard {
  // The ref whose card is shown, if one is.
  shown(): Over | undefined;
  // hear runs fn when what's shown changes, and returns what stops it.
  hear(fn: () => void): () => void;
  // The page said the pointer's on this ref now, or on none.
  over(now: Over | undefined): void;
  // The pointer's on the card: it stays for as long as it is.
  stay(): void;
  // The pointer left the card.
  leave(): void;
  // Nothing's shown, at once: the page went, or ws moved it.
  clear(): void;
}

// frameCard keeps one frame's card. A card shows a moment after the
// pointer comes to its ref, and goes a moment after the pointer leaves
// both the ref and the card. On the way from one ref to another, the
// first's card goes as it would have, and the second's comes as it would
// have: each as if it were alone.
export function frameCard(): FrameCard {
  let shown: Over | undefined;
  let onCard = false;
  let show: ReturnType<typeof setTimeout> | undefined;
  let hide: ReturnType<typeof setTimeout> | undefined;
  const heard = new Set<() => void>();

  const stopHiding = () => {
    clearTimeout(hide);
    hide = undefined;
  };
  const set = (next: Over | undefined) => {
    stopHiding();
    // A card that's new, or none, has no pointer on it.
    onCard = false;
    if (next === shown) return;
    shown = next;
    for (const fn of [...heard]) fn();
  };
  // The first asking sets when it goes: one that comes while it's on its
  // way out doesn't put that off.
  const hideSoon = () => {
    hide ??= setTimeout(() => set(undefined), HIDE_AFTER);
  };

  return {
    shown: () => shown,
    hear(fn) {
      heard.add(fn);
      return () => void heard.delete(fn);
    },
    over(now) {
      clearTimeout(show);
      // Back on the ref whose card is up, from the card or from beside
      // it: the card stays.
      if (now && shown && same(now, shown)) return stopHiding();
      // The page says the pointer left the ref a turn after it did: by
      // then the pointer may be on the card already, and the card stays.
      if (shown && !onCard) hideSoon();
      if (now) show = setTimeout(() => set(now), SHOW_AFTER);
    },
    stay() {
      onCard = true;
      stopHiding();
    },
    leave() {
      onCard = false;
      if (shown) hideSoon();
    },
    clear() {
      clearTimeout(show);
      set(undefined);
    },
  };
}

// anchorIn is a box in a frame's page as somewhere in ws's window, for a
// card to be placed by: the frame's corner, and the box from it. It's
// read each time it's asked, so it's right wherever the frame is by then
// (a pane docked beside it since), and it's there for as long as the
// frame is.
export function anchorIn(frame: () => Pick<Element, "isConnected" | "getBoundingClientRect" | "clientLeft" | "clientTop"> | null | undefined, box: V1.Box): Anchor {
  return {
    get isConnected() {
      return !!frame()?.isConnected;
    },
    getBoundingClientRect() {
      const f = frame();
      const at = f?.getBoundingClientRect();
      // The page starts inside the frame's border, where it has one.
      const left = (at?.left ?? 0) + (f?.clientLeft ?? 0) + box.x;
      const top = (at?.top ?? 0) + (f?.clientTop ?? 0) + box.y;
      return { left, top, right: left + box.width, bottom: top + box.height };
    },
  };
}
