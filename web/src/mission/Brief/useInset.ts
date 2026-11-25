import { useLayoutEffect, useState } from "react";
import { firstHead } from "./find";

// useInset is how far in the bar's toolbar starts, so that it starts
// where the documents' text does, however the panes are laid out:
// measured, as the text's column is centred in its own. side and open are
// what the layout goes by besides the window (the documents side by side,
// the planner beside them): it's measured again when they change.
export function useInset(main: HTMLElement | null, tools: HTMLElement | null, wide: boolean, side: boolean, open: boolean): number {
  const [inset, setInset] = useState(0);
  useLayoutEffect(() => {
    if (!main || !tools) return;
    // (A narrow window's toolbar is a row of its own, from the bar's edge.)
    if (!wide) return setInset(0);
    const look = () => {
      const head = firstHead(main);
      if (head) setInset(Math.max(0, Math.round(head.getBoundingClientRect().left - tools.getBoundingClientRect().left)));
    };
    look();
    const seen = new ResizeObserver(look);
    seen.observe(main);
    seen.observe(tools);
    return () => seen.disconnect();
  }, [main, tools, side, open, wide]);
  return inset;
}
