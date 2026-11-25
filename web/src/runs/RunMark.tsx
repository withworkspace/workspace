import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import { LiveDot } from "../ui/LiveDot";
import css from "./RunMark.module.css";

const results: Record<string, string> = { passed: css.passed, failed: css.failed };

// RunMark is a run's mark at the head of its line. One that's running is
// a dot that pulses, green once it's up (something answers on its port, or
// it's a task at work); one that's ended is a small circle (ended: in a
// line that's quieter for it, it sits a little higher). A task that's
// ended says what it came to (task: passed, failed, stopped), as an icon
// in that result's colour.
export function RunMark({ live, up, ended, task, title }: { live?: boolean; up?: boolean; ended?: boolean; task?: string; title?: string }) {
  if (task) {
    return (
      <span className={cx(css.mark, css.task, results[task])} role="img" aria-label={task} title={title}>
        <Icon name={task === "passed" ? "check-circle" : task === "failed" ? "x-circle" : "circle-dashed"} size={13} />
      </span>
    );
  }
  return (
    <span className={cx(css.mark, up && css.up, ended && css.ended)} aria-hidden title={title}>
      {live ? <LiveDot /> : <Icon name="circle" size={8} />}
    </span>
  );
}
