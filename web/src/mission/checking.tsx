import { useEffect, useState } from "react";
import type { Checking } from "../api";
import { Spinner } from "../ui/Spinner";
import { capital } from "../ui/text";

// What ws is running while it checks a step: a check that's a command,
// how long it's been going (ticking each second), the last line it's
// printed and when it's stopped; or the judge reading its checks.

// useSecond is the time now, ticking each second while since is set:
// the page's own clock ticks each minute.
function useSecond(since: string | undefined): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (!since) return;
    setNow(Date.now());
    const t = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(t);
  }, [since]);
  return now;
}

// clock is a length of time to the second: "45s", "1m 20s", "1h 2m".
export function clock(ms: number): string {
  const s = Math.max(0, Math.floor(ms / 1000));
  if (s < 60) return `${s}s`;
  if (s < 3600) return `${Math.floor(s / 60)}m ${String(s % 60).padStart(2, "0")}s`;
  return `${Math.floor(s / 3600)}h ${Math.floor((s % 3600) / 60)}m`;
}

// limit is a timeout as a step's limit says it: "15m", "1h".
function limit(ms: number): string {
  const m = Math.round(ms / 60000);
  return m >= 60 && m % 60 === 0 ? `${m / 60}h` : m >= 60 ? `${Math.floor(m / 60)}h${m % 60}m` : `${m}m`;
}

// checkNumbers names checks by number: "check 2", "checks 4 and 5".
export function checkNumbers(ns: number[] | undefined): string {
  if (!ns?.length) return "no checks";
  if (ns.length === 1) return `check ${ns[0]}`;
  return `checks ${ns.slice(0, -1).join(", ")} and ${ns[ns.length - 1]}`;
}

// checkingTitle is what's running, in a few words.
export function checkingTitle(c: Checking): string {
  return c.n ? `Check ${c.n} of ${c.of} for step ${c.step}` : `${capital(c.judge || "the judge")} is reading ${checkNumbers(c.judged)} of step ${c.step}`;
}

// CheckingNow is what's running, under the headline's title.
export function CheckingNow({ c }: { c: Checking }) {
  const now = useSecond(c.since);
  const took = now - new Date(c.since).getTime();
  const stopsIn = new Date(c.until).getTime() - new Date(c.since).getTime();
  return (
    <div className="checking-now">
      {c.n ? (
        <div className="checking-command">
          <code title={c.command}>{c.command}</code>
        </div>
      ) : null}
      <div className="checking-time">
        <Spinner size={12} />
        <span className="checking-elapsed" title={`Since ${new Date(c.since).toLocaleTimeString()}`}>
          {clock(took)}
        </span>
        <span className="dim">so far · stopped at {limit(stopsIn)}</span>
      </div>
      {c.n ? (
        <div className={c.line ? "checking-line" : "checking-line quiet"} title={c.line ? "The last line it printed" : undefined}>
          {c.line || "Nothing printed yet"}
        </div>
      ) : null}
      {c.n && c.judged?.length ? <div className="dim checking-after">Then the judge reads {checkNumbers(c.judged)}.</div> : null}
    </div>
  );
}

// CheckingLine is what's running in a line, for a list's row.
export function CheckingLine({ c }: { c: Checking }) {
  const now = useSecond(c.since);
  return (
    <>
      {checkingTitle(c)}
      {c.n ? (
        <>
          : <code>{c.command}</code>
        </>
      ) : null}{" "}
      · {clock(now - new Date(c.since).getTime())}
      {c.n && c.line ? <span className="dim"> · {c.line}</span> : null}
    </>
  );
}
