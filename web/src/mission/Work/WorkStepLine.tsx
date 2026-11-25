import type { MissionDetail, Stage } from "../../api";
import { Chip } from "../../ui/Chip";
import { CrewBadge } from "../CrewBadge";
import type { WorkCapsule } from "../workRows";
import { MissionWorkStep } from "./MissionWorkStep";
import { MissionWorkStepBundle } from "./MissionWorkStepBundle";
import { bundleLink } from "./words";

// A step that ran in the capsule, in a line: done, now or to come, who's
// on it, and the bundles that show it. How it passed is said on hover.
export function WorkStepLine({ d, x, s }: { d: MissionDetail; x: WorkCapsule; s: Stage }) {
  const bundles = x.bundles.filter((b) => b.step === s.n && !b.replacedBy);
  return (
    <MissionWorkStep n={s.n} title={s.title} state={s.state} hint={s.why ? `${s.title}: ${s.why}` : s.text}>
      {s.crew?.length ? s.crew.map((id) => <CrewBadge key={id} id={id} live={d.crew.some((k) => k.id === id && !k.ended)} />) : s.who && s.who !== "crew" && <Chip tone="quiet">{s.who}</Chip>}
      {s.type && s.type !== s.title && <span className="dim">{s.type}</span>}
      {s.state === "current" && <Chip tone="info">now</Chip>}
      {bundles.map((b) => (
        <MissionWorkStepBundle key={b.id} id={b.id} to={bundleLink(d.slug, b.id)} title={`${b.id}${b.verdict ? `: ${b.verdict}` : ""}${b.stale ? ", of a commit the capsule has moved on from" : ""}`} />
      ))}
    </MissionWorkStep>
  );
}
