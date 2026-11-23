import { useEffect, useState } from "react";
import { getSpace, getSpaceActivity, type Space, type SpaceActivity, type SpaceDoc, spaceTo } from "../api";
import { Icon } from "../ui/Icon";
import { Time } from "../ui/Time";
import { TLink } from "../ui/TLink";
import { lastChanged } from "./happening";
import { Chip } from "../ui/Chip";
import { Count } from "../ui/Count";

// A space as a card on Documents: its kinds, each with how many, where
// they stand and which changed last (else its folders), and how it
// stands: fresh or behind, what's in flight, your drafts in it.

// What Documents reads of each space beyond its count: its documents and
// what's happening in it, a space's card filled in as it comes; read again
// when ws has found a space other than it said (seen: the workspace's
// spacesSeen). What's happening that's not been read yet isn't shown.
export type SpaceDetail = { docs?: SpaceDoc[]; activity?: SpaceActivity };

export function useSpaceDetails(spaces: Space[], seen?: string): Map<string, SpaceDetail> {
  const [details, setDetails] = useState<Map<string, SpaceDetail>>(new Map());
  const key = `${spaces.map((s) => s.repo).join("\n")}\n${seen ?? ""}`;
  useEffect(() => {
    let stale = false;
    const put = (repo: string, d: SpaceDetail) => !stale && setDetails((m) => new Map(m).set(repo, { ...m.get(repo), ...d }));
    for (const s of spaces) {
      if (!s.cloned) continue;
      getSpace(s.repo).then(
        (r) => put(s.repo, { docs: r.docs }),
        () => {},
      );
      getSpaceActivity(s.repo).then(
        (a) => !a.unread && put(s.repo, { activity: a }),
        () => {},
      );
    }
    return () => {
      stale = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key]);
  return details;
}

export function SpaceCard({ space, detail, drafts, id, selected, onHover }: { space: Space; detail?: SpaceDetail; drafts: number; id: string; selected: boolean; onHover: () => void }) {
  const docs = detail?.docs;
  const a = detail?.activity;
  const root = space.folder ?? "";
  // Its kinds; what's of none, by its top folder (or Other, beside kinds).
  const rows: { key: string; to: string; name: string; mono?: boolean; docs: SpaceDoc[]; stands?: [string, number][] }[] = [];
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
      rows.push({ key: `folder-${top}`, to: spaceTo(space.repo, { folder: root + top }), name: top || "(top)", mono: true, docs: list });
  }
  return (
    <article id={id} className={`space-card${selected ? " selected" : ""}`} onMouseMove={onHover}>
      <TLink to={spaceTo(space.repo)} className="space-card-head">
        <span className="repo-dot" style={{ background: space.color }} />
        <span className="space-card-title">{space.label}</span>
        {space.folder && <span className="mono dim">{space.folder}</span>}
        {space.label !== space.repo && <span className="mono dim space-card-repo">{space.repo}</span>}
        <span className="space-card-count">{space.cloned ? `${space.count} ${space.count === 1 ? "document" : "documents"}` : "Not cloned"}</span>
        <Icon name="chevron-right" size={14} className="dim" />
      </TLink>
      {space.cloned && (
        <ul className="space-card-kinds">
          {rows.map((r) => {
            const last = lastChanged(r.docs);
            return (
              <li key={r.key}>
                <TLink to={r.to}>
                  <span className={`space-card-kind${r.mono ? " mono" : ""}`}>{r.name}</span> <Count>{r.docs.length}</Count>
                  {last && (
                    <span className="space-card-last" title={`Changed last: ${last.title}`}>
                      {last.title} · <Time iso={last.changed!} as="age" />
                    </span>
                  )}
                  {r.stands && r.stands.length > 0 && <span className="space-card-stands">{r.stands.map(([s, n]) => `${n} ${s}`).join(" · ")}</span>}
                </TLink>
              </li>
            );
          })}
          {!docs && !rows.length && (
            <li className="dim space-card-reading">
              {space.folders.map((f) => `${f.name} ${f.count}`).join(" · ") || "Reading it…"}
            </li>
          )}
        </ul>
      )}
      <footer className="space-card-foot">
        <span>
          {!space.cloned ? (
            <>
              <Icon name="box" size={12} /> <span className="mono">ws setup {space.repo}</span> clones it
            </>
          ) : a?.fresh.behind ? (
            <span className="space-card-behind">
              <Icon name="alert" size={12} /> {a.fresh.behind} {a.fresh.behind === 1 ? "commit" : "commits"} behind
            </span>
          ) : a ? (
            <>
              <Icon name="check" size={12} /> Up to date
              {a.fresh.fetched && (
                <>
                  {" "}
                  · fetched <Time iso={a.fresh.fetched} as="age" /> ago
                </>
              )}
            </>
          ) : null}
        </span>
        {!!a?.inFlight.length && <Chip tone="progress" className="foot-chip">{a.inFlight.length} in flight</Chip>}
        {drafts > 0 && <Chip tone="attention" className="foot-chip">{drafts === 1 ? "1 draft of yours" : `${drafts} drafts of yours`}</Chip>}
      </footer>
    </article>
  );
}
