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

type Open = (e: MouseEvent<Element>, label: string, items: ContextItems) => void;

const ContextMenuContext = createContext<Open | undefined>(undefined);

type Shown = { label: string; items?: ContextItems; open: boolean; x: number; y: number; n: number };

// ContextMenuProvider goes round the page once: it holds the one menu,
// and draws it where it was asked for.
export function ContextMenuProvider({ children }: { children: ReactNode }) {
  const [shown, setShown] = useState<Shown>({ label: "", open: false, x: 0, y: 0, n: 0 });
  // Somewhere for the menu to open by: an empty box where the pointer was.
  const point = useRef<HTMLDivElement>(null);

  const open = useCallback<Open>((e, label, items) => {
    const { clientX: x, clientY: y } = e;
    setShown((s) => ({ label, items, open: true, x, y, n: s.n + 1 }));
  }, []);
  const close = useCallback(() => setShown((s) => (s.open ? { ...s, open: false } : s)), []);

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
    <ContextMenuContext.Provider value={open}>
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
// open; so it is while words are selected, which is how they're copied;
// and so it is where there's no ContextMenuProvider.
export function useContextMenu(): (label: string, items: ContextItems | null | undefined | false) => (e: MouseEvent<Element>) => void {
  const open = useContext(ContextMenuContext);
  return useMemo(
    () => (label, items) => (e) => {
      if (!open || !items) return;
      if (document.getSelection()?.type === "Range") return;
      e.preventDefault();
      open(e, label, items);
    },
    [open],
  );
}
