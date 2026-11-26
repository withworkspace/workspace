import type { ReactNode } from "react";
import { Button } from "../../ui/Button";
import { Icon } from "../../ui/Icon";
import { MissionBriefPlanner } from "./MissionBriefPlanner";
import { Text } from "../../ui/Text";

// BriefNoPlanner is where a mission that was briefed without a
// conversation gets a planner: briefing, by having it briefed; under way,
// by saying what should change in the plan, which ws puts to a planner of
// its own.
export function BriefNoPlanner({ briefing, busy, briefIt, field, fold }: { briefing: boolean; busy: boolean; briefIt: () => void; field: ReactNode; fold?: () => void }) {
  return (
    <MissionBriefPlanner label="The planner" onFold={fold} foot={field}>
      {briefing ? (
        <>
          <Text as="p" dim>It's been written without a planner. Have it briefed, and a planner reads round it, asks what it needs to, and proposes its brief and a plan here.</Text>
          <div>
            <Button type="button" onClick={briefIt} disabled={busy} aria-busy={busy}>
              <Icon name="pencil" /> Brief it
            </Button>
          </div>
        </>
      ) : (
        <Text as="p" dim>It was briefed without a conversation. Say what should change, and a planner reads the mission and revises its plan. The steps behind the mission stay as they are. A change to a checkpoint or Z comes back to you to accept; anything else applies.</Text>
      )}
    </MissionBriefPlanner>
  );
}
