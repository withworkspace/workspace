import { useContext } from "react";
import { HoverCard } from "../ui/HoverCard";
import { Inline } from "../ui/Inline";
import { Status } from "../ui/Status";
import { crewStatus } from "../ui/statusOf";
import { Time } from "../ui/Time";
import { MissionCrewBadge } from "./MissionCrewBadge";
import { MissionCrewCard } from "./MissionCrewCard";
import { MissionData } from "./MissionData";

// CrewBadge is a mission's crew by name, which says more of it on hover
// where the page knows the mission: who it is, where it stands, what it
// last said.
export function CrewBadge({ id, live, large }: { id: string; live?: boolean; large?: boolean }) {
  const d = useContext(MissionData);
  const c = d?.crew.find((x) => x.id === id);
  const badge = <MissionCrewBadge id={id} live={live} large={large} />;
  if (!c) return badge;
  return (
    <HoverCard trigger={badge}>
      <MissionCrewCard
        badge={<MissionCrewBadge id={id} live={live} />}
        status={<Status s={crewStatus(c)} />}
        what={c.role === "briefing" ? "Briefing agent" : c.model ? `${c.harness} on ${c.model}` : c.harness}
        capsule={c.capsule}
        started={<Time iso={c.started} />}
        ended={c.endedAt && <Time iso={c.endedAt} />}
        said={(c.note || c.lastReply) && <Inline text={c.note || c.lastReply || ""} lines={3} />}
        to={d && `/missions/${d.slug}/crew/${c.id}`}
        more={c.ended ? "Read its transcript" : "Open its terminal"}
      />
    </HoverCard>
  );
}
