import { type Citation, docTo } from "../../api";
import { useThingMenu } from "../../shell/ThingMenu";
import { ReaderCiteList } from "./ReaderCiteList";

// DocCiteList is a document's citations as a ReaderCiteList: each to the
// document it is, with what Find can do to it on its menu.
export function DocCiteList({ title, list }: { title: string; list: Citation[] }) {
  const menu = useThingMenu();
  return (
    <ReaderCiteList
      title={title}
      items={list.map((c) => {
        const to = docTo(`${c.repo}:${c.path}`);
        const name = c.path.split("/").pop() ?? c.path;
        return { key: `${c.repo}:${c.path}`, to, title: c.title || name, file: name, line: c.line, how: c.how, onContextMenu: menu(to) };
      })}
    />
  );
}
