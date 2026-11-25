import { Time } from "../ui/Time";
import css from "./TeamSeenLine.module.css";

// TeamSeenLine is the rule in a day's rows under what's new: you'd seen
// what's below it when you last looked.
export function TeamSeenLine({ at }: { at: string }) {
  return (
    <li className={css.line} role="separator">
      <span>
        You'd seen what's below <Time iso={at} as="relative" />
      </span>
    </li>
  );
}
