// The wait of something shown on hover over a plugin's frame (a ref's
// card: ui.ts), as ws has it for a ref in its own pages. The frame's page
// says when the pointer comes to what it's about, and when it goes; ws
// sees the pointer on what it drew itself. Whoever's looking can't tell
// the ref was in a frame: the card waits as long before it shows, stays
// as long after the pointer goes, and can be moved onto.

// How long the pointer's on something before what's about it shows, and
// how long that stays once the pointer's gone, so it can be moved onto:
// as for a ref in ws's own pages (refs/RefTag.tsx).
export const SHOW_AFTER = 350;
export const HIDE_AFTER = 160;

// Hovering is what's shown on hover over one frame: which, as the page
// says where the pointer is and ws says when it's on what's shown.
export interface Hovering<T> {
  // What's shown, if anything is.
  shown(): T | undefined;
  // hear runs fn when what's shown changes, and returns what stops it.
  hear(fn: () => void): () => void;
  // The page said the pointer's on this now, or on nothing.
  over(now: T | undefined): void;
  // The pointer's on what's shown: it stays for as long as it is.
  stay(): void;
  // The pointer left what's shown.
  leave(): void;
  // Nothing's shown, at once: the page went, or ws moved it.
  clear(): void;
}

// hovering keeps one frame's. Something shows a moment after the pointer
// comes to what it's about, and goes a moment after the pointer leaves
// both that and what's shown. On the way from one to another, the first's
// goes as it would have, and the second's comes as it would have: each as
// if it were alone. same says whether two are of the same thing in the
// same place: the pointer coming back to it keeps what's shown.
export function hovering<T>(same: (a: T, b: T) => boolean): Hovering<T> {
  let shown: T | undefined;
  let onShown = false;
  let show: ReturnType<typeof setTimeout> | undefined;
  let hide: ReturnType<typeof setTimeout> | undefined;
  const heard = new Set<() => void>();

  const stopHiding = () => {
    clearTimeout(hide);
    hide = undefined;
  };
  const set = (next: T | undefined) => {
    stopHiding();
    // What's new, or nothing, has no pointer on it.
    onShown = false;
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
      // Back on what's shown, from it or from beside it: it stays.
      if (now !== undefined && shown !== undefined && same(now, shown)) return stopHiding();
      // The page says the pointer left a turn after it did: by then the
      // pointer may be on what's shown already, and it stays.
      if (shown !== undefined && !onShown) hideSoon();
      if (now !== undefined) show = setTimeout(() => set(now), SHOW_AFTER);
    },
    stay() {
      onShown = true;
      stopHiding();
    },
    leave() {
      onShown = false;
      if (shown !== undefined) hideSoon();
    },
    clear() {
      clearTimeout(show);
      set(undefined);
    },
  };
}
