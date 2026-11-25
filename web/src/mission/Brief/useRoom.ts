import { useEffect, useState } from "react";

// useRoom says whether an element is at least so wide, as it's laid out:
// whether the documents' pane has room for the brief and the plan side by
// side.
export function useRoom(el: HTMLElement | null, width: number): boolean {
  const [roomy, setRoomy] = useState(false);
  useEffect(() => {
    if (!el) return;
    const seen = new ResizeObserver(() => setRoomy(el.clientWidth >= width));
    seen.observe(el);
    return () => seen.disconnect();
  }, [el, width]);
  return roomy;
}
