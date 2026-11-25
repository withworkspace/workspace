import type { CapsuleDetail, MissionDetail } from "../../api";
import { Icon } from "../../ui/Icon";
import { CrewBadge } from "../parts";
import type { WorkCapsule } from "../workRows";
import { MissionWorkLines } from "./MissionWorkLines";
import { MissionWorkRow } from "./MissionWorkRow";
import { MissionWorkState } from "./MissionWorkState";
import { useSummary } from "./useSummary";

// A capsule in a line: its repo (and its own name, when that isn't the
// mission's), how much has changed, who's in it now, and where it stands.
export function WorkCapsuleRow({ d, x, selected, select, label, review, seen }: { d: MissionDetail; x: WorkCapsule; selected: boolean; select: (key: string) => void; label: (repo: string) => string; review: (c: CapsuleDetail) => void; seen?: string }) {
  const { c } = x;
  // What's known of it: it's read when it's the capsule shown.
  const { value: sum } = useSummary(d.slug, c, { seen, kept: !selected });
  const loose = sum?.changed.filter((f) => f.uncommitted).length ?? 0;
  // A crew who's at work is in the chip already.
  const here = x.live && !x.state.live ? x.live : undefined;
  return (
    <MissionWorkRow
      id={`item-${x.key}`}
      icon={<Icon name="box" size={14} />}
      name={label(c.repo)}
      slug={c.name !== d.slug ? c.name : undefined}
      stat={sum && (sum.commitCount > 0 || loose > 0) ? sum.commitCount > 0 ? <MissionWorkLines additions={sum.additions} deletions={sum.deletions} /> : `${loose} not committed` : undefined}
      end={
        <>
          {here && <CrewBadge id={here.id} live />}
          <MissionWorkState {...x.state} keys="o" />
        </>
      }
      selected={selected}
      quiet={x.where !== "hand"}
      title={`${x.id}${c.branch && c.branch !== c.name ? ` (${c.branch})` : ""}`}
      onSelect={() => select(x.key)}
      onOpen={() => review(c)}
    />
  );
}
