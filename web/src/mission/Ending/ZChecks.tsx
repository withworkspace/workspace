import type { Conclusion } from "../../api";
import { MissionEndingZ } from "./MissionEndingZ";

// ZChecks is Z as it was checked, for a mission's ending (suggested or
// made): each part met or not, and how. A part nobody checked says so; Z
// nobody checked at all isn't drawn, as there's nothing to say of it.
export function ZChecks({ c }: { c: Conclusion }) {
  if (!c.z?.some((z) => z.met || z.how)) return null;
  return <MissionEndingZ parts={c.z.map((z) => ({ z: z.z, how: z.how, stands: z.met ? "met" : z.how ? "unmet" : "unchecked" }))} />;
}
