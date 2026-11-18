// A backdrop in a plugin's app: what it puts behind a dialog, drawn with
// the class ws's shell styles (.ws-backdrop, plugins/shell.css). In a
// frame it can only dim the frame, so while the app's page has one ws dims
// what's beside the frame (dim.ts, App.tsx), and the two read as one.
//
// ws looks for it in the page; the app doesn't say. The frame is this
// origin's own, so a watch on its document is told of a change in the same
// turn the app made it, before anything's drawn: both halves come in the
// same frame and go in the same frame. A message on the port arrives a
// turn later, which for a click is a frame later, every time.

// The class of the shell's backdrop.
export const BACKDROP = "ws-backdrop";

// Watched is as much of a page's document as the watch uses.
export interface Watched {
  readonly documentElement: Node;
  querySelector(selectors: string): unknown;
}

// Observe runs changed whenever something under root might have gained or
// lost the class, and returns what stops it.
export type Observe = (root: Node, changed: () => void) => () => void;

// In a browser: elements coming and going, and classes changing.
const mutations: Observe = (root, changed) => {
  const watch = new MutationObserver(changed);
  watch.observe(root, { childList: true, subtree: true, attributes: true, attributeFilter: ["class"] });
  return () => watch.disconnect();
};

// watchBackdrop says whether the page has a backdrop up: once at the
// start if it has, then as its first comes and as its last goes. It
// returns what stops it.
export function watchBackdrop(page: Watched, said: (on: boolean) => void, observe: Observe = mutations): () => void {
  let on = false;
  const look = () => {
    const now = page.querySelector(`.${BACKDROP}`) !== null;
    if (now === on) return;
    on = now;
    said(on);
  };
  const stop = observe(page.documentElement, look);
  look();
  return stop;
}
