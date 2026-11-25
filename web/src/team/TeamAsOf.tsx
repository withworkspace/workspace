import { cx } from "../ui/cx";
import { Spinner } from "../ui/Spinner";
import { Time } from "../ui/Time";
import css from "./TeamAsOf.module.css";

// TeamAsOf says, in the Team page's head, when git and GitHub were last
// looked at, or that they're being looked at now.
export function TeamAsOf({ at, looking }: { at: string; looking?: boolean }) {
  return (
    <span className={cx("dim", css.asOf)} title="When git and GitHub were last looked at. While this page is open they're looked at every couple of minutes.">
      {looking ? (
        <>
          <Spinner size={12} /> Looking…
        </>
      ) : (
        <>
          As of <Time iso={at} as="relative" />
        </>
      )}
    </span>
  );
}
