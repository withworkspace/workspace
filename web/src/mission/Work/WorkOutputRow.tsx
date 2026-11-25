import type { Artifact } from "../../api";
import { Icon } from "../../ui/Icon";
import { MissionOutputState } from "../MissionOutputState";
import { MissionOutputThreads } from "../MissionOutputThreads";
import { CrewBadge } from "../CrewBadge";
import { artifactTitle, isArtifact } from "../outputs";
import { outputKey } from "../workRows";
import { MissionWorkRow } from "./MissionWorkRow";

// An output in a line: who left it (a handoff's crew) or what it is, what
// it says, and how it stands or when it was made. made is one in the list
// of what a capsule made: the row the keys go to is the list's own.
export function WorkOutputRow({ a, selected, select, open, made }: { a: Artifact; selected: boolean; select: (key: string) => void; open: (a: Artifact) => void; made?: boolean }) {
  const { title, desc } = artifactTitle(a);
  const key = outputKey(a);
  const handoff = a.kind === "handoff";
  const stands = isArtifact(a) || (a.state !== "private" && a.state !== "draft");
  return (
    <MissionWorkRow
      id={made ? undefined : `item-${key}`}
      icon={handoff && a.by ? <CrewBadge id={a.by} /> : <Icon name={handoff ? "arrow-right" : "file"} size={14} />}
      name={handoff ? (desc ?? title) : title}
      end={
        <>
          {a.threads && <MissionOutputThreads t={a.threads} />}
          {stands && <MissionOutputState state={a.state} />}
        </>
      }
      when={stands ? undefined : a.added}
      selected={selected}
      output
      plain={handoff}
      title={desc ?? title}
      onSelect={() => select(key)}
      onOpen={() => open(a)}
    />
  );
}
