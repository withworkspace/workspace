import { useEffect, useState } from "react";

// useSecond is the time now, ticking each second while since is set:
// the page's own clock ticks each minute.
export function useSecond(since: string | undefined): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (!since) return;
    setNow(Date.now());
    const t = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(t);
  }, [since]);
  return now;
}
