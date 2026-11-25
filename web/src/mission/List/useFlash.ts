import { useEffect, useRef, useState } from "react";
import type { Mission } from "../../api";

// useFlash is the missions whose headline changed since the last render,
// for a moment: a change isn't missed.
export function useFlash(missions: Mission[]): Set<string> {
  const before = useRef<Map<string, string> | null>(null);
  const [flashing, setFlashing] = useState<Set<string>>(new Set());
  useEffect(() => {
    // What's running while ws checks a step changes as it prints: it
    // flashes as each check starts, not with each line.
    const now = new Map(missions.map((m) => [m.slug, `${m.headline.kind}|${m.headline.checking ? m.headline.since : m.headline.text}`]));
    if (before.current) {
      const changed = [...now].filter(([slug, h]) => before.current!.has(slug) && before.current!.get(slug) !== h).map(([slug]) => slug);
      if (changed.length) {
        setFlashing(new Set(changed));
        const t = setTimeout(() => setFlashing(new Set()), 1700);
        before.current = now;
        return () => clearTimeout(t);
      }
    }
    before.current = now;
  }, [missions]);
  return flashing;
}
