import { cx } from "../ui/cx";
import { Icon, type IconName } from "../ui/Icon";
import { Inline } from "../ui/inline";
import css from "./MissionPlanCheckList.module.css";

// A check of a step's: what it asks, how it last came out (or nothing,
// where it hasn't been run), and who said so.
export type StepCheck = { text: string; result?: "met" | "unmet" | "can't tell" | "yours"; by?: string };

const results: Record<NonNullable<StepCheck["result"]>, [IconName, string]> = {
  met: ["check", css.met],
  unmet: ["x", css.unmet],
  "can't tell": ["question", css.cant],
  yours: ["user", css.yours],
};

// MissionPlanCheckList is a step's checks in a mission's details, each
// with how it last came out in its icon's colour, two lines of it at
// most; said, with how it came out and who said it in words too, where
// it isn't met.
export function MissionPlanCheckList({ checks, said }: { checks: StepCheck[]; said?: boolean }) {
  return (
    <ul className={css.checks}>
      {checks.map((c, i) => {
        const [icon, cls] = c.result ? results[c.result] : (["circle-dashed", css.none] as [IconName, string]);
        return (
          <li key={i} className={cx(css.check, cls)}>
            <Icon name={icon} size={12} />
            <span className={css.text}>
              <Inline text={c.text} />
              {said && c.result && c.result !== "met" && (
                <span className="dim">
                  {" "}
                  ({c.result}
                  {c.by ? `: ${c.by}` : ""})
                </span>
              )}
            </span>
          </li>
        );
      })}
    </ul>
  );
}
