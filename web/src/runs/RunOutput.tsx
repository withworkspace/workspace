import { useEffect, useRef } from "react";
import { cx } from "../ui/cx";
import { humanError } from "../ui/strings";
import { useRunLog } from "./runlog";
import { plain } from "./runRows";
import css from "./RunOutput.module.css";

// RunOutput is the end of a run's output, followed while it runs (live),
// and kept at its foot as more comes unless you've scrolled up. A page
// gives it its height by a class of its own: the Runs page's rows cap
// it, a place's Runs tab fills what's left with it.
export function RunOutput({ runRef, live, className, empty = "No output." }: { runRef: string; live: boolean; className?: string; empty?: string }) {
  const { value, error } = useRunLog(runRef, live);
  const pre = useRef<HTMLPreElement>(null);
  const stuck = useRef(true); // at the foot: new output keeps it there
  const text = value ? plain(value.log) : "";
  useEffect(() => {
    const el = pre.current;
    if (el && stuck.current) el.scrollTop = el.scrollHeight;
  }, [text]);
  // Another run's output starts at its foot.
  useEffect(() => {
    stuck.current = true;
  }, [runRef]);
  if (error && !value) return <div className={cx(css.unread, className)}>Could not read output: {humanError(error)}</div>;
  return (
    <pre
      ref={pre}
      className={cx(css.log, className)}
      tabIndex={0}
      aria-label="Output"
      onScroll={(e) => {
        const el = e.currentTarget;
        stuck.current = el.scrollHeight - el.scrollTop - el.clientHeight < 24;
      }}
    >
      {value ? text || <span className={css.empty}>{live ? "Nothing yet." : empty}</span> : "…"}
    </pre>
  );
}
