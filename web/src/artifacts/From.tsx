import { RefTag } from "../refs/RefTag";
import { refKey, refName } from "../refs/refs";
import { TLink } from "../ui/TLink";
import { useWorkspace } from "../workspace/store";
import { ArtifactFrom } from "./ArtifactFrom";
import { fromRef, isConversation, missionTalk } from "./shown";

// From is where a version came from: a conversation or a mission, drawn
// as its ref where this machine has it, its title where it hasn't (one
// made on another machine).
export function From({ from, title, inline }: { from: string; title?: string; inline?: boolean }) {
  const talk = missionTalk(from);
  const here = useWorkspace((w) => (isConversation(from) ? w.conversations.some((t) => t.id === from) : w.missions.some((m) => m.slug === (talk?.slug ?? from))));
  if (!here) return <ArtifactFrom from={from} title={title} />;
  const r = fromRef(from);
  if (talk)
    return (
      <span title={title ? `${talk.id} on ${talk.slug}: ${title}` : undefined}>
        <TLink to={`/missions/${talk.slug}/conversations/${talk.id}`} className="link">
          {talk.id}
        </TLink>{" "}
        on{" "}
        <RefTag refKey={refKey(r)} inline={inline}>
          {refName(r)}
        </RefTag>
      </span>
    );
  return (
    <RefTag refKey={refKey(r)} inline={inline}>
      {refName(r)}
    </RefTag>
  );
}
