import { createContext, type MouseEvent, type ReactNode, useCallback, useContext, useEffect, useMemo, useRef, useState } from "react";
import css from "./ContextMenu.module.css";
import { MenuList } from "./Menu";
import { PopoverSurface } from "./Popover";

// A context menu is what can be done to the thing under the pointer,
// opened there by the other button of the mouse (or however the browser
// is asked for its own). There's one for the whole page, drawn by
// ContextMenuProvider; anything in it says what its menu holds with
// useContextMenu. It's a Menu's list, so its items are MenuItems, in
// MenuSections with MenuSeparators between, and its keys are a menu's.

// ContextItems is what a menu holds, drawn when it's opened.
export type ContextItems = () => ReactNode;

// Open opens it at a point in the page, with what it holds; closed is
// called once it's put away (another's opening puts it away too).
type Open = (at: { clientX: number; clientY: number }, label: string, items: ContextItems, closed?: () => void) => void;

// Pressed is what was selected as the press that asked for a menu began.
// On a Mac the press itself selects the word under it, for the browser's
// own menu (Safari and Chrome do), so what's selected by the time a menu
// is asked for isn't what was chosen. chosen says words were selected
// under the pointer, which is how they're copied; was is what was
// selected, to put back. Undefined when the menu's asked for by a key.
type Pressed = { chosen: boolean; was: Range | null };

const ContextMenuContext = createContext<{ open: Open; pressed: () => Pressed | undefined } | undefined>(undefined);

// under says a range is drawn at a point in the page.
function under(range: Range, x: number, y: number): boolean {
  for (const r of range.getClientRects()) if (x >= r.left && x <= r.right && y >= r.top && y <= r.bottom) return true;
  return false;
}

// putBack puts the selection back as it was before the press, if the
// browser has changed it since.
function putBack(was: Range | null) {
  const s = document.getSelection();
  if (!s) return;
  const now = s.rangeCount ? s.getRangeAt(0) : null;
  const same = now && was ? now.compareBoundaryPoints(Range.START_TO_START, was) === 0 && now.compareBoundaryPoints(Range.END_TO_END, was) === 0 : now === was;
  if (same) return;
  s.removeAllRanges();
  if (was) s.addRange(was);
}

type Shown = { label: string; items?: ContextItems; open: boolean; x: number; y: number; n: number };

// ContextMenuProvider goes round the page once: it holds the one menu,
// and draws it where it was asked for.
export function ContextMenuProvider({ children }: { children: ReactNode }) {
  const [shown, setShown] = useState<Shown>({ label: "", open: false, x: 0, y: 0, n: 0 });
  // Somewhere for the menu to open by: an empty box where the pointer was.
  const point = useRef<HTMLDivElement>(null);

  // What's to be told when the one that's open is put away.
  const closed = useRef<() => void>(undefined);
  const told = () => {
    const was = closed.current;
    closed.current = undefined;
    was?.();
  };
  const open = useCallback<Open>((at, label, items, then) => {
    told();
    closed.current = then;
    const { clientX: x, clientY: y } = at;
    setShown((s) => ({ label, items, open: true, x, y, n: s.n + 1 }));
  }, []);
  const close = useCallback(() => {
    told();
    setShown((s) => (s.open ? { ...s, open: false } : s));
  }, []);

  // What was selected as the last press began; a key put down since
  // forgets it, as a menu asked for by the keyboard has no press.
  const press = useRef<Pressed>(undefined);
  useEffect(() => {
    const down = (e: PointerEvent) => {
      const s = document.getSelection();
      const was = s?.rangeCount ? s.getRangeAt(0).cloneRange() : null;
      press.current = { chosen: s?.type === "Range" && !!was && under(was, e.clientX, e.clientY), was };
    };
    const key = () => (press.current = undefined);
    document.addEventListener("pointerdown", down, true);
    document.addEventListener("keydown", key, true);
    return () => {
      document.removeEventListener("pointerdown", down, true);
      document.removeEventListener("keydown", key, true);
    };
  }, []);
  const given = useMemo(() => ({ open, pressed: () => press.current }), [open]);

  // While it's open the rest of the page is out of reach (react-aria
  // makes it inert, as under any menu), so asking for a menu somewhere
  // else would do nothing to this one, and open the browser's over it.
  // It's passed on instead: this one closes, and once the page can be
  // pointed at again, whatever is under the pointer is asked for its
  // menu, as one press. Over the menu itself there's no menu to ask for,
  // the browser's neither.
  useEffect(() => {
    if (!shown.open) return;
    const on = (e: globalThis.MouseEvent) => {
      if (!e.isTrusted || !(e.target instanceof Element)) return; // not the one passed on
      e.preventDefault();
      if (e.target.closest('[role="menu"]')) return;
      e.stopPropagation();
      const { clientX, clientY } = e;
      close();
      requestAnimationFrame(() => {
        const under = document.elementFromPoint(clientX, clientY);
        under?.dispatchEvent(new window.MouseEvent("contextmenu", { bubbles: true, cancelable: true, view: window, button: 2, buttons: 0, clientX, clientY }));
      });
    };
    document.addEventListener("contextmenu", on, true);
    return () => document.removeEventListener("contextmenu", on, true);
  }, [shown.open, close]);

  return (
    <ContextMenuContext.Provider value={given}>
      {children}
      <div ref={point} className={css.point} style={{ left: shown.x, top: shown.y }} aria-hidden="true" />
      {/* A menu asked for while one's open is a new one, not that one moved. */}
      <PopoverSurface key={shown.n} open={shown.open} onOpenChange={(now) => !now && close()} at={point} placement="bottom start" offset={0}>
        <MenuList aria-label={shown.label} onClose={close} focused className={css.list}>
          {shown.items?.()}
        </MenuList>
      </PopoverSurface>
    </ContextMenuContext.Provider>
  );
}

// useContextMenu gives what opens the menu: called with what the menu is
// of (a screen reader hears it) and what it holds, it's an element's
// onContextMenu. With nothing to hold, the browser's own menu is left to
// open; so it is over words that were selected before the press, which
// is how they're copied; and so it is where there's no
// ContextMenuProvider. A word the press itself selected is let go again.
export function useContextMenu(): (label: string, items: ContextItems | null | undefined | false) => (e: MouseEvent<Element>) => void {
  const menus = useContext(ContextMenuContext);
  return useMemo(
    () => (label, items) => (e) => {
      if (!menus || !items) return;
      const pressed = menus.pressed();
      if (pressed ? pressed.chosen : document.getSelection()?.type === "Range") return;
      e.preventDefault();
      if (pressed) putBack(pressed.was);
      menus.open(e, label, items);
    },
    [menus],
  );
}

// useContextMenuAt gives what opens the menu at a point in the page, not
// from an event of this page's: a right click in a plugin's frame, which
// the frame says, at where in the page it was. closed is called once it's
// put away. It's false where there's no ContextMenuProvider.
export function useContextMenuAt(): (x: number, y: number, label: string, items: ContextItems, closed?: () => void) => boolean {
  const menus = useContext(ContextMenuContext);
  return useMemo(
    () => (x, y, label, items, closed) => {
      if (!menus) return false;
      menus.open({ clientX: x, clientY: y }, label, items, closed);
      return true;
    },
    [menus],
  );
}
