import { useMemo, useState } from "react";
import { type DocHit, docTo, type SpaceDoc, type SpaceKind } from "../api";
import { Icon } from "../ui/Icon";
import { Option, Select } from "../ui/Select";
import { Time } from "../ui/time";
import { TLink } from "../ui/TLink";
import { Chip } from "../ui/Chip";
import { Count } from "../ui/Count";
import { GroupHead } from "../ui/GroupHead";
import { Row, RowLine, RowMain, RowTitle, Rows, rowLinkClass } from "../ui/Rows";
import { Segment, Segmented } from "../ui/Segmented";
import { useThingMenu } from "../shell/ThingMenu";

// The segment for every state: no state is named this.
const everyStand = "(all)";

// A space's library: a list for each of the workspace's kinds of document
// whose path is in it, each with its number or date, its title, where it
// stands (the folder it's in: proposed, accepted) and who changed it
// last. Sorted by any of them, and filtered by where it stands.

export type Sort = "made" | "title" | "stands" | "changed";

// sorted is a kind's documents in the order asked: by what made them (a
// number, the oldest first; a date, the newest first), title, where they
// stand (in the order of a document's life), or when they last changed.
export function sorted(docs: SpaceDoc[], by: Sort, states: string[] = []): SpaceDoc[] {
  const made = (d: SpaceDoc) => d.number ?? d.date ?? "";
  const dated = docs.some((d) => d.date && !d.number);
  const cmp: Record<Sort, (a: SpaceDoc, b: SpaceDoc) => number> = {
    made: (a, b) => (dated ? made(b).localeCompare(made(a)) : made(a).localeCompare(made(b), undefined, { numeric: true })),
    title: (a, b) => a.title.localeCompare(b.title, undefined, { numeric: true }),
    stands: (a, b) => states.indexOf(a.state ?? "") - states.indexOf(b.state ?? ""),
    changed: (a, b) => (b.changed ?? "").localeCompare(a.changed ?? ""),
  };
  return [...docs].sort((a, b) => cmp[by](a, b) || a.path.localeCompare(b.path));
}

// A list's rows can be picked (a space's page shows the one picked beside
// its list): a click picks it, a double click (or ⏎) opens it, and a
// modified click is the browser's. A search's hit says where in it.
export type Picking = {
  picked?: string; // its path
  onPick: (d: SpaceDoc) => void;
  onOpen: (d: SpaceDoc) => void;
  hits?: Map<string, DocHit>;
};

export function KindList({
  repo,
  kind,
  docs,
  at,
  index,
  setIndex,
  flights,
  picking,
}: {
  repo: string;
  kind: SpaceKind;
  docs: SpaceDoc[]; // as they're shown
  at: number; // the index of its first row, among the page's
  index: number;
  setIndex?: (i: number) => void; // the cursor follows the pointer
  flights?: Map<string, string[]>; // the paths in flight, and in which capsules
  picking?: Picking;
}) {
  const menu = useThingMenu();
  // The column of numbers or dates, if any of them has one.
  const made = docs.some((d) => d.number || d.date);
  return (
    <Rows className="library" role="listbox" aria-label={kind.title}>
      {docs.map((d, j) => {
        const i = at + j;
        const hit = picking?.hits?.get(d.path);
        return (
          <Row
            selected={i === index} picked={picking?.picked === d.path}
            key={d.path}
            id={libraryID(d.path)}
            role="option"
            aria-selected={i === index}
            onMouseMove={setIndex && (() => i !== index && setIndex(i))}
          >
            <TLink to={docTo(`${repo}:${d.path}`)} className={rowLinkClass()} onContextMenu={picking ? undefined : menu(docTo(`${repo}:${d.path}`))} onClick={picking && pickOn(picking, d)} onDoubleClick={picking && (() => picking.onOpen(d))}>
              {made && <span className="lib-made mono dim">{d.number ?? d.date ?? ""}</span>}
              <RowMain>
                <RowLine>
                  <RowTitle>{d.title}</RowTitle>
                  {d.state && kind.states && <Chip tone={d.state === kind.states[0] ? "quiet" : "done"}>{d.state}</Chip>}
                  {flights?.has(d.path) && <InFlightChip capsules={flights.get(d.path)!} />}
                </RowLine>
                {hit && <HitLine hit={hit} />}
              </RowMain>
              {d.changed && (
                <span className="lib-changed dim" title={`Last changed by ${d.changedBy ?? "someone"}`}>
                  {d.changedBy?.split(" ")[0]} · <Time iso={d.changed} as="age" />
                </span>
              )}
            </TLink>
          </Row>
        );
      })}
    </Rows>
  );
}

