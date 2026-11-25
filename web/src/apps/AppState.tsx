import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import { Spinner } from "../ui/Spinner";
import { toneClass } from "../ui/tone";
import type { AppStanding } from "./words";
import css from "./AppState.module.css";

// AppState is how an app's run stands, in a word, in its tone: up (a dot),
// starting (the spinner), exited, or not running.
export function AppState({ kind, words, title }: AppStanding) {
  return (
    <span className={cx(css.state, toneClass(kind === "up" ? "done" : kind === "exited" ? "danger" : undefined))} title={title}>
      {kind === "up" ? (
        <>
          <span className={css.dot} /> {words}
        </>
      ) : kind === "starting" ? (
        <>
          <Spinner size={13} /> {words}
        </>
      ) : kind === "exited" ? (
        <>
          <Icon name="x-circle" size={13} /> {words}
        </>
      ) : (
        words
      )}
    </span>
  );
}
