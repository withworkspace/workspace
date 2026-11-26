import { cx } from "../ui/cx";
import { Icon, type IconName } from "../ui/Icon";
import { Inline } from "../ui/Inline";
import type { DecisionEffect } from "./decisions";
import css from "./MissionDecisionEffects.module.css";

type Mark = { icon: IconName; tone: string; words: string };
const marks: Record<string, Mark> = {
  met: { icon: "check-circle", tone: css.met, words: "met, citing this decision" },
  unmet: { icon: "x-circle", tone: css.unmet, words: "unmet, citing this decision" },
};
// A check that came out some other way (can't tell), and a step passed.
const other: Mark = { icon: "question", tone: css.open, words: "judged, citing this decision" };
const passed: Mark = { icon: "check", tone: css.met, words: "passed on this decision" };

// MissionDecisionEffects is where a decision was reckoned with, a line
// each: a step's check whose verdict names it, with how the check came
// out and its words, or a step that was passed on it.
export function MissionDecisionEffects({ effects }: { effects: DecisionEffect[] }) {
  if (!effects.length) return null;
  return (
    <ul className={css.effects} aria-label="Where applied">
      {effects.map((e) => {
        const m = e.check ? (marks[e.result ?? ""] ?? other) : passed;
        return (
          <li key={`${e.step}:${e.check ?? 0}`} className={css.effect}>
            <Icon name={m.icon} size={12} className={cx(css.mark, m.tone)} />
            <span className={css.where}>
              Step {e.step}
              {e.check ? `, check ${e.check}` : ""}
            </span>
            <span>{m.words}</span>
            <span className={css.text}>
              <Inline text={e.text} lines={1} />
            </span>
          </li>
        );
      })}
    </ul>
  );
}
