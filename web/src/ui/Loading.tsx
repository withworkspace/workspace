import { useEffect, useState } from "react";
import css from "./Loading.module.css";
import { cx } from "./cx";
import { Spinner } from "./Spinner";

// Loading shows nothing for a moment (most loads are quicker), then a
// spinner and what's loading. It keeps its room meanwhile, so nothing
// moves when it shows.
export function Loading({ what, delay = 300, className }: { what?: string; delay?: number; className?: string }) {
  const [shown, setShown] = useState(delay === 0);
  useEffect(() => {
    const t = setTimeout(() => setShown(true), delay);
    return () => clearTimeout(t);
  }, [delay]);
  return (
    <div className={cx(css.loading, shown && css.shown, className)} role="status" aria-live="polite">
      {shown && (
        <>
          <Spinner size={16} />
          <span>{what ?? "Loading…"}</span>
        </>
      )}
    </div>
  );
}
