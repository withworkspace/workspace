import { type KeyLike, saidOf } from "../keymap/keymap";
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

// How many are shown at once: a new one sends the oldest away.
const most = 3;

// How long one takes to slide away, in milliseconds (Toasts.module.css's
// leave, which is as long).
export const leaveMs = 180;

// Leaving is react-aria's queue, but one that's closed (timed out, its ×,
// sent away by a new one) stays a moment first, said to be leaving, so
// it's seen to slide away rather than vanish: react-aria's drops it at
// once. Where motion's to be kept down it goes at once still.
class Leaving<T> extends ToastQueue<T> {
  private gone = new Set<string>();
  private told = new Set<() => void>();

  // leaving is whether the toast with key is on its way out.
  leaving = (key: string) => this.gone.has(key);

  // goingNow is the keys of those leaving: a new set each time one starts
  // or finishes, for useSyncExternalStore to tell apart.
  goingNow = (): ReadonlySet<string> => this.gone;

  // onLeaving is told when one starts to leave.
  onLeaving = (fn: () => void) => {
    this.told.add(fn);
    return () => void this.told.delete(fn);
  };

  close(key: string) {
    if (this.gone.has(key)) return;
    const still = typeof window === "undefined" || window.matchMedia?.("(prefers-reduced-motion: reduce)").matches;
    if (still || !this.visibleToasts.some((t) => t.key === key)) return super.close(key);
    this.gone = new Set(this.gone).add(key);
    for (const fn of this.told) fn();
    setTimeout(() => {
      super.close(key);
      this.gone = new Set([...this.gone].filter((k) => k !== key));
    }, leaveMs);
  }

  clear() {
    this.gone = new Set();
    super.clear();
  }
}

// toasts is what's being said, newest first (visibleToasts), those
// leaving too.
export const toasts = new Leaving<ToastItem>();

// saying is what's being said now, newest first: not what's leaving.
export const saying = (): ToastItem[] => toasts.visibleToasts.filter((t) => !toasts.leaving(t.key)).map((t) => t.content);

// toast says something, for a few seconds (longer when there's something
// to press: Undo, Open), or as long as ms says.
export function toast(text: string, opts: { tone?: ToastItem["tone"]; action?: ToastItem["action"]; keys?: KeyLike; ms?: number } = {}) {
  const staying = toasts.visibleToasts.filter((t) => !toasts.leaving(t.key));
  for (const t of staying.slice(most - 1)) toasts.close(t.key);
  toasts.add({ text, tone: opts.tone ?? "info", action: opts.action, keys: saidOf(opts.keys) }, { timeout: opts.ms ?? (opts.action ? 6000 : 3500) });
}
