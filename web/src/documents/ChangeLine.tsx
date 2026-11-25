import { docTo, type Space, type SpaceCommit } from "../api";
import { DocumentsChangeLine } from "./DocumentsChangeLine";
import { DocumentsSpaceTag } from "./DocumentsSpaceTag";
import { type InSpace, saysNoMore } from "./happening";
import { mark } from "./space";

// ChangeLine is a commit on one line, by the first document it changed,
// drawn as a DocumentsChangeLine: what the commit said only when it says
// more than that; which space, where there's more than one (space).
export function ChangeLine({ c, space }: { c: InSpace<SpaceCommit>; space?: Space }) {
  const d = c.docs[0];
  if (!d) return null;
  const rest = c.docs.slice(1);
  return (
    <DocumentsChangeLine
      how={d.how}
      name={d.title || d.path.split("/").pop() || d.path}
      path={d.path}
      to={d.how === "removed" ? undefined : docTo(`${c.repo}:${d.path}`)}
      more={rest.length > 0 ? { count: rest.length, title: rest.map((r) => `${mark(r.how)} ${r.title || r.path}`).join("\n") } : undefined}
      subject={saysNoMore(c.subject, d) ? undefined : c.subject}
      space={space && <DocumentsSpaceTag label={space.label} color={space.color} />}
      by={c.by}
      at={c.at}
    />
  );
}
