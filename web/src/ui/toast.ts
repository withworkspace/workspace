import { UNSTABLE_ToastQueue as ToastQueue } from "react-aria-components";

// What's said in passing, in the window's corner, for a few seconds: what
// was done, or what went wrong. toast says it; Toasts (Toasts.tsx) draws
// what's being said. It's kept here, apart from how it's drawn, so a part
// can say something without drawing anything.
//
// It's react-aria's toast queue: Toasts is its region, which keeps each
// one while the pointer's on it or the focus is in it, and is a landmark
// F6 reaches (so its Undo can be pressed from the keys).

export interface ToastItem {
  text: string;
  tone: "info" | "danger" | "done";
  action?: { label: string; run: () => void };
  keys?: string; // a key said after the words, as Keys draws one (a key hint)
}

// How many are shown at once: a new one takes the oldest's place.
const most = 3;

// toasts is what's being said, newest first (visibleToasts).
export const toasts = new ToastQueue<ToastItem>({ maxVisibleToasts: most });

// saying is what's being said now, newest first.
export const saying = (): ToastItem[] => toasts.visibleToasts.map((t) => t.content);

// toast says something, for a few seconds (longer when there's something
// to press: Undo, Open), or as long as ms says.
export function toast(text: string, opts: { tone?: ToastItem["tone"]; action?: ToastItem["action"]; keys?: string; ms?: number } = {}) {
  const shown = toasts.visibleToasts;
  if (shown.length >= most) toasts.close(shown[shown.length - 1].key);
  toasts.add({ text, tone: opts.tone ?? "info", action: opts.action, keys: opts.keys }, { timeout: opts.ms ?? (opts.action ? 6000 : 3500) });
}
