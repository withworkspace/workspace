import { Link } from "react-router";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import { Inline } from "../ui/inline";
import css from "./MissionPlanChecks.module.css";

// A check of a step's, as it came out: what it asks, whether it was met
// (or couldn't be told, or is yours to say), why not in a line, and where
// all it printed is kept.
export type PlanCheck = { text: string; result?: "met" | "unmet" | "can't tell" | "yours"; why?: string; kept?: string };

// MissionPlanChecks is what a step's checks came to in their latest round,
// under the step: each with a tick, a cross or a question mark, and why
// where it wasn't met.
export function MissionPlanChecks({ checks }: { checks: PlanCheck[] }) {
  return (
    <ul className={css.checks}>
      {checks.map((c) => (
        <li key={c.text} className={cx(css.check, c.result && c.result !== "met" && css.notMet)}>
          <Icon name={c.result === "met" ? "check" : c.result === "unmet" ? "x" : "question"} size={11} />{" "}
          <span>
            <Inline text={c.text} />
          </span>
          {c.result && c.result !== "met" && c.why && <span className={css.why}>{c.why}</span>}
          {c.kept && (
            <Link to={`/runs?show=${encodeURIComponent(c.kept)}`} className={css.kept} title="All it printed, and what it wrote">
              <Icon name="terminal" size={11} />
            </Link>
          )}
        </li>
      ))}
    </ul>
  );
}
