import { flushSync } from "react-dom";
import { Link, type LinkProps, useNavigate } from "react-router";

// TLink is a link that moves between a list and its page with a short
// cross-fade of the page (a view transition), where the browser has them;
// elsewhere it's a plain link. Modified clicks (a new tab) are the
// browser's.
export function TLink({ onClick, ...props }: LinkProps) {
  const navigate = useNavigate();
  return (
    <Link
      {...props}
      onClick={(e) => {
        onClick?.(e);
        if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
        const start = (document as Document & { startViewTransition?: (cb: () => void) => unknown }).startViewTransition;
        if (!start || window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
        e.preventDefault();
        start.call(document, () => flushSync(() => navigate(props.to)));
      }}
    />
  );
}
