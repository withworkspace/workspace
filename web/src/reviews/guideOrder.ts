import type { ReviewGuide } from "../api";
import type { Order } from "../review/files";
import { capital } from "../ui/text";

// guideOrder is a guide's groups as an order for the list of files.
export function guideOrder(g: ReviewGuide): Order | undefined {
  if (!g.guide) return undefined;
  const notes: Record<string, string> = {};
  // A file's line is written to follow its path; it's shown on a line of
  // its own, so it starts with a capital.
  for (const group of g.guide.groups) for (const f of group.files) if (f.note) notes[f.path] = capital(f.note);
  return {
    key: `${g.id}:${g.at}`,
    stale: g.moved,
    groups: g.guide.groups.map((x) => ({ name: x.name, why: x.why, skim: x.skim, files: x.files.map((f) => f.path) })),
    notes,
  };
}
