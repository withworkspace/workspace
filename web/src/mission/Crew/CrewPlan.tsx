import type { TranscriptEntry } from "../../api";
import { Todos } from "../../ui/Todos";
import { MissionCrewPlan } from "./MissionCrewPlan";

// CrewPlan is crew's latest plan as the part pinned on top draws it: how
// many of its items are done, the one it's on, and the items.
export function CrewPlan({ todos, live }: { todos: NonNullable<TranscriptEntry["todos"]>; live: boolean }) {
  const done = todos.filter((t) => t.status === "completed").length;
  const now = todos.find((t) => t.status === "in_progress");
  return (
    <MissionCrewPlan done={done} of={todos.length} now={now && (now.active || now.content)}>
      <Todos todos={todos} live={live} />
    </MissionCrewPlan>
  );
}
