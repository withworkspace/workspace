import { createContext, useContext } from "react";

// pageScroller is what scrolls a page, as the shell around it stays put.
// Code that follows or moves the page's scroll asks it.
//
// It's the page's own scroller where the page has one: a page that fills
// its room and scrolls a part of itself (a mission's main column, beside
// its Details) marks that part data-page-scroller. The mark counts only
// while that part does scroll: on a narrow window such a page stacks, and
// the whole of it scrolls again. Otherwise it's the shell's <main>.
export function pageScroller(): HTMLElement | null {
  const own = document.querySelector<HTMLElement>("[data-page-scroller]");
  if (own && /auto|scroll/.test(getComputedStyle(own).overflowY)) return own;
  return document.getElementById("main");
}

// A part drawn in a dock scrolls with the dock's body, not with the page.
// The dock's body says so here (ScrollerContext), and a part that follows
// or moves "its" scroll asks useScroller for what scrolls it: the dock's
// body where it's in one, the page's scroller otherwise.
export const ScrollerContext = createContext<(() => HTMLElement | null) | null>(null);

export function useScroller(): () => HTMLElement | null {
  return useContext(ScrollerContext) ?? pageScroller;
}
