import { Spinner } from "../ui/Spinner";
import { Time } from "../ui/Time";
import css from "./TeamAsOf.module.css";
import { Text } from "../ui/Text";

// TeamAsOf says, in the Team page's head, when git and GitHub were last
// looked at, or that they're being looked at now.
export function TeamAsOf({ at, looking }: { at: string; looking?: boolean }) {
  return (
    <Text dim className={css.asOf} title="Last refreshed. Refreshes every few minutes while open.">
      {looking ? (
        <>
          <Spinner size={12} /> Refreshing…
        </>
      ) : (
        <>
          As of <Time iso={at} as="relative" />
        </>
      )}
    </Text>
  );
}
