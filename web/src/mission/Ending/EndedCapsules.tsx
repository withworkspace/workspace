import type { EndedCapsule } from "../../api";
import { MissionEndingCapsules } from "./MissionEndingCapsules";

// EndedCapsules are a mission's capsules that ended alone, drawn as
// MissionEndingCapsules; with none, nothing.
export function EndedCapsules({ list }: { list?: EndedCapsule[] }) {
  if (!list?.length) return null;
  return <MissionEndingCapsules capsules={list.map((c) => ({ repo: c.repo, name: c.name, why: c.why, to: c.to, dropped: c.ending === "dropped" }))} />;
}
