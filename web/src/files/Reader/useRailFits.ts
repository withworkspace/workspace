import { type RefObject, useLayoutEffect, useState } from "react";
import { railRoom } from "../reading";

// useRailFits is whether the rail fits beside the text in the reader as
// wide as it is now (the page's width inside its padding).
export function useRailFits(page: RefObject<HTMLElement | null>): boolean {
  const [fits, setFits] = useState(true);
  useLayoutEffect(() => {
    const el = page.current;
    if (!el) return;
    const check = () => {
      const s = getComputedStyle(el);
      setFits(el.clientWidth - parseFloat(s.paddingLeft) - parseFloat(s.paddingRight) >= railRoom);
    };
    check();
    const watch = new ResizeObserver(check);
    watch.observe(el);
    return () => watch.disconnect();
  }, [page]);
  return fits;
}
