import { useCallback, useEffect, useMemo, useState } from "react";
import { Crumbs } from "../../ui/Crumbs";
import { useLocation, useNavigate, useParams, useSearchParams } from "react-router";
import { AnnotatedDoc } from "../../annotate/Annotate";
import { type ArtifactContent, type DocDraft, dropDraft, getArtifact, getDraft, kindHint, placeOf } from "../../api";
import { useSaid } from "../../keymap/keymap";
import { useCommand, useCommands } from "../../keymap/useCommands";
import { artifactKeys } from "./keys";
import { Markdown } from "../../markdown/LazyMarkdown";
import { highlightCode, languageOf, useHighlighter } from "../../review/highlight";
import { Icon } from "../../ui/Icon";
import { useTitle } from "../../ui/useTitle";
import { toast } from "../../ui/toast";
import { Skeleton } from "../../ui/Skeleton";
import { ErrorCard } from "../../ui/ErrorCard";
import { Callout } from "../../ui/Callout";
import { keyProps } from "../../ui/keyHint";
import { Keys, keysSaid } from "../../ui/Keys";
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
  const edit = useCommand(artifactKeys.edit, startEdit, { enabled: canEdit && !editing });
  const again = useCommand(artifactKeys.reload, load, { enabled: !editing });
  const reading = !editing;
  // A document that's commented on has its own j, k and esc (annotate's):
  // its Esc clears what's picked out in it first, then goes back.
  const scrolls = reading && !annotating;
  useCommands(artifactKeys, {
    down: scrolls ? () => scroll(60) : undefined,
    up: scrolls ? () => scroll(-60) : undefined,
    halfDown: reading ? () => scroll(window.innerHeight / 2) : undefined,
    halfUp: reading ? () => scroll(-window.innerHeight / 2) : undefined,
    top: reading ? () => pageScroller()?.scrollTo({ top: 0 }) : undefined,
    bottom: reading ? () => pageScroller()?.scrollTo({ top: pageScroller()!.scrollHeight }) : undefined,
    open: reading ? () => void (a?.url && window.open(a.url, "_blank", "noopener")) : undefined,
    back: scrolls ? () => void back() : undefined,
  });
  const editKey = useSaid(artifactKeys.edit);
  const openKey = useSaid(artifactKeys.open);

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
                  <Icon name="pencil" size={12} /> Edit {editKey && <Keys k={editKey} />}
                </Button>
              )}
              {a.url && (
                <a href={a.url} target="_blank" rel="noopener noreferrer" className={buttonClass({ size: "small" })} {...keyProps(editing ? undefined : openKey, "Open in repo")}>
                  <Icon name="external" size={12} /> Open {openKey && <Keys k={openKey} />}
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
                Continue editing {editKey && <Keys k={editKey} />}
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
            if (kept) toast(editKey ? `Draft saved: ${keysSaid(editKey).join(" ")} to continue` : "Draft saved", { tone: "info" });
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
