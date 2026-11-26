import { cx } from "../ui/cx";
import { Inline } from "../ui/Inline";
import { Spinner } from "../ui/Spinner";
import css from "./TeamGist.module.css";
import { Text } from "../ui/Text";

// TeamGist is what a change does, as an agent wrote it from the change,
// under its name; or, reading, that it's being written.
export function TeamGist({ text, reading }: { text?: string; reading?: boolean }) {
  if (text) return <Inline text={text} className={css.gist} />;
  if (!reading) return null;
  return (
    <Text dim className={cx(css.gist, css.reading)}>
      <Spinner size={11} /> Reading the change…
    </Text>
  );
}
