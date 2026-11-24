import { useEffect, useMemo } from "react";
import { type ArtifactContent, getDoc, placeOf } from "../api";
import { DockBody } from "../chrome/DockBody";
import { PaneHead } from "../chrome/PaneHead";
import { useTabNames } from "../chrome/tabNames";
import { DocumentPeek } from "../documents/DocumentPeek";
import { docPeekOf, titleOf } from "../documents/peek";
import { useLoaded } from "../load";
import { Markdown } from "../markdown/LazyMarkdown";
import { ErrorCard } from "../ui/ErrorCard";
import { Loading } from "../ui/Loading";
import { humanError } from "../ui/text";
import { useWorkspace } from "../workspace/store";

type DocumentPaneProps = {
  // Its tab's address, which is its own page's.
  to: string;
  // The document as ws names it: repo:path, or repo/capsule:path.
  docRef: string;
};

// DocumentPane is a document, peeked at beside the page: its text as it
// is now, drawn as a DocumentPeek. Its links go where they'd go on its
// own page, in the page's place.
export function DocumentPane({ to, docRef }: DocumentPaneProps) {
  const spaces = useWorkspace((w) => w.spaces);
  const { value: a, error, reload } = useLoaded<ArtifactContent>(() => getDoc(docRef), [docRef]);
  const of = useMemo(() => a && docPeekOf(a, docRef, (repo) => spaces?.find((s) => s.repo === repo)?.label ?? repo), [a, docRef, spaces]);
  const where = a?.in;
  const place = useMemo(() => where && placeOf(where), [where?.repo, where?.worktree, where?.path]);
  // Until it's read, it's called by its file's name.
  const title = of?.title ?? titleOf("", docRef.slice(docRef.indexOf(":") + 1));
  useEffect(() => {
    if (of) useTabNames.getState().say(to, of.title);
  }, [to, of]);
  return (
    <DockBody label={title} pad>
      <PaneHead title={title} to={to} />
      {error && !a && <ErrorCard title="Couldn't read the document" error={humanError(error)} retry={() => void reload()} />}
      {!a && !error && <Loading what="Reading the document…" />}
      {of && (
        <DocumentPeek repo={of.repo} capsule={of.capsule} path={of.path} words={of.words} mission={of.mission}>
          <Markdown text={of.text} at={place} />
        </DocumentPeek>
      )}
    </DockBody>
  );
}
