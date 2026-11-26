import { flushSync } from "react-dom";
import { Link, type LinkProps, useLocation, useNavigate, useResolvedPath } from "react-router";
import { keepsSidebarQuery } from "../chrome/chromeState";
import { wantsRoom } from "../shell/room";

// TLink is a link that moves between a list and its page with a short
// cross-fade of the page (a view transition), where the browser has them;
// elsewhere it's a plain link. Modified clicks (a new tab) are the
// browser's.
//
// It's a plain link too when the sidebar folds or opens on the way (to a
// mission, or back from one): the page's column moves then, and a view
// transition's pictures of the page, taken before and after, don't move
// with it (they trail it, are squeezed into it, or lie over the sidebar,
// each browser its own way). The page coming fades in by itself there
// (styles.css, page-in). On a window wide enough that the sidebar stays
// open everywhere (keepsSidebarFrom), it never folds on the way.
export function TLink({ onClick, ...props }: LinkProps) {
  const navigate = useNavigate();
  const here = useLocation().pathname;
  const there = useResolvedPath(props.to).pathname;
  return (
    <Link
      {...props}
      onClick={(e) => {
        onClick?.(e);
        if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
        const start = (document as Document & { startViewTransition?: (cb: () => void) => unknown }).startViewTransition;
        if (!start || window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
        if (wantsRoom(here) !== wantsRoom(there) && !window.matchMedia(keepsSidebarQuery).matches) return;
        e.preventDefault();
        start.call(document, () => flushSync(() => navigate(props.to)));
      }}
    />
  );
}
