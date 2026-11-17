import { useEffect, useRef } from "react";

export type KeyMap = Record<string, (e: KeyboardEvent) => void>;

// keyName is how a key press is named in a KeyMap: "j", "J", "Enter",
// "Escape", "mod+d" (⌘ on macOS, ctrl elsewhere), "shift+ArrowDown". Shift
// is named only for keys whose name it doesn't change already.
export function keyName(e: KeyboardEvent): string {
  const mod = e.metaKey || e.ctrlKey ? "mod+" : "";
  const shift = e.shiftKey && e.key.length > 1 ? "shift+" : "";
  return mod + shift + e.key;
}

// typing says a key's typed into a field: a text box, an editor, or what
// says it's one with data-field (a Select's button and its list, where
// typing a name picks one).
function typing(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  return target.isContentEditable || ["INPUT", "TEXTAREA", "SELECT"].includes(target.tagName) || target.closest("[data-field]") !== null;
}

// While a modal (help, a finder) is open, only its keys work.
let modals = 0;

// The key maps bound now, so what's global (the go-to keys) can tell a
// key the page uses itself.
const bound = new Set<{ current: KeyMap }>();

// pageBinds is whether a page binds key now (named as keyName names it).
export function pageBinds(key: string): boolean {
  for (const m of bound) if (key in m.current) return true;
  return false;
}

// useKeys binds keys for as long as the component is mounted. Keys typed
// into a field are the field's, apart from Escape. A modal's keys work
// while it's open, and nobody else's do.
export function useKeys(keys: KeyMap, opts: { modal?: boolean } = {}) {
  const current = useRef(keys);
  current.current = keys;
  const modal = !!opts.modal;
  useEffect(() => {
    if (modal) modals++;
    bound.add(current);
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
      bound.delete(current);
      window.removeEventListener("keydown", onKey);
    };
  }, [modal]);
}
