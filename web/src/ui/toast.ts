// What's said in passing, at the foot of the page, for a few seconds: what
// was done, or what went wrong. toast says it; Toasts (Toasts.tsx) draws
// what's being said. It's kept here, apart from how it's drawn, so a part
// can say something without drawing anything.

export interface ToastItem {
  id: number;
  text: string;
  tone: "info" | "danger" | "done";
  keys?: string; // a key said after the words, as Keys draws one (a key hint)
  action?: { label: string; run: () => void };
}

let toasts: ToastItem[] = [];
const listeners = new Set<() => void>();
let toastID = 0;

export function setToasts(next: ToastItem[]) {
  toasts = next;
  listeners.forEach((l) => l());
}

export const getToasts = () => toasts;

export function onToasts(l: () => void): () => void {
  listeners.add(l);
  return () => listeners.delete(l);
}

// toast says something, for a few seconds (longer when there's something
// to press: Undo, Open). Three at most are shown: a new one takes the
// oldest's place.
export function toast(text: string, opts: { tone?: ToastItem["tone"]; action?: ToastItem["action"]; keys?: string; ms?: number } = {}) {
  const id = ++toastID;
  setToasts([...toasts.slice(-2), { id, text, tone: opts.tone ?? "info", action: opts.action, keys: opts.keys }]);
  setTimeout(() => setToasts(toasts.filter((t) => t.id !== id)), opts.ms ?? (opts.action ? 6000 : 3500));
}
