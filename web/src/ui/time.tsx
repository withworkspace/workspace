import { useSyncExternalStore } from "react";

// Times, one way everywhere: relative under a day ("5m ago"), then
// "Yesterday 23:13", "Mon 16:20", "5 Nov, 16:20", and "5 Nov 2025" in other
// years. Every one has the exact time on hover, and relative ones tick.

// The clock everything relative reads: one timer for the page, ticking
// each minute (and when the page shows again), so "2m ago" doesn't go
// stale on a quiet page.
let now = Date.now();
const listeners = new Set<() => void>();
let timer: ReturnType<typeof setInterval> | undefined;

function tick() {
  now = Date.now();
  listeners.forEach((l) => l());
}

function subscribe(l: () => void) {
  listeners.add(l);
  if (!timer) {
    timer = setInterval(tick, 30_000);
    document.addEventListener("visibilitychange", tick);
  }
  return () => {
    listeners.delete(l);
    if (!listeners.size && timer) {
      clearInterval(timer);
      timer = undefined;
      document.removeEventListener("visibilitychange", tick);
    }
  };
}

export function useNow(): number {
  return useSyncExternalStore(subscribe, () => now);
}

const DAY = 86_400_000;

function startOfDay(t: number) {
  const d = new Date(t);
  d.setHours(0, 0, 0, 0);
  return d.getTime();
}

const daysBetween = (a: number, b: number) => Math.round((startOfDay(b) - startOfDay(a)) / DAY);

export const clock = (iso: string | number) => new Date(iso).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });

// stamp is a point in time as it reads best from now: the clock today,
// the day and clock this week, the date after.
export function stamp(iso: string | number, at = Date.now()): string {
  const t = new Date(iso).getTime();
  const days = daysBetween(t, at);
  if (days === 0) return clock(t);
  if (days === 1) return `Yesterday ${clock(t)}`;
  if (days > 1 && days < 7) return `${new Date(t).toLocaleDateString([], { weekday: "short" })} ${clock(t)}`;
  if (new Date(t).getFullYear() === new Date(at).getFullYear()) return `${shortDate(t)}, ${clock(t)}`;
  return new Date(t).toLocaleDateString([], { day: "numeric", month: "short", year: "numeric" });
}

const shortDate = (t: number) => new Date(t).toLocaleDateString([], { day: "numeric", month: "short" });

// relative is how long ago: "just now", "5m ago", "3h ago", then as stamp
// without the clock ("Yesterday", "Mon", "5 Nov").
export function relative(iso: string | number, at = Date.now()): string {
  const t = new Date(iso).getTime();
  const s = (at - t) / 1000;
  if (s < 45) return "just now";
  if (s < 3600) return `${Math.max(1, Math.round(s / 60))}m ago`;
  if (s < 6 * 3600 || daysBetween(t, at) === 0) return `${Math.round(s / 3600)}h ago`;
  const days = daysBetween(t, at);
  if (days === 1) return "yesterday";
  if (days < 7) return new Date(t).toLocaleDateString([], { weekday: "long" });
  if (new Date(t).getFullYear() === new Date(at).getFullYear()) return shortDate(t);
  return new Date(t).toLocaleDateString([], { day: "numeric", month: "short", year: "numeric" });
}

// age is a compact age for a list's column: "now", "5m", "3h", "4d", "6w".
export function age(iso: string | number, at = Date.now()): string {
  const s = (at - new Date(iso).getTime()) / 1000;
  if (s < 60) return "now";
  if (s < 3600) return `${Math.floor(s / 60)}m`;
  if (s < 86400) return `${Math.floor(s / 3600)}h`;
  if (s < 86400 * 14) return `${Math.floor(s / 86400)}d`;
  return `${Math.floor(s / (86400 * 7))}w`;
}

// exact is the whole time, for a tooltip.
export const exact = (iso: string | number) =>
  new Date(iso).toLocaleString([], { weekday: "short", day: "numeric", month: "long", year: "numeric", hour: "2-digit", minute: "2-digit" });

// dayLabel names the day a time is on, for grouping by day.
export function dayLabel(iso: string | number, at = Date.now()): string {
  const t = new Date(iso).getTime();
  const days = daysBetween(t, at);
  if (days === 0) return "Today";
  if (days === 1) return "Yesterday";
  const sameYear = new Date(t).getFullYear() === new Date(at).getFullYear();
  return new Date(t).toLocaleDateString([], { weekday: "long", day: "numeric", month: "long", ...(sameYear ? {} : { year: "numeric" }) });
}

export const dayKey = (iso: string | number) => new Date(startOfDay(new Date(iso).getTime())).toISOString();

// duration is how long from a to b (or now): "30s", "4m", "2h 5m", "3d".
export function duration(from: string | number, to: string | number = Date.now()): string {
  const s = Math.max(0, Math.round((new Date(to).getTime() - new Date(from).getTime()) / 1000));
  if (s < 60) return `${s}s`;
  if (s < 3600) return `${Math.round(s / 60)}m`;
  if (s < 86400) {
    const h = Math.floor(s / 3600);
    const m = Math.round((s % 3600) / 60);
    return m ? `${h}h ${m}m` : `${h}h`;
  }
  return `${Math.round(s / 86400)}d`;
}

// Time is a time as text, with the exact time on hover. Relative and age
// tick with the page's clock.
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
