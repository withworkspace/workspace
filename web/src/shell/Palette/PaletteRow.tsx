import { FileIcon } from "../../ui/FileIcon";
import { Icon } from "../../ui/Icon";
import { Status } from "../../ui/Status";
import type { GroupKey, Item } from "../find";
import { FindCodeRow } from "./FindCodeRow";
import { FindRow } from "./FindRow";
import { FindTitle } from "./FindTitle";
import { titleParts } from "./title";

// fileOf is the file a row is, if it's one (a file, a line of one): it
// has its language's icon, as in the trees.
function fileOf(it: Item): string | undefined {
  if (it.icon !== "file") return undefined;
  if (it.code) return it.code.path;
  if (it.card?.kind === "file") return it.card.path;
  if (it.path) return it.title.replace(/:\d+(-\d+)?$/, "");
  return undefined;
}

// PaletteRow is one of what Find found (it), as the row it's drawn as: a
// line a file says is a FindCodeRow, anything else a FindRow, with its
// status or its icon as its mark. section is the group it's shown in.
export function PaletteRow({ it, on, section, onOver, onPick }: { it: Item; on: boolean; section: GroupKey; onOver: () => void; onPick: (tab: boolean) => void }) {
  const id = `pal-${it.id}`;
  if (it.code) return <FindCodeRow id={id} on={on} h={it.code} onOver={onOver} onPick={onPick} />;
  const file = fileOf(it);
  // Among other kinds, a row says what it is.
  const says = section === "best" || section === "needs" || section === "linked" || section === "recent";
  return (
    <FindRow
      id={id}
      on={on}
      mark={it.status ? <Status s={it.status} iconOnly /> : file ? <FileIcon path={file} /> : <Icon name={it.icon ?? "chevron-right"} />}
      title={<FindTitle parts={titleParts(it.title, it.at ?? [], it.path)} path={it.path} />}
      hint={it.hint}
      notes={it.notes}
      tone={it.tone && it.tone !== "quiet" ? it.tone : undefined}
      kind={says ? it.kind : undefined}
      onOver={onOver}
      onPick={onPick}
    />
  );
}
