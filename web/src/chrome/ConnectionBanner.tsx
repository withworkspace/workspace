import { useEffect, useState } from "react";
import type { Connection } from "../api";
import { duration } from "../ui/when";
import { WindowLost } from "./WindowLost";

// ConnectionBanner says when the workspace's daemon has gone, and for how
// long: what's on the page may be stale until it's back. It's drawn as a
// WindowLost, which ticks each second.
export function ConnectionBanner({ connection }: { connection: Connection }) {
  const [lostAt, setLostAt] = useState<number>();
  const [, tick] = useState(0);
  useEffect(() => {
    if (connection === "lost") setLostAt((t) => t ?? Date.now());
    if (connection === "live") setLostAt(undefined);
  }, [connection]);
  useEffect(() => {
    if (!lostAt) return;
    const t = setInterval(() => tick((n) => n + 1), 1000);
    return () => clearInterval(t);
  }, [lostAt]);
  if (!lostAt) return null;
  return <WindowLost for={duration(lostAt)} />;
}
