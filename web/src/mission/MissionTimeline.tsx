import { useRef } from "react";
import { RefContext } from "../refs/context";
import { cx } from "../ui/cx";
import { FloatingCard } from "../ui/FloatingCard";
import { useHoverDelay } from "../ui/useHoverDelay";
import { type Moment, MissionMomentCard } from "./MissionMomentCard";
import css from "./MissionTimeline.module.css";

// A run of crew's, as a bar: where it starts and how wide it is (in
// hundredths of the strip), which lane it's in, its crew's colour, whether
// it's running now, and what it says when pointed at.
export type TimelineBar = { key: string; hue: number; from: number; width: number; lane: number; live?: boolean; title?: string };
export type TimelineMoment = Moment;
// A tick: where it is along the strip, and the moments it stands for.
export type TimelineTick = { at: number; moments: Moment[] };

// Which of a tick's kinds it's drawn as: what's asked of you first.
const order: Moment["kind"][] = ["ask", "you", "done", "decided"];
const kinds: Record<Moment["kind"], string> = { done: css.done, you: css.you, decided: css.decided, ask: css.ask };

// How many lanes of bars are drawn, at most.
const most = 3;

// MissionTimeline is a mission's life in a strip along the top of its
// page: when each crew worked, as thin bars in their colours (quietly; the
// one running now less so), and the moments that mattered as ticks over
// them, from its start to now. A tick says what happened then on hover.
//
// It draws what it's given (mission/timeline.ts makes it of a mission).
// With nothing given it's the strip alone, for a page that's loading.
export function MissionTimeline({ bars = [], ticks = [], lanes = 1 }: { bars?: TimelineBar[]; ticks?: TimelineTick[]; lanes?: number }) {
  const hover = useHoverDelay<number>();
  const els = useRef(new Map<number, HTMLElement>());
  const shown = hover.shown === undefined ? undefined : ticks[hover.shown];
  const anchor = hover.shown === undefined ? undefined : els.current.get(hover.shown);
  return (
    <div className={css.timeline} style={{ "--lanes": Math.max(1, Math.min(lanes, most)) } as React.CSSProperties} aria-label="The mission over time" data-mission-timeline="">
      <div className={css.track}>
        {bars
          .filter((b) => b.lane < most)
          .map((b) => (
            <span
              key={b.key}
              className={cx(css.bar, b.live && css.live)}
              style={{ "--hue": b.hue, "--lane": b.lane, left: `${b.from}%`, width: `max(3px, ${b.width}%)` } as React.CSSProperties}
              title={b.title}
            />
          ))}
        {ticks.map((t, i) => (
          <span
            key={i}
            ref={(el) => {
              if (el) els.current.set(i, el);
              else els.current.delete(i);
            }}
            className={cx(css.mark, kinds[order.find((k) => t.moments.some((m) => m.kind === k)) ?? "done"])}
            style={{ left: `${t.at}%` }}
            tabIndex={0}
            aria-label={t.moments.map((m) => `${m.when}: ${m.what}`).join("; ")}
            onMouseEnter={() => hover.over(i)}
            onMouseLeave={hover.out}
            onFocus={() => hover.over(i)}
            onBlur={hover.out}
          />
        ))}
      </div>
      {shown && anchor && (
        <FloatingCard anchor={anchor} className={css.card} onEnter={hover.stay} onLeave={hover.out}>
          {/* Inside a card, refs are words: no cards in cards. */}
          <RefContext.Provider value={undefined}>
            <MissionMomentCard moments={shown.moments} />
          </RefContext.Provider>
        </FloatingCard>
      )}
    </div>
  );
}
