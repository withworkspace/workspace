import { Spinner } from "../ui/Spinner";
import { Time } from "../ui/Time";
import css from "./ReviewsAsOf.module.css";

// ReviewsAsOf says when GitHub was last asked for the open pull requests,
// or that it's being asked now.
export function ReviewsAsOf({ at, asking }: { at: string; asking: boolean }) {
  return (
    <span className={`dim ${css.asOf}`} title="When GitHub was last asked. While this page is open it's asked every couple of minutes.">
      {asking ? (
        <>
          <Spinner size={12} /> Asking GitHub…
        </>
      ) : (
        <>
          As of <Time iso={at} as="relative" />
        </>
      )}
    </span>
  );
}
