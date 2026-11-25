import { useLayoutEffect } from "react";
import { useLocation, useNavigationType } from "react-router";
import { pageScroller } from "../ui/scroller";

// useScrollMemory puts a page back where you left it when you come back
// to it (back, forward); somewhere new starts at the top.
const scrolls = new Map<string, number>();
export function useScrollMemory() {
  const location = useLocation();
  const how = useNavigationType();
  useLayoutEffect(() => {
    const main = document.getElementById("main");
    if (!main) return;
    const key = location.key;
    const want = how === "POP" ? scrolls.get(key) : undefined;
    // What scrolls is asked for each time: a page may scroll a part of
    // itself (ui/scroller.ts), which isn't there until the page has drawn.
    if (want === undefined) {
      const at = pageScroller();
      if (how !== "POP" && at) at.scrollTop = 0;
    } else {
      // The page may still be loading: try for a moment as it fills.
      let tries = 0;
      const put = () => {
        const at = pageScroller();
        if (at) at.scrollTop = want;
        if ((!at || Math.abs(at.scrollTop - want) > 2) && tries++ < 20) setTimeout(put, 50);
      };
      put();
    }
    // Kept as you scroll: by the time the next page draws, this one's gone.
    // Heard on the way down to whatever scrolls, as a scroll doesn't come
    // back up from a part of the page.
    const keep = (e: Event) => {
      if (e.target === pageScroller()) scrolls.set(key, (e.target as HTMLElement).scrollTop);
    };
    main.addEventListener("scroll", keep, { passive: true, capture: true });
    return () => main.removeEventListener("scroll", keep, { capture: true });
  }, [location.key, how]);
}
