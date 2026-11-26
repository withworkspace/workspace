import { Spinner } from "../ui/Spinner";
import { Time } from "../ui/Time";
import css from "./ReviewsAsOf.module.css";
import { Text } from "../ui/Text";

// ReviewsAsOf says when GitHub was last asked for the open pull requests,
// or that it's being asked now.
export function ReviewsAsOf({ at, asking }: { at: string; asking: boolean }) {
  return (
    <Text dim className={css.asOf} title="When GitHub was last asked. While this page is open it's asked every couple of minutes.">
      {asking ? (
        <>
          <Spinner size={12} /> Asking GitHub…
        </>
      ) : (
        <>
          As of <Time iso={at} as="relative" />
        </>
      )}
    </Text>
  );
}
