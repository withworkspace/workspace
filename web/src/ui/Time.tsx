import { age, clock, exact, relative, stamp, useNow } from "./when";

// Time is a time as text, with the exact time on hover: as it reads best
// from now (stamp), how long ago (relative), a compact age for a list's
// column, or the clock alone. Relative and age tick with the page's clock.
// It has no look of its own: it's in its place's words, or the class the
// page gives it.
export function Time({ iso, as = "stamp", className }: { iso: string; as?: "stamp" | "relative" | "age" | "clock"; className?: string }) {
  const at = useNow();
  if (!iso) return null;
  const text = as === "relative" ? relative(iso, at) : as === "age" ? age(iso, at) : as === "clock" ? clock(iso) : stamp(iso, at);
  return (
    <time dateTime={iso} title={exact(iso)} className={className}>
      {text}
    </time>
  );
}
