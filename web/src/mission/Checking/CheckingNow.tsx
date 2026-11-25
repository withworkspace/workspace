import type { Checking } from "../../api";
import { MissionCheckingNow } from "./MissionCheckingNow";
import { useSecond } from "./useSecond";
import { checkNumbers, clock, limit } from "./words";

// CheckingNow is what's running while ws checks a step, under the
// headline's title, ticking each second: drawn as a MissionCheckingNow.
export function CheckingNow({ c }: { c: Checking }) {
  const now = useSecond(c.since);
  const took = now - new Date(c.since).getTime();
  const stopsIn = new Date(c.until).getTime() - new Date(c.since).getTime();
  return (
    <MissionCheckingNow
      judge={!c.n}
      command={c.command}
      elapsed={clock(took)}
      since={new Date(c.since).toLocaleTimeString()}
      stops={limit(stopsIn)}
      line={c.line}
      after={c.n && c.judged?.length ? checkNumbers(c.judged) : undefined}
    />
  );
}
