import { useCallback, useEffect, useMemo, useState } from "react";
import { Link, useLocation, useNavigate, useParams, useSearchParams } from "react-router";
import { AnnotatedDoc, annotateKeys } from "../annotate/Annotate";
import { type ArtifactContent, type DocDraft, dropDraft, getArtifact, getDraft, kindHint, placeOf, type Workspace } from "../api";
import { useKeys } from "../keys";
import { Markdown } from "../markdown/LazyMarkdown";
import { highlightCode, languageOf, useHighlighter } from "../review/highlight";
import { Icon } from "../ui/Icon";
import { Callout, Copyable, ErrorCard, Keys, Skeleton, toast, useTitle } from "../ui/kit";
import { Time } from "../ui/time";
import { DocEdit } from "./DocEdit";
import { ArtifactState, artifactTitle } from "./parts";
import { useLatest } from "../load";

export const artifactKeys: [string, string][] = [
  ...annotateKeys.map(([k, what]): [string, string] => (k === "j / k" ? [k, `${what} (in code: scroll)`] : [k, what])),
  ["e", "Edit it (a document in a capsule, or the mission's own)"],
  ["mod+D / mod+U", "Down / up half a page"],
  ["g / G", "Top / bottom"],
  ["o", "Open it on GitHub (or where it lives)"],
  ["r", "Reload"],
  ["esc", "Back"],
];

