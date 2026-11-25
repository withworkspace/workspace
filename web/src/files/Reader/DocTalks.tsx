import type { Talk } from "../../api";
import { useThingMenu } from "../../shell/ThingMenu";
import { talkHref, talkStatus } from "../../talk/Talk";
import { clip } from "../../ui/text";
import { ReaderTalks } from "./ReaderTalks";

// DocTalks is the conversations about a document as ReaderTalks: each by
// its name, the passage it was asked of, and where it stands.
export function DocTalks({ list }: { list: Talk[] }) {
  const menu = useThingMenu();
  return (
    <ReaderTalks
      items={list.map((t) => ({
        id: t.id,
        to: talkHref("", t.id),
        title: t.title || t.opening,
        summary: t.summary,
        status: talkStatus(t),
        sub: t.quote || t.section ? `${t.quote ? `“${clip(t.quote, 90)}”` : ""}${t.section ? `${t.quote ? " in " : ""}§ ${t.section}` : ""}` : undefined,
        at: t.updated || t.at,
        onContextMenu: menu(talkHref("", t.id)),
      }))}
    />
  );
}
