import { useEffect, useLayoutEffect } from "react";

// fitText sizes a textarea to what's in it, up to max pixels. One that
// isn't laid out (in a view that's hidden) has no height to go by: it's
// left at its rows, and fitted when it's shown.
function fitText(el: HTMLTextAreaElement, max: number) {
  if (el.getClientRects().length === 0) {
    el.style.height = "";
    return;
  }
  el.style.height = "auto";
  el.style.height = `${Math.min(el.scrollHeight + 2, max)}px`;
  el.style.overflowY = el.scrollHeight + 2 > max ? "auto" : "hidden";
}

// useAutosize grows a textarea with what's typed, up to max pixels, and
// fits it again when its width changes: when a hidden view it's in is
// shown, or the window's resized and its lines wrap differently.
export function useAutosize(ref: React.RefObject<HTMLTextAreaElement | null>, value: string, max = 320) {
  useLayoutEffect(() => {
    const el = ref.current;
    if (el) fitText(el, max);
  }, [ref, value, max]);
  useEffect(() => {
    const el = ref.current;
    if (!el || typeof ResizeObserver === "undefined") return;
    // Only its width: fitting it changes its height, which would come
    // back here.
    let width = el.clientWidth;
    const watch = new ResizeObserver(() => {
      if (el.clientWidth === width) return;
      width = el.clientWidth;
      fitText(el, max);
    });
    watch.observe(el);
    return () => watch.disconnect();
  }, [ref, max]);
}
