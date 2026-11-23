import type { MissionDetail } from "../api";
import { useMemo } from "react";
import { duration, stamp, useNow } from "../ui/when";
import { crewHue } from "./crewHue";
import type { TimelineBar, TimelineMoment, TimelineTick } from "./MissionTimeline";

// How near two moments are (in hundredths of the strip) before they're
// one tick.
const tickApart = 0.4;

// timelineOf is a mission's life as its timeline draws it (MissionTimeline):
// when each crew worked, as bars in lanes, and the moments that mattered
// (stages accepted, reviews, decisions, what was asked) as ticks, from its
// start to now; moments too close to point at apart share a tick.
export function timelineOf(d: MissionDetail, now: number): { bars: TimelineBar[]; ticks: TimelineTick[]; lanes: number } {
  const start = new Date(d.created).getTime();
  const lastEvent = d.events.length ? new Date(d.events[d.events.length - 1].time).getTime() : start;
  const end = d.state === "archived" || d.state === "landed" ? Math.max(lastEvent, start + 60_000) : now;
  const span = Math.max(end - start, 60_000);
  const at = (when: string | number) => Math.min(100, Math.max(0, ((new Date(when).getTime() - start) / span) * 100));
  // When each crew ran: from the log, start (or resume) to end, so crew
  // resumed days later show as two runs, not one long one; crew with
  // nothing in the log, by when they started and ended.
  const spans: { c: (typeof d.crew)[number]; from: number; to: number }[] = [];
  for (const c of d.crew) {
    const mine = d.events.filter((e) => e.by === c.id && /^crew_(started|resumed|ended)$/.test(e.kind));
    if (!mine.length) {
      const from = new Date(c.started).getTime();
      spans.push({ c, from, to: c.endedAt ? new Date(c.endedAt).getTime() : c.ended ? from : now });
      continue;
    }
    let open: number | undefined;
    for (const e of mine) {
      const t = new Date(e.time).getTime();
      if (e.kind === "crew_ended") {
        spans.push({ c, from: open ?? t, to: t });
        open = undefined;
      } else open ??= t;
    }
    if (open !== undefined) spans.push({ c, from: open, to: c.ended ? open : now });
  }
  // In lanes, so crew running at once don't cover each other.
  const lanes: number[] = [];
  const bars = spans
    .sort((a, b) => a.from - b.from)
    .map((sp, i): TimelineBar => {
      let lane = lanes.findIndex((free) => free <= sp.from);
      if (lane < 0) lane = lanes.push(0) - 1;
      lanes[lane] = sp.to;
      const over = sp.to < now - 60_000;
      return {
        key: `${sp.c.id}-${i}`,
        hue: crewHue(sp.c.id),
        from: at(sp.from),
        width: at(sp.to) - at(sp.from),
        lane,
        live: !(sp.c.ended || over),
        title: `${sp.c.id}: ${stamp(sp.from)}${over ? ` to ${stamp(sp.to)} (${duration(sp.from, sp.to)})` : ", running now"}`,
      };
    });
  const ticks: TimelineTick[] = [];
  for (const e of d.events) {
    const m = momentOf(e, d.plan ? "Step" : "Stage");
    if (!m) continue;
    const x = at(e.time);
    const last = ticks[ticks.length - 1];
    if (last && x - last.at < tickApart) last.moments.push(m);
    else ticks.push({ at: x, moments: [m] });
  }
  return { bars, ticks, lanes: lanes.length };
}

function momentOf(e: MissionDetail["events"][number], step: string): TimelineMoment | undefined {
  const m = (kind: TimelineMoment["kind"], what: string): TimelineMoment => ({ kind, what, when: stamp(e.time), text: e.text });
  if (e.kind === "stage" && /accepted/.test(e.text)) return m("done", `${step} accepted`);
  if (e.kind === "review") return m("you", "Reviewed");
  if (e.kind === "decision") return m("decided", "Decided");
  if (e.kind === "flag" || e.kind === "crew_asked") return m("ask", "Asked");
  return undefined;
}

// useTimeline is a mission's timeline as it stands now, kept up as time
// passes (crew still running grow); nothing while there's no mission, or
// nothing to show of it.
export function useTimeline(d: MissionDetail | undefined) {
  const now = useNow();
  return useMemo(() => {
    if (!d) return undefined;
    const t = timelineOf(d, now);
    return t.bars.length || t.ticks.length ? t : undefined;
  }, [d, now]);
}
