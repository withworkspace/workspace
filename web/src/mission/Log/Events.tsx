import { Fragment } from "react";
import type { MissionEvent } from "../../api";
import { Inline } from "../../ui/Inline";
import { dayKey, dayLabel, useNow } from "../../ui/when";
import { CrewBadge } from "../CrewBadge";
import { viewEvent } from "../eventView";
import { MissionLogActor } from "./MissionLogActor";
import { MissionLogDay, MissionLogEvent, MissionLogEvents } from "./MissionLogEvents";

// Events is a stretch of the log: by day, each line who and what, its time
// on the right. lines clamps each (a summary); without, they're whole.
export function Events({ events, lines, days = true }: { events: MissionEvent[]; lines?: number; days?: boolean }) {
  const now = useNow();
  const spansDays = days && new Set(events.map((e) => dayKey(e.time))).size > 1;
  let lastDay = "";
  return (
    <MissionLogEvents>
      {events.map((e, i) => {
        const v = viewEvent(e);
        const day = dayKey(e.time);
        const head = spansDays && day !== lastDay;
        lastDay = day;
        return (
          <Fragment key={i}>
            {head && <MissionLogDay>{dayLabel(e.time, now)}</MissionLogDay>}
            <MissionLogEvent
              actor={v.actor === "crew" && v.crew ? <CrewBadge id={v.crew} /> : <MissionLogActor actor={v.actor} who={v.who || "ws"} />}
              who={v.who && v.actor !== "crew" ? v.who : undefined}
              title={v.title}
              quiet={v.actor === "ws"}
              iso={e.time}
              as={spansDays ? "clock" : "stamp"}
            >
              <Inline text={v.text} lines={lines} />
            </MissionLogEvent>
          </Fragment>
        );
      })}
    </MissionLogEvents>
  );
}
