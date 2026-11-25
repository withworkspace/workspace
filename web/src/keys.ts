import { useEffect, useMemo, useRef } from "react";

export type KeyMap = Record<string, (e: KeyboardEvent) => void>;

// keyName is how a key press is named in a KeyMap: "j", "J", "Enter",
// "Escape", "mod+d" (⌘ on macOS, ctrl elsewhere), "shift+ArrowDown". Shift
// is named only for keys whose name it doesn't change already.
export function keyName(e: KeyboardEvent): string {
  const mod = e.metaKey || e.ctrlKey ? "mod+" : "";
  const shift = e.shiftKey && e.key.length > 1 ? "shift+" : "";
  return mod + shift + e.key;
}

// How long after g its letter still counts, in milliseconds: g then m
// goes to the missions.
export const leaderWithin = 900;

// The keys of ws's own that work in a plugin's frame as they do outside
// it. The frame is another document, so a key pressed there never reaches
// this one: the plugin's page is told these (the frame protocol's
// welcome), and passes each on as it's pressed. With ⌘ or Ctrl held, k
// (Find), p (Find, for files) and j (the page's conversation), always; bare, when nothing's being typed
// in, ? (these keys), C (a new conversation, beside) and g, and the
// letter after g. They're ws's global keys (useGlobalKeys in App.tsx):
// one added there that should work in a plugin's page is added here. z
// (a peek at what the pointer's on) isn't: what the pointer's on in a
// plugin's page is the plugin's to know.
export const frameKeys = { chords: ["k", "p", "j"], bare: ["?", "C", "g"], leaders: ["g"], within: leaderWithin };

// typing says a key's typed into a field: a text box, an editor, or what
// says it's one with data-field (a Select's button and its list, where
// typing a name picks one).
function typing(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  return target.isContentEditable || ["INPUT", "TEXTAREA", "SELECT"].includes(target.tagName) || target.closest("[data-field]") !== null;
}

// While a modal (help, a finder) is open, only its keys work.
let modals = 0;

// useKeys binds keys for as long as the component is mounted. Keys typed
// into a field are the field's, apart from Escape. A modal's keys work
// while it's open, and nobody else's do.
export function useKeys(keys: KeyMap, opts: { modal?: boolean } = {}) {
  const current = useRef(keys);
  current.current = keys;
  const modal = !!opts.modal;
  useEffect(() => {
    if (modal) modals++;
    const onKey = (e: KeyboardEvent) => {
      // Handled already (an editor's own Escape, say, which may have closed
      // it, and with it the modal that kept this one quiet).
      if (e.defaultPrevented) return;
      if (e.isComposing || (typing(e.target) && e.key !== "Escape")) return;
      if (!modal && modals > 0) return;
      const fn = current.current[keyName(e)];
      if (fn) {
        e.preventDefault();
        fn(e);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => {
      if (modal) modals--;
      window.removeEventListener("keydown", onKey);
    };
  }, [modal]);
}

// A Keyed function is one a key calls too, and says which: what
// useShortcut gives back. A button given it to press (ui/Button.tsx)
// reads the key off it, for its tooltip, and for the hint that there was
// a key when it's pressed with the pointer (ui/keyHint.ts).
export type Keyed<F> = F & { readonly keys?: string };

// keysOf is the key a function says calls it, if it's a Keyed one that's
// bound now.
export function keysOf(fn: unknown): string | undefined {
  return typeof fn === "function" ? (fn as Keyed<() => void>).keys || undefined : undefined;
}

// useShortcut binds a key to fn, as useKeys does, and gives fn back
// saying the key, to hand to the button that does the same: the key's
// written once, and the button knows it.
//
//   const close = useShortcut("Escape", () => setOpen(false));
//   <Button onClick={close} title="Close">
//
// Given several keys, each calls fn, and the first is the one it says
// (["+", "="]: + in the tooltip, and = does it too). While enabled is
// false the key isn't bound and the function says none, though it still
// does what it does when called. It's the same function for as long as
// its key is, so it can be a prop or a dependency.
//
// It has to reach the button as it is: () => close() around it has no
// key to read. A button whose press isn't what the key does (the key
// acts on the row the cursor's on, the button on its own) is told the
// key itself: <Button keys="v">.
export function useShortcut(key: string | string[], fn: () => void, opts: { enabled?: boolean; modal?: boolean } = {}): Keyed<() => void> {
  const current = useRef(fn);
  current.current = fn;
  const all = typeof key === "string" ? [key] : key;
  const on = opts.enabled !== false;
  const map: KeyMap = {};
  if (on) for (const k of all) map[k] = () => current.current();
  useKeys(map, { modal: opts.modal });
  const said = on ? all[0] : undefined;
  return useMemo(() => Object.assign(() => current.current(), { keys: said }), [said]);
}

// A search box of a page's own (the explorer's, in a capsule) is nearer
// than Find's code search: while one is mounted, mod+shift+F opens it.
const codeSearchers: (() => void)[] = [];

// claimCodeSearch says the page has a search box of its own, which open
// opens, until what it returns is called. The one claimed last wins.
export function claimCodeSearch(open: () => void): () => void {
  codeSearchers.push(open);
  return () => {
    const i = codeSearchers.indexOf(open);
    if (i >= 0) codeSearchers.splice(i, 1);
  };
}

// useOwnCodeSearch claims mod+shift+F for the page's own search box while
// open is given (undefined: it hasn't one, now).
export function useOwnCodeSearch(open: (() => void) | undefined) {
  const current = useRef(open);
  current.current = open;
  const has = !!open;
  useEffect(() => (has ? claimCodeSearch(() => current.current?.()) : undefined), [has]);
}

// isCodeSearchKey is mod+shift+F, however the browser spells its letter
// (with ⌘ held, some say "f", some "F").
export const isCodeSearchKey = (e: Pick<KeyboardEvent, "metaKey" | "ctrlKey" | "shiftKey" | "altKey" | "key">) => (e.metaKey || e.ctrlKey) && e.shiftKey && !e.altKey && e.key.toLowerCase() === "f";

// codeSearchKey is what mod+shift+F does now: nothing while an overlay is
// open (Find, a finder, a dialog: the key's theirs, or nobody's); the
// page's own search box, where it has one; Find's code search otherwise.
export function codeSearchKey(overlayOpen: boolean): { to: "nothing" } | { to: "own"; open: () => void } | { to: "find" } {
  if (overlayOpen) return { to: "nothing" };
  const own = codeSearchers[codeSearchers.length - 1];
  return own ? { to: "own", open: own } : { to: "find" };
}
