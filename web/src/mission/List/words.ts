import type { HeadlineGroup, Mission } from "../../api";

// How the missions list is put together and said.

// The groups the list is in, by whose court a mission is in (its
// headline's group): what needs you first.
const groups: { group: HeadlineGroup; title: string }[] = [
  { group: "needs-you", title: "Needs you" },
  { group: "in-progress", title: "In progress" },
  { group: "waiting", title: "Waiting" },
  { group: "up-next", title: "Up next" },
];

export const isPast = (m: Mission) => m.headline.group === "past";

export type MissionSection = { title: string; missions: Mission[]; past?: boolean };

// sectionsOf is the list's groups with missions in them, in order, and the
// past ones last when they're shown. A mission is in one group.
export function sectionsOf(missions: Mission[], showPast: boolean): MissionSection[] {
  const placed = new Set<string>();
  const out: MissionSection[] = [];
  for (const g of groups) {
    const ms = missions.filter((m) => !placed.has(m.slug) && m.headline.group === g.group);
    ms.forEach((m) => placed.add(m.slug));
    if (ms.length) out.push({ title: g.title, missions: ms });
  }
  if (showPast) {
    const ms = missions.filter((m) => !placed.has(m.slug) && isPast(m));
    if (ms.length) out.push({ title: "Past", missions: ms, past: true });
  }
  return out;
}

// onRoute is where a mission is on its route, in a few words: "Step 2 of
// 7", or "At Z" once every step's done; its whole sentence is the title.
export const onRoute = (at: string) => /^(?:Step|Stage) \d+ of \d+/.exec(at)?.[0] ?? (/^Every step done/.test(at) ? "At Z" : "");

