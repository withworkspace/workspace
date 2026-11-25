import { docTo, type SpaceDoc, type SpaceKind } from "../api";
import { useThingMenu } from "../shell/ThingMenu";
import { Chip } from "../ui/Chip";
import { Rows } from "../ui/Rows";
import { DocumentsInFlightChip } from "./DocumentsInFlightChip";
import { type Picking, pickOn } from "./picking";
import { libraryID } from "./space";
import { SpaceDocRow } from "./SpaceDocRow";
import { SpaceHitLine } from "./SpaceHitLine";

// A space's library: a list for each of the workspace's kinds of document
// whose path is in it, each with its number or date, its title, where it
// stands (the folder it's in: proposed, accepted) and who changed it
// last. KindList is one kind's list, its rows SpaceDocRows.
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
    <Rows role="listbox" aria-label={kind.title}>
      {docs.map((d, j) => {
        const i = at + j;
        const hit = picking?.hits?.get(d.path);
        const to = docTo(`${repo}:${d.path}`);
        return (
          <SpaceDocRow
            key={d.path}
            id={libraryID(d.path)}
            to={to}
            title={d.title}
            selected={i === index}
            picked={picking?.picked === d.path}
            onHover={setIndex && (() => i !== index && setIndex(i))}
            onContextMenu={picking ? undefined : menu(to)}
            onClick={picking && pickOn(picking, d)}
            onDoubleClick={picking && (() => picking.onOpen(d))}
            made={made ? (d.number ?? d.date ?? "") : undefined}
            chips={
              <>
                {d.state && kind.states && <Chip tone={d.state === kind.states[0] ? "quiet" : "done"}>{d.state}</Chip>}
                {flights?.has(d.path) && <DocumentsInFlightChip capsules={flights.get(d.path)!} />}
              </>
            }
            hit={hit && <SpaceHitLine heading={hit.heading} snippet={hit.snippet} />}
            changed={d.changed ? { by: d.changedBy, at: d.changed } : undefined}
          />
        );
      })}
    </Rows>
  );
}
