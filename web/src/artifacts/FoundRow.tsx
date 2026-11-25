import type { ArtifactFound } from "../api";
import { useThingMenu } from "../shell/ThingMenu";
import { Chip } from "../ui/Chip";
import { Icon } from "../ui/Icon";
import { ArtifactRow, artifactRowCodeClass } from "./ArtifactRow";
import { foundHref } from "./hrefs";
import { foundBy } from "./words";
import { Text } from "../ui/Text";

// FoundRow is an artifact found in a repo, in the list.
export function FoundRow({ a }: { a: ArtifactFound }) {
  const menu = useThingMenu();
  return (
    <ArtifactRow to={foundHref(a.name)} onContextMenu={menu(foundHref(a.name), a.name)} title={a.title} chips={<Chip tone="done">Promoted</Chip>} when={{ by: foundBy(a), at: a.changed, title: foundBy(a) }}>
      <span title={`It lives in ${a.where}`}>
        <Icon name="branch" size={11} /> {a.label}
      </span>
      <Text mono className={artifactRowCodeClass}>{a.where}</Text>
    </ArtifactRow>
  );
}
