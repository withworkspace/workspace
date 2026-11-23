import { useEffect, useMemo, useRef, useState } from "react";
import { Link } from "react-router";
import { type ArtifactContent, docAddress, docTo, filesTo, getDoc, GROUND, placeOf, type SpaceActivity, type SpaceDoc, type SpaceKind } from "../api";
import { goToSection, useSections } from "../files/Contents";
import { useLatest } from "../load";
import { Markdown } from "../markdown/LazyMarkdown";
import { Icon } from "../ui/Icon";
import { ErrorCard, Skeleton } from "../ui/kit";
import { Copyable } from "../ui/Copyable";
import { Keys } from "../ui/Keys";
import { Time } from "../ui/time";
import { ChangeLine } from "./Activity";
import { withoutTitle, wordsIn } from "./happening";
import { Chip } from "../ui/Chip";
import { Button, buttonClass } from "../ui/Button";

// Preview is a space's document beside its list, as the reader shows it:
// what it is and where, who changed it last, how long it is, its sections
// (each scrolls the preview there), and its text; then the changes to it
// lately. It scrolls by itself. Read it (⏎) opens the reader, where it's
// asked about and edited.
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
  return (
    <aside className="doc-preview" ref={pane} aria-label={`${doc.title}, previewed`}>
      <header className="doc-preview-head">
        <div className="doc-preview-where">
          {kind && <Chip tone="quiet">{kind.title}</Chip>}
          {doc.state && kind?.states && <Chip tone={doc.state === kind.states[0] ? "quiet" : "done"}>{doc.state}</Chip>}
          {(doc.number || doc.date) && <span className="mono">{doc.number ?? doc.date}</span>}
          <Copyable text={ref} className="doc-preview-ref" title="Copy its ref">
            {`${repo} · ${doc.path}`}
          </Copyable>
        </div>
        <h2 className="doc-preview-title">{doc.title}</h2>
        <p className="doc-preview-meta">
          {doc.changed && (
            <>
              Changed by {doc.changedBy ?? "someone"}, <Time iso={doc.changed} as="age" /> ago
            </>
          )}
          {doc.changed && words > 0 && " · "}
          {words > 0 && `${words.toLocaleString()} words · ${Math.max(1, Math.round(words / 230))} min read`}
        </p>
        <div className="doc-preview-actions">
          <Link to={docTo(ref)} className={buttonClass({ variant: "primary" })} title="Read it in the reader, where it's asked about (⏎)">
            <Icon name="book" size={13} /> Read it <Keys k="⏎" />
          </Link>
          {canEdit && (
            <Link to={docAddress(ref, shown?.page ?? true)} className={buttonClass()} title="Edit it as a page: cells, questions and passes, saved back to the repo when you say">
              <Icon name="pencil" size={13} /> Edit as a page
            </Link>
          )}
          <Link to={filesTo(repo, GROUND, doc.path)} className={buttonClass({ variant: "ghost" })} title="Open it in Files">
            <Icon name="layers" size={13} /> In Files
          </Link>
        </div>
      </header>
      {flights && flights.length > 0 && (
        <div className="banner tone-progress doc-preview-flight">
          <Icon name="box" size={13} /> Changed in{" "}
          {flights.map((c, i) => (
            <span key={c}>
              {i > 0 && ", "}
              <Link to={docAddress(`${repo}/${c}:${doc.path}`)}>{c}</Link>
            </span>
          ))}
          : in flight, so what's here may be about to change.
        </div>
      )}
      {sections.filter((s) => s.level === 2).length > 1 && (
        <nav className="doc-preview-sections" aria-label="Its sections">
          {sections
            .filter((s) => s.level === 2)
            .map((s) => (
              <Button variant="link" key={s.anchor} onClick={() => goToSection(body.current, s.anchor)}>
                {s.text}
              </Button>
            ))}
        </nav>
      )}
      {error && <ErrorCard title="Couldn't read it" error={error} retry={load} />}
      {!shown && !error && <Skeleton rows={8} />}
      {shown && text !== undefined && (
        <article className="artifact-body prose doc-preview-body" ref={body}>
          <Markdown text={text} at={place} />
        </article>
      )}
      {changes.length > 0 && (
        <section className="doc-preview-changes" aria-label="Changes to it lately">
          <h3>Changes to it lately</h3>
          <ul className="doc-changes">
            {changes.map((c) => (
              <ChangeLine key={c.sha} c={{ ...c, repo }} />
            ))}
          </ul>
        </section>
      )}
    </aside>
  );
}
