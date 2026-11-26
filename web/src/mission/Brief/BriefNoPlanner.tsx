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
    <MissionBriefPlanner label="Planner" onFold={fold} foot={field}>
      {briefing ? (
        <>
          <Text as="p" dim>Written without a planner. Brief the mission to have a planner propose a brief and plan here.</Text>
          <div>
            <Button type="button" onClick={briefIt} disabled={busy} aria-busy={busy}>
              <Icon name="pencil" /> Brief mission
            </Button>
          </div>
        </>
      ) : (
        <Text as="p" dim>Briefed without a conversation. Say what should change and a planner revises the plan. Finished steps stay as they are. Changes to a checkpoint or Z need your approval; others apply directly.</Text>
      )}
    </MissionBriefPlanner>
  );
}
