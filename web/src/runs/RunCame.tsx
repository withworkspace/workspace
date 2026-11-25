import { cx } from "../ui/cx";
import css from "./RunCame.module.css";

const results: Record<string, string> = { passed: css.passed, failed: css.failed, stopped: css.stopped, running: css.running };

// RunCame is what a task came to, in words in the result's colour: passed,
// failed, or (quietly) stopped or still running. as is the element it is:
// a fact's value in a list of them is a dd.
export function RunCame({ result, as: As = "span", children }: { result?: string; as?: "span" | "dd"; children: React.ReactNode }) {
  return <As className={cx(css.came, result && results[result])}>{children}</As>;
}
