import { type RefObject, useEffect } from "react";

// useClickAway closes a menu on a click anywhere outside it, but on what
// toggles it (matching toggles), which does that itself.
export function useClickAway(box: RefObject<HTMLElement | null>, toggles: string, close: () => void) {
  useEffect(() => {
    const down = (e: MouseEvent) => {
      const t = e.target as Element | null;
      if (box.current?.contains(t) || t?.closest?.(toggles)) return;
      close();
    };
    document.addEventListener("mousedown", down);
    return () => document.removeEventListener("mousedown", down);
  }, [box, toggles, close]);
}
