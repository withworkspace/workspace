import type { MissionEvent } from "../../api";
import { plain } from "../../ui/text";
import { viewEvent } from "../eventView";

// What the Log tab shows of the mission's log: everything, or one who's
// events (you, crew, the orchestrator), or its threads.
export const logFilters = [
  { id: "all", label: "Everything" },
  { id: "you", label: "You" },
  { id: "crew", label: "Crew" },
  { id: "orchestrator", label: "Orchestrator" },
  { id: "threads", label: "Threads" },
] as const;

export type LogFilter = (typeof logFilters)[number]["id"];

// logShows is whether an event is shown, by the filter and what's
// searched for.
export function logShows(e: MissionEvent, filter: LogFilter, q: string): boolean {
  const v = viewEvent(e);
  if (filter === "threads" && e.kind !== "thread") return false;
  if (filter !== "all" && filter !== "threads" && v.actor !== filter) return false;
  return !q || plain(e.text).toLowerCase().includes(q.toLowerCase());
}
