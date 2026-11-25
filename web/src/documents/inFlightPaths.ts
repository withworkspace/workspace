import type { SpaceActivity } from "../api";

// inFlightPaths is the paths in flight in a space, and in which capsules.
export function inFlightPaths(activity?: SpaceActivity): Map<string, string[]> {
  const out = new Map<string, string[]>();
  for (const f of activity?.inFlight ?? []) for (const d of f.docs) out.set(d.path, [...(out.get(d.path) ?? []), f.capsule]);
  return out;
}