// pickOn is a row's click, where its list's rows are picked: a plain click
// picks it; a modified one opens it as a link does.
export const pickOn = (picking: Picking, d: SpaceDoc) => (e: React.MouseEvent) => {
  if (e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
  e.preventDefault();
  if (e.detail < 2) picking.onPick(d);
};

// HitLine is where a search found a document: the section, and its words
// with those found marked.
export function HitLine({ hit }: { hit: DocHit }) {
  return (
    <RowLine className="doc-hit-line">
      {hit.heading && <span className="doc-hit-heading">{hit.heading} ›</span>}
      <span className="doc-hit-snippet">{hit.snippet.map((p, j) => (p.mark ? <mark key={j}>{p.text}</mark> : <span key={j}>{p.text}</span>))}</span>
    </RowLine>
  );
}

// KindHead is a kind's list's heading: its title and how many (of them,
// how many a search found), where they stand, and how it's sorted.
export function KindHead({
  kind,
  docs,
  sort,
  setSort,
  stand,
  setStand,
  found,
}: {
  kind: SpaceKind;
  docs: SpaceDoc[]; // all of the kind's
  sort: Sort;
  setSort: (s: Sort) => void;
  stand: string; // "" for all
  setStand: (s: string) => void;
  found?: number; // of them, how many a search found
}) {
  const counts = useMemo(() => {
    const c = new Map<string, number>();
    for (const d of docs) c.set(d.state ?? "", (c.get(d.state ?? "") ?? 0) + 1);
    return c;
  }, [docs]);
  const states = kind.states ?? [];
  return (
    <GroupHead className="library-head">
      {kind.title} <Count>{found === undefined ? docs.length : `${found} of ${docs.length}`}</Count>
      {states.length > 0 && (
        <Segmented size="small" className="lib-stands" value={stand || everyStand} onChange={(v) => setStand(v === everyStand ? "" : v)} aria-label={`Where each ${kind.title} stands`}>
          <Segment value={everyStand}>All</Segment>
          {states.map((s) => (
            <Segment key={s} value={s}>
              {s} {counts.get(s) ?? 0}
            </Segment>
          ))}
        </Segmented>
      )}
      <span className="spacer" />
      <label className="lib-sort dim">
        <Icon name="list" size={12} />
        <Select size="tiny" value={sort} onChange={(v) => setSort(v as Sort)} aria-label={`Sort the ${kind.title} list`}>
          <Option value="made">{docs.some((d) => d.number) ? "By number" : "Newest first"}</Option>
          <Option value="title">By title</Option>
          {states.length > 0 && <Option value="stands">By where it stands</Option>}
          <Option value="changed">Changed lately</Option>
        </Select>
      </label>
    </GroupHead>
  );
}

// InFlightChip says a document is changed in capsules: in flight.
export function InFlightChip({ capsules }: { capsules: string[] }) {
  return (
    <Chip tone="progress" title={`Changed in ${capsules.join(", ")}: in flight`}>
      in flight
    </Chip>
  );
}

export const libraryID = (path: string) => `lib-${path.replace(/[^\w-]/g, "_")}`;

// useKindOrder is how each kind's list is sorted and filtered, as chosen.
export function useKindOrder() {
  const [sorts, setSorts] = useState<Record<string, Sort>>({});
  const [stands, setStands] = useState<Record<string, string>>({});
  return {
    sortOf: (k: string) => sorts[k] ?? "made",
    setSort: (k: string, s: Sort) => setSorts((x) => ({ ...x, [k]: s })),
    standOf: (k: string) => stands[k] ?? "",
    setStand: (k: string, s: string) => setStands((x) => ({ ...x, [k]: s })),
  };
}
