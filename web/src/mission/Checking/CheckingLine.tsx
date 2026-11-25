import type { Checking } from "../../api";
import { MissionCheckingLine } from "./MissionCheckingLine";
import { useSecond } from "./useSecond";
import { checkingTitle, clock } from "./words";

// CheckingLine is what's running while ws checks a step, in a line,
// ticking each second: drawn as a MissionCheckingLine.
export function CheckingLine({ c }: { c: Checking }) {
  const now = useSecond(c.since);
  return <MissionCheckingLine title={checkingTitle(c)} judge={!c.n} command={c.command} elapsed={clock(now - new Date(c.since).getTime())} line={c.line} />;
}
