import { useState } from "react";
import type { Artifact, MissionDetail } from "../../api";
import { Empty } from "../../ui/Empty";
import { GroupHead } from "../../ui/GroupHead";
import { MoreLink } from "../../ui/MoreLink";
import { CrewRow } from "../CrewRow";
import { crewElsewhere } from "../workRows";
import { Events } from "./Events";
import { MissionLogCrew, MissionLogHandoff } from "./MissionLogCrew";
import { MissionLogMoved } from "./MissionLogMoved";
import { MissionLogTools } from "./MissionLogTools";
import { type LogFilter, logFilters, logShows } from "./words";

// Log is a mission's Log tab: everything that's happened on it, newest
// first, by whom and what it says; then the crew who worked in none of
// its capsules (the rest are on Work), with their handoffs.
export function Log({
  d,
  selected,
  openArtifact,
  toWork,
}: {
  d: MissionDetail;
  selected: (key: string) => boolean;
  openArtifact: (a: Artifact) => void;
  toWork: () => void;
}) {
  const elsewhere = crewElsewhere(d);
  const [filter, setFilter] = useState<LogFilter>("all");
  const [q, setQ] = useState("");
  const events = d.events.filter((e) => logShows(e, filter, q));
  return (
    <>
      <section>
        <MissionLogTools filters={logFilters} filter={filter} onFilter={setFilter} q={q} onQ={setQ} />
        {events.length ? (
          <Events events={[...events].reverse()} />
        ) : (
          <Empty icon="search" title={d.events.length ? "Nothing matches" : "Nothing yet"}>
            {d.events.length ? "Try another filter or search." : "Mission activity shows here."}
          </Empty>
        )}
      </section>
      {d.crew.length > 0 && (
        <MissionLogMoved>
          {elsewhere.length < d.crew.length && (
            <>
              Crew are on{" "}
              <MoreLink onClick={toWork} keys="3">
                Work
              </MoreLink>, by capsule.{" "}
            </>
          )}
          {elsewhere.length > 0 && "These worked outside the mission's capsules."}
        </MissionLogMoved>
      )}
      {elsewhere.length > 0 && (
        <section>
          <GroupHead count={elsewhere.length}>
            Crew
          </GroupHead>
          {[...elsewhere].reverse().map((c) => (
            <MissionLogCrew key={c.id}>
              <CrewRow c={c} slug={d.slug} detail />
              {c.handoff && (
                <MissionLogHandoff id={`item-h:${c.id}`} selected={selected(`h:${c.id}`)} onOpen={() => openArtifact({ ref: c.handoff!, readable: true } as Artifact)}>
                  {" Read "}
                  {c.id}'s handoff
                </MissionLogHandoff>
              )}
            </MissionLogCrew>
          ))}
        </section>
      )}
    </>
  );
}
