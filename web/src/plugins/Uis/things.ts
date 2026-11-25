import type { V1 } from "@withworkspace/ui-lib/protocol";

// What ws's UIs for a plugin's thing that lives in a repo (ThingPromoteUi,
// ThingSaveUi) work out of what they're asked.

// said is why something failed, in its own words.
export const said = (err: unknown) => (err instanceof Error ? err.message : String(err));

// asked is the thing a page asked about, as its UI was given it.
export function asked(of: Record<string, unknown>): V1.ThingAsked {
  return { kind: String(of.kind ?? ""), ref: String(of.ref ?? ""), ...(typeof of.title === "string" && of.title ? { title: of.title } : {}) };
}

// grouped is where a thing would live in a place, in a folder of it:
// records:sketches/release-flow/ in cross-stack is
// records:sketches/cross-stack/release-flow/.
export function grouped(where: string, under: string): string {
  const group = under
    .split("/")
    .map((s) => s.trim())
    .filter(Boolean)
    .join("/");
  const at = where.replace(/\/$/, "").lastIndexOf("/");
  const cut = at < 0 ? where.indexOf(":") : at;
  return group && cut >= 0 ? `${where.slice(0, cut + 1)}${group}/${where.slice(cut + 1)}` : where;
}
