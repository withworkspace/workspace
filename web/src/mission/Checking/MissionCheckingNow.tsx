import { cx } from "../../ui/cx";
import { Spinner } from "../../ui/Spinner";
import css from "./MissionCheckingNow.module.css";
import { Text } from "../../ui/Text";

type MissionCheckingNowProps = {
  // The judge's turn: it's reading checks, not running one.
  judge?: boolean;
  // The check that's a command, as it's run.
  command?: string;
  // How long it's been going, and since when (its title).
  elapsed: string;
  since: string;
  // When it's stopped, as a step's limit says it.
  stops: string;
  // A command's last line printed (none: nothing printed yet).
  line?: string;
  // The checks the judge reads after it, in words.
  after?: string;
};

// MissionCheckingNow is what ws is running while it checks a step, under
// the headline's title: the command, how long it's been going (and when
// it's stopped), the last line it printed, and what the judge reads after
// it; or, on the judge's turn, how long it's been reading.
export function MissionCheckingNow({ judge, command, elapsed, since, stops, line, after }: MissionCheckingNowProps) {
  return (
    <div className={css.now}>
      {!judge ? (
        <div className={css.command}>
          <code title={command}>{command}</code>
        </div>
      ) : null}
      <div className={css.time}>
        <Spinner size={12} />
        <span className={css.elapsed} title={`Since ${since}`}>
          {elapsed}
        </span>
        <Text dim>so far · stops at {stops}</Text>
      </div>
      {!judge ? (
        <div className={line ? css.line : cx(css.line, css.quiet)} title={line ? "Last output line" : undefined}>
          {line || "No output yet"}
        </div>
      ) : null}
      {after ? <Text as="div" dim className={css.after}>Then the judge reads {after}.</Text> : null}
    </div>
  );
}
