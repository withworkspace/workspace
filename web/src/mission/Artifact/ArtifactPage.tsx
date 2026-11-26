import { useCallback, useEffect, useMemo, useState } from "react";
import { Crumbs } from "../../ui/Crumbs";
import { useLocation, useNavigate, useParams, useSearchParams } from "react-router";
import { AnnotatedDoc } from "../../annotate/Annotate";
import { type ArtifactContent, type DocDraft, dropDraft, getArtifact, getDraft, kindHint, placeOf } from "../../api";
import { useKeys, useShortcut } from "../../keys";
import { Markdown } from "../../markdown/LazyMarkdown";
import { highlightCode, languageOf, useHighlighter } from "../../review/highlight";
import { Icon } from "../../ui/Icon";
import { useTitle } from "../../ui/useTitle";
import { toast } from "../../ui/toast";
import { Skeleton } from "../../ui/Skeleton";
import { ErrorCard } from "../../ui/ErrorCard";
import { Callout } from "../../ui/Callout";
import { keyProps } from "../../ui/keyHint";
import { Keys } from "../../ui/Keys";
import { Time } from "../../ui/Time";
import { artifactTitle } from "../outputs";
import { DocEdit } from "./DocEdit";
import { MissionArtifactCode } from "./MissionArtifactCode";
import { MissionArtifactMedia } from "./MissionArtifactMedia";
import { MissionArtifactMeta } from "./MissionArtifactMeta";
import { MissionArtifactPage } from "./MissionArtifactPage";
import { MissionArtifactPath } from "./MissionArtifactPath";
import { MissionArtifactText } from "./MissionArtifactText";
import { useLatest } from "../../load";
import { Button, buttonClass } from "../../ui/Button";
import { PageHead } from "../../ui/PageHead";
import { pageScroller } from "../../ui/scroller";
import { capital } from "../../ui/strings";
import { useWorkspace } from "../../workspace/store";

// ArtifactPage shows one of a mission's artifacts: markdown rendered, with
// diagrams, to read and review (comments on its text) and edit in place
// (e); anything else as code.
export function ArtifactPage() {
  const { slug = "" } = useParams();
  const [params] = useSearchParams();
  const ref = params.get("ref") ?? "";
  const navigate = useNavigate();
  const location = useLocation();
  const mission = useWorkspace((w) => w.missions.find((m) => m.slug === slug));
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

  const scroll = (by: number) => pageScroller()?.scrollBy({ top: by });
  // Back where you came from, or to the mission's work if this was opened
  // directly.
  const back = () => (location.key !== "default" ? navigate(-1) : navigate(`/missions/${slug}?tab=work`));
  // A document's own keys move through it; code just scrolls.
  const annotating = !!a?.annotatable;
  // What the page's buttons do too, each with its key: the key's bound
  // here, and the button's handed what comes back, which says it. Not
  // while you edit, when the editor has the keys.
  const edit = useShortcut("e", startEdit, { enabled: canEdit && !editing });
  const again = useShortcut("r", load, { enabled: !editing });
  useKeys(editing ? {} : {
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
    g: () => pageScroller()?.scrollTo({ top: 0 }),
    G: () => pageScroller()?.scrollTo({ top: pageScroller()!.scrollHeight }),
    o: () => a?.url && window.open(a.url, "_blank", "noopener"),
    // A document's Esc clears what's picked out in it first: it goes back.
    ...(annotating ? {} : { Escape: back, h: back }),
  });

  return (
    <MissionArtifactPage annotating={annotating}>
      <Crumbs steps={[{ to: `/missions/${slug}?tab=work`, label: mission?.title ?? slug }]} />
      {/* A document that starts with its own title is its own heading. */}
      {!(a?.markdown && /^#\s/.test(a.text.trimStart())) && (
        <PageHead title={named?.title ?? (a ? a.ref : "")} />
      )}
      {a && (
        <MissionArtifactMeta
          kind={a.docKind ? a.docKind.title : capital(a.kind)}
          hint={a.docKind ? kindHint(a.docKind) : undefined}
          state={a.state}
          by={a.by}
          added={a.added}
          docRef={a.ref}
          actions={
            <>
              {a.markdown && !editing && (
                <Button size="small" onClick={edit} unavailable={a.editable ? undefined : `Not editable here: ${a.uneditable}`} title="Edit">
                  <Icon name="pencil" size={12} /> Edit <Keys k="e" />
                </Button>
              )}
              {a.url && (
                <a href={a.url} target="_blank" rel="noopener noreferrer" className={buttonClass({ size: "small" })} {...keyProps(editing ? undefined : "o", "Open in repo")}>
                  <Icon name="external" size={12} /> Open <Keys k="o" />
                </a>
              )}
            </>
          }
        />
      )}
      {error && <ErrorCard title="Could not open document" error={error} retry={again} />}
      {!a && !error && <Skeleton rows={10} />}
      {a && !editing && draft && (
        <Callout
          tone="attention"
          icon="pencil"
          title="Unsaved edit"
          actions={
            <>
              <Button variant="primary" size="small" onClick={edit}>
                Continue editing <Keys k="e" />
              </Button>
              <Button variant="ghost" size="small" onClick={() => dropDraft(slug, ref).then(() => setDraft(null))}>
                Discard
              </Button>
            </>
          }
        >
          Draft saved <Time iso={draft.at} as="relative" />
          {draft.base !== a.version ? ". Document changed since. Saving will merge." : "."}
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
            if (kept) toast("Draft saved: e to continue", { tone: "info" });
            getDraft(slug, ref).then(setDraft, () => {});
          }}
        />
      ) : (
        a &&
        (a.media && a.raw ? (
          <MissionArtifactMedia src={a.raw} video={a.media.startsWith("video/")} label={a.label || a.ref} />
        ) : a.annotatable ? (
          <AnnotatedDoc key={a.ref} slug={slug} artifact={a} updated={mission?.updated} onBack={back} onChanged={load} />
        ) : a.markdown ? (
          <MissionArtifactText>
            <Markdown text={a.text} at={place} />
          </MissionArtifactText>
        ) : (
          <MissionArtifactCode html={highlightCode(a.text, languageOf(a.path ?? a.ref))} />
        ))
      )}
      {a?.path && <MissionArtifactPath path={a.path} />}
    </MissionArtifactPage>
  );
}
