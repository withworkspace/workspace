import type { Checking } from "../../api";
import { capital } from "../../ui/strings";

// How what ws is running while it checks a step is said.

// clock is a length of time to the second: "45s", "1m 20s", "1h 2m".
export function clock(ms: number): string {
  const s = Math.max(0, Math.floor(ms / 1000));
  if (s < 60) return `${s}s`;
  if (s < 3600) return `${Math.floor(s / 60)}m ${String(s % 60).padStart(2, "0")}s`;
  return `${Math.floor(s / 3600)}h ${Math.floor((s % 3600) / 60)}m`;
}

// limit is a timeout as a step's limit says it: "15m", "1h".
export function limit(ms: number): string {
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
