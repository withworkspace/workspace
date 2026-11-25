import { type DocHit, docTo, type SpaceDoc } from "../api";
import { useThingMenu } from "../shell/ThingMenu";
import { Rows } from "../ui/Rows";
import { DocumentsInFlightChip } from "./DocumentsInFlightChip";
import { type Picking, pickOn } from "./picking";
import { docID } from "./space";
import { SpaceDocRow } from "./SpaceDocRow";
import { SpaceHitLine } from "./SpaceHitLine";

// FolderList is a folder's documents in a space's list (those of no
// kind), its rows SpaceDocRows that say their file.
export function FolderList({
  repo,
  label,
  docs,
  at,
  index,
  setIndex,
  flights,
  hits,
  picking,
}: {
  repo: string;
  label: string; // the folder's, to a screen reader
  docs: SpaceDoc[];
  at: number; // the index of its first row, among the page's
  index: number;
  setIndex?: (i: number) => void; // the cursor follows the pointer
  flights: Map<string, string[]>;
  hits?: Map<string, DocHit>; // what a search found, by path
  picking?: Picking;
}) {
  const menu = useThingMenu();
  return (
    <Rows role="listbox" aria-label={label}>
      {docs.map((d, j) => {
        const i = at + j;
        const hit = hits?.get(d.path);
        const to = docTo(`${repo}:${d.path}`);
        return (
          <SpaceDocRow
            key={d.path}
            id={docID(d.path)}
            to={to}
            title={d.title}
            selected={i === index}
            picked={picking?.picked === d.path}
            onHover={setIndex && (() => i !== index && setIndex(i))}
            onContextMenu={picking ? undefined : menu(to)}
            onClick={picking && pickOn(picking, d)}
            onDoubleClick={picking && (() => picking.onOpen(d))}
            chips={flights.has(d.path) && <DocumentsInFlightChip capsules={flights.get(d.path)!} />}
            hit={hit && <SpaceHitLine heading={hit.heading} snippet={hit.snippet} />}
            file={d.path.split("/").pop() ?? d.path}
          />
        );
      })}
    </Rows>
  );
}
