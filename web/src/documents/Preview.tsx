import { useEffect, useMemo, useRef, useState } from "react";
import { Link } from "react-router";
import { type ArtifactContent, docAddress, docTo, filesTo, getDoc, GROUND, placeOf, type SpaceActivity, type SpaceDoc, type SpaceKind } from "../api";
import { goToSection } from "../files/Reader/goToSection";
import { useSections } from "../files/Reader/useSections";
import { useLatest } from "../load";
import { Markdown } from "../markdown/LazyMarkdown";
import { buttonClass } from "../ui/Button";
import { Chip } from "../ui/Chip";
import { ErrorCard } from "../ui/ErrorCard";
import { Icon } from "../ui/Icon";
import { keyProps } from "../ui/keyHint";
import { Keys } from "../ui/Keys";
import { Time } from "../ui/Time";
import { ChangeLine } from "./ChangeLine";
import { DocumentsChanges } from "./DocumentsChanges";
import { withoutTitle, wordsIn } from "./happening";
import { SpacePreview } from "./SpacePreview";
import { SpacePreviewBody } from "./SpacePreviewBody";
import { SpacePreviewChanges } from "./SpacePreviewChanges";
import { SpacePreviewFlight } from "./SpacePreviewFlight";
import { SpacePreviewHead } from "./SpacePreviewHead";
import { SpacePreviewSections } from "./SpacePreviewSections";
import { SpacePreviewSkeleton } from "./SpacePreviewSkeleton";

// Preview is a space's document beside its list, as the reader shows it
// (a SpacePreview): what it is and where, who changed it last, how long
// it is, its sections (each scrolls the preview there), and its text;
// then the changes to it lately. It scrolls by itself. Read it (⏎) opens
// the reader, where it's asked about and edited.
export function Preview({ repo, doc, kind, activity, flights, anchor }: { repo: string; doc: SpaceDoc; kind?: SpaceKind; activity?: SpaceActivity; flights?: string[]; anchor?: string }) {
  const ref = `${repo}:${doc.path}`;
  const [a, setA] = useState<ArtifactContent>();
  const [error, setError] = useState<string>();
  const begin = useLatest();
  const load = () => {
    const fresh = begin();
    setError(undefined);
    getDoc(ref).then(
      (x) => fresh() && setA(x),
      (e: Error) => fresh() && setError(e.message),
    );
  };
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(load, [ref]);
  const shown = a?.ref === ref || a?.path === doc.path ? a : undefined;
  const pane = useRef<HTMLElement>(null);
  const body = useRef<HTMLElement>(null);
  const text = useMemo(() => (shown ? withoutTitle(shown.text) : undefined), [shown]);
  const { sections } = useSections(body, text);
  const where = shown?.in;
  const place = useMemo(() => where && placeOf(where), [where?.repo, where?.worktree, where?.path]);
  // A new document starts at its top, or at the section a search led to.
  useEffect(() => {
    if (!shown) return;
    if (anchor) requestAnimationFrame(() => goToSection(body.current, anchor));
    else if (pane.current) pane.current.scrollTop = 0;
  }, [shown, anchor]);
  const words = text ? wordsIn(text) : 0;
  const changes = (activity?.lately ?? []).filter((c) => c.docs.some((d) => d.path === doc.path));
  const canEdit = !!shown?.editable && !shown.mission;
  const heads = sections.filter((s) => s.level === 2);
  return (
    <SpacePreview label={`${doc.title} preview`} paneRef={pane}>
      <SpacePreviewHead
        chips={
          <>
            {kind && <Chip tone="quiet">{kind.title}</Chip>}
            {doc.state && kind?.states && <Chip tone={doc.state === kind.states[0] ? "quiet" : "done"}>{doc.state}</Chip>}
          </>
        }
        made={doc.number || doc.date ? (doc.number ?? doc.date) : undefined}
        docRef={ref}
        where={`${repo} · ${doc.path}`}
        title={doc.title}
        meta={
          <>
            {doc.changed && (
              <>
                Changed by {doc.changedBy ?? "someone"}, <Time iso={doc.changed} as="age" /> ago
              </>
            )}
            {doc.changed && words > 0 && " · "}
            {words > 0 && `${words.toLocaleString()} words · ${Math.max(1, Math.round(words / 230))} min read`}
          </>
        }
        actions={
          <>
            <Link to={docTo(ref)} className={buttonClass({ variant: "primary" })} {...keyProps("⏎", "Open in reader")}>
              <Icon name="book" size={13} /> Read <Keys k="⏎" />
            </Link>
            {canEdit && (
              <Link to={docAddress(ref, shown?.page ?? true)} className={buttonClass()} title="Edit as a page; save to the repo when ready">
                <Icon name="pencil" size={13} /> Edit as a page
              </Link>
            )}
            <Link to={filesTo(repo, GROUND, doc.path)} className={buttonClass({ variant: "ghost" })} title="Open in Files">
              <Icon name="layers" size={13} /> In Files
            </Link>
          </>
        }
      />
      {flights && flights.length > 0 && <SpacePreviewFlight capsules={flights.map((c) => ({ name: c, to: docAddress(`${repo}/${c}:${doc.path}`) }))} />}
      {heads.length > 1 && <SpacePreviewSections sections={heads} onGo={(anchor) => goToSection(body.current, anchor)} />}
      {error && <ErrorCard title="Could not load document" error={error} retry={load} />}
      {!shown && !error && <SpacePreviewSkeleton />}
      {shown && text !== undefined && (
        <SpacePreviewBody bodyRef={body}>
          <Markdown text={text} at={place} />
        </SpacePreviewBody>
      )}
      {changes.length > 0 && (
        <SpacePreviewChanges>
          <DocumentsChanges>
            {changes.map((c) => (
              <ChangeLine key={c.sha} c={{ ...c, repo }} />
            ))}
          </DocumentsChanges>
        </SpacePreviewChanges>
      )}
    </SpacePreview>
  );
}
