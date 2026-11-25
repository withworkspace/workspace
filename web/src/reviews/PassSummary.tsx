import { Inline } from "../ui/Inline";
import { Time } from "../ui/Time";
import css from "./PassSummary.module.css";

// PassSummary is what a pass said of the change, and when.
export function PassSummary({ text, at }: { text: string; at?: string }) {
  return (
    <p className={css.summary}>
      <Inline text={text} /> {at && <Time iso={at} as="relative" className="dim" />}
    </p>
  );
}
