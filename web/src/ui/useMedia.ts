import { useEffect, useState } from "react";

// useMedia is whether a media query matches now, kept up to date.
export function useMedia(query: string): boolean {
  const [on, setOn] = useState(() => window.matchMedia(query).matches);
  useEffect(() => {
    const m = window.matchMedia(query);
    const changed = () => setOn(m.matches);
    m.addEventListener("change", changed);
    changed();
    return () => m.removeEventListener("change", changed);
  }, [query]);
  return on;
}