// ArtifactPage shows one of a mission's artifacts: markdown rendered, with
// diagrams, to read and review (comments on its text) and edit in place
// (e); anything else as code.
export function ArtifactPage({ workspace }: { workspace: Workspace }) {
  const { slug = "" } = useParams();
  const [params] = useSearchParams();
  const ref = params.get("ref") ?? "";
  const navigate = useNavigate();
  const location = useLocation();
  const mission = workspace.missions.find((m) => m.slug === slug);
  const [a, setA] = useState<ArtifactContent>();
  const [error, setError] = useState<string>();
  useHighlighter();
  const [editing, setEditing] = useState(false);
  // Your unsaved edit of it, kept with the mission, if there's one.
  const [draft, setDraft] = useState<DocDraft | null>();
  const named = a ? artifactTitle(a) : undefined;
  useTitle(named?.title ?? "Document");

  const begin = useLatest();
  const load = useCallback(() => {
    const fresh = begin();
    getArtifact(slug, ref).then(
      (x) => {
        if (!fresh()) return;
        setA(x);
        setError(undefined);
        if (x.markdown && x.editable) getDraft(slug, ref).then((d) => fresh() && setDraft(d), () => fresh() && setDraft(null));
        else setDraft(null);
      },
      (e: Error) => fresh() && setError(e.message),
    );
  }, [slug, ref, begin]);
  const place = useMemo(() => a?.in && placeOf(a.in), [a?.in?.repo, a?.in?.worktree, a?.in?.path]);
  const canEdit = !!a?.markdown && !!a.editable;
  const startEdit = () => canEdit && setEditing(true);
  useEffect(load, [load]);

  const scroller = () => document.querySelector("main");
  const scroll = (by: number) => scroller()?.scrollBy({ top: by });
  // Back where you came from, or to the mission's work if this was opened
  // directly.
  const back = () => (location.key !== "default" ? navigate(-1) : navigate(`/missions/${slug}?tab=work`));
  // A document's own keys move through it; code just scrolls.
  const annotating = !!a?.annotatable;
  // While you edit, the editor has the keys.
  useKeys(editing ? {} : {
    ...(canEdit ? { e: startEdit } : {}),
    ...(annotating
      ? {}
      : {
          j: () => scroll(60),
          ArrowDown: () => scroll(60),
          k: () => scroll(-60),
          ArrowUp: () => scroll(-60),
        }),
    "mod+d": () => scroll(window.innerHeight / 2),
    "mod+u": () => scroll(-window.innerHeight / 2),
    g: () => scroller()?.scrollTo({ top: 0 }),
    G: () => scroller()?.scrollTo({ top: scroller()!.scrollHeight }),
    o: () => a?.url && window.open(a.url, "_blank", "noopener"),
    r: load,
    // A document's Esc clears what's picked out in it first: it goes back.
    ...(annotating ? {} : { Escape: back, h: back }),
  });

  return (
    <div className={`page artifact-page${annotating ? " annotating" : ""}`}>
      <nav className="crumbs" aria-label="Breadcrumb">
        <Link to={`/missions/${slug}?tab=work`}>
          <Icon name="arrow-left" size={13} /> {mission?.title ?? slug}
        </Link>
      </nav>
      {/* A document that starts with its own title is its own heading. */}
      {!(a?.markdown && /^#\s/.test(a.text.trimStart())) && (
        <header className="page-head">
          <h1>{named?.title ?? (a ? a.ref : "")}</h1>
        </header>
      )}
      {a && (
        <div className="doc-meta">
          {a.docKind ? (
            <span className="doc-kind" title={kindHint(a.docKind)}>
              {a.docKind.title}
            </span>
          ) : (
            <span className="doc-kind">{a.kind[0].toUpperCase() + a.kind.slice(1)}</span>
          )}
          <ArtifactState state={a.state} />
          {a.by && <span>by {a.by}</span>}
          {a.added && <Time iso={a.added} as="relative" />}
          <Copyable text={a.ref} className="mono doc-ref" title="Copy its ref">
            {a.ref.replace(":", " · ")}
          </Copyable>
          <span className="spacer" />
          {a.markdown && !editing && (
            <button className="small" onClick={startEdit} disabled={!a.editable} title={a.editable ? "Edit it here (e)" : `Can't be edited here: ${a.uneditable}`}>
              <Icon name="pencil" size={12} /> Edit <Keys k="e" />
            </button>
          )}
          {a.url && (
            <a href={a.url} target="_blank" rel="noopener noreferrer" className="button-link small-link" title="Open it where it lives (o)">
              <Icon name="external" size={12} /> Open <Keys k="o" />
            </a>
          )}
        </div>
      )}
      {error && <ErrorCard title="Couldn't open this document" error={error} retry={load} />}
      {!a && !error && <Skeleton rows={10} />}
      {a && !editing && draft && (
        <Callout
          tone="attention"
          icon="pencil"
          title="You've an unsaved edit of this"
          actions={
            <>
              <button className="small primary" onClick={startEdit}>
                Go on editing <Keys k="e" />
              </button>
              <button className="small ghost" onClick={() => dropDraft(slug, ref).then(() => setDraft(null))}>
                Discard it
              </button>
            </>
          }
        >
          Kept <Time iso={draft.at} as="relative" />
          {draft.base !== a.version ? ". The document's changed since: saving merges your edit with those changes." : "."}
        </Callout>
      )}
      {a && editing ? (
        <DocEdit
          slug={slug}
          a={a}
          draft={draft}
          onSaved={() => {
            setEditing(false);
            setDraft(null);
            load();
          }}
          onClose={(kept) => {
            setEditing(false);
            if (kept) toast("Your edit's kept as a draft: e goes on with it", { tone: "info" });
            getDraft(slug, ref).then(setDraft, () => {});
          }}
        />
      ) : (
        a &&
        (a.media && a.raw ? (
          <MediaView a={a} />
        ) : a.annotatable ? (
          <AnnotatedDoc key={a.ref} slug={slug} artifact={a} updated={mission?.updated} onBack={back} onChanged={load} />
        ) : a.markdown ? (
          <article className="artifact-body prose">
            <Markdown text={a.text} at={place} />
          </article>
        ) : (
          <pre className="code-block artifact-code">
            <code dangerouslySetInnerHTML={{ __html: highlightCode(a.text, languageOf(a.path ?? a.ref)) }} />
          </pre>
        ))
      )}
      {a?.path && (
        <p className="doc-path">
          <Copyable text={a.path} className="mono" title="Copy where it is" />
        </p>
      )}
    </div>
  );
}

// MediaView is a mission's image or video (crew's proof, say), shown as it
// is, under its label (the page's title): an image opens on its own, full
// size; a video plays here.
function MediaView({ a }: { a: ArtifactContent }) {
  const what = a.label || a.ref;
  return (
    <figure className="artifact-media">
      {a.media?.startsWith("video/") ? (
        <video src={a.raw} controls preload="metadata" aria-label={what} />
      ) : (
        <a href={a.raw} target="_blank" rel="noopener noreferrer" title="Open it on its own, full size">
          <img src={a.raw} alt={what} />
        </a>
      )}
    </figure>
  );
}
