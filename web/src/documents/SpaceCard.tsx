import { type Space, type SpaceDoc, spaceTo } from "../api";
import { DocumentsSpaceCard } from "./DocumentsSpaceCard";
import { DocumentsSpaceCardFoot, type SpaceStands } from "./DocumentsSpaceCardFoot";
import { DocumentsSpaceCardKind } from "./DocumentsSpaceCardKind";
import { DocumentsSpaceCardKinds } from "./DocumentsSpaceCardKinds";
import { lastChanged } from "./happening";
import type { SpaceDetail } from "./useSpaceDetails";

// SpaceCard is a space as a card on Documents (a DocumentsSpaceCard): its
// kinds, each with how many, where they stand and which changed last
// (else its folders), and how it stands: fresh or behind, what's in
// flight, your drafts in it.
export function SpaceCard({ space, detail, drafts, id, selected, onHover }: { space: Space; detail?: SpaceDetail; drafts: number; id: string; selected: boolean; onHover: () => void }) {
  const docs = detail?.docs;
  const a = detail?.activity;
  const root = space.folder ?? "";
  // Its kinds; what's of none, by its top folder (or Other, beside kinds).
  const rows: { key: string; to: string; name: string; folder?: boolean; docs: SpaceDoc[]; stands?: [string, number][] }[] = [];
  for (const k of space.kinds) {
    const of = docs?.filter((d) => d.kind === k.name) ?? [];
    rows.push({
      key: `kind-${k.name}`,
      to: spaceTo(space.repo, { kind: k.name }),
      name: k.title,
      docs: of,
      stands: (k.states ?? []).map((s): [string, number] => [s, of.filter((d) => d.state === s).length]).filter(([, n]) => n > 0),
    });
  }
  const other = docs?.filter((d) => !(d.kind && space.kinds.some((k) => k.name === d.kind))) ?? [];
  if (space.kinds.length) {
    if (other.length) rows.push({ key: "other", to: spaceTo(space.repo, { folder: root }), name: "Other", docs: other });
  } else if (docs) {
    const tops = new Map<string, SpaceDoc[]>();
    for (const d of other) {
      const rel = d.path.slice(root.length);
      const top = rel.includes("/") ? rel.slice(0, rel.indexOf("/") + 1) : "";
      tops.set(top, [...(tops.get(top) ?? []), d]);
    }
    for (const [top, list] of [...tops].sort(([x], [y]) => (x === "" ? -1 : y === "" ? 1 : x.localeCompare(y))))
      rows.push({ key: `folder-${top}`, to: spaceTo(space.repo, { folder: root + top }), name: top || "(top)", folder: true, docs: list });
  }
  const stands: SpaceStands = !space.cloned ? { cloned: false, repo: space.repo } : a?.fresh.behind ? { behind: a.fresh.behind } : a ? { fetched: a.fresh.fetched } : undefined;
  return (
    <DocumentsSpaceCard
      id={id}
      to={spaceTo(space.repo)}
      label={space.label}
      color={space.color}
      folder={space.folder}
      repo={space.label !== space.repo ? space.repo : undefined}
      count={space.cloned ? `${space.count} ${space.count === 1 ? "document" : "documents"}` : "Not cloned"}
      selected={selected}
      onHover={onHover}
      foot={<DocumentsSpaceCardFoot stands={stands} inFlight={a?.inFlight.length ?? 0} drafts={drafts} />}
    >
      {space.cloned && (
        <DocumentsSpaceCardKinds reading={!docs && !rows.length ? space.folders.map((f) => `${f.name} ${f.count}`).join(" · ") || "Reading it…" : undefined}>
          {rows.map((r) => {
            const last = lastChanged(r.docs);
            return (
              <DocumentsSpaceCardKind
                key={r.key}
                to={r.to}
                name={r.name}
                folder={r.folder}
                count={r.docs.length}
                last={last && { title: last.title, at: last.changed! }}
                stands={r.stands && r.stands.length > 0 ? r.stands.map(([s, n]) => `${n} ${s}`).join(" · ") : undefined}
              />
            );
          })}
        </DocumentsSpaceCardKinds>
      )}
    </DocumentsSpaceCard>
  );
}
