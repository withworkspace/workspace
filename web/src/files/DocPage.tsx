import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Crumbs } from "../ui/Crumbs";
import { Link, useLocation, useNavigate } from "react-router";
import { type ArtifactContent, type Citation, type DocDraft, docAddress, docRef as fileRef, docTo, dropDraft, filesTo, GROUND, getCites, getDoc, getDraft, getSpaceActivity, getThreads, kindHint, placeOf, type SpaceFlight, spaceAt, spaceTo, type Talk } from "../api";
import { useKeys } from "../keys";
import { Markdown } from "../markdown/LazyMarkdown";
import { type PageRef, wsAddress } from "../refs/refs";
import { Icon } from "../ui/Icon";
import { Callout, Copyable, ErrorCard, Keys, Skeleton, toast, useTitle } from "../ui/kit";
import { Status } from "../ui/status";
import { Time } from "../ui/time";
import { talkHref, talkStatus } from "../talk/Talk";
import { useLatest } from "../load";
import { aboutDoc, citedSection, type Passage } from "../ask/ask";
import { AskBar, type AskBarHandle } from "../ask/AskBar";
import { passageIn, SelectionTools, type SelectionToolsHandle } from "../ask/Selection";
import { ContentsMenu, ContentsPill, ContentsRail, goToSection, useRailFits, useRailHidden, useReading, useSections } from "./Contents";
import { FolderMenu, Pager, PathCrumbs, useShelves } from "./Path";
import { type Here, type Shelf, siblings, trail } from "./shelves";
import { PagePin } from "../shell/focus";
import { Chip, chipClass } from "../ui/Chip";
import { Button, buttonClass } from "../ui/Button";
import { PageHead } from "../ui/PageHead";
import { GroupHead } from "../ui/GroupHead";
import { Row, RowLine, RowMain, RowSub, RowTitle, Rows, rowLinkClass, rowStatusClass } from "../ui/Rows";
import { clip } from "../ui/text";
import { pageScroller } from "../ui/scroller";
import { useWorkspace } from "../workspace/store";

export const docKeys: [string, string][] = [
  ["e", "Edit it, here: as the page tied to it"],
  ["mod+D / mod+U", "Down / up half a page"],
  ["g / G", "Top / bottom"],
  ["t", "Show or hide its contents"],
  ["[ / ]", "Previous / next in its folder (or its kind's list)"],
  ["u", "What's in its folder; again to go up"],
  ["/", "Go to the ask bar: what you ask opens beside the page"],
  ["a", "Ask (about what's selected, if anything)"],
  ["o", "Open it in Files"],
  ["r", "Reload"],
  ["esc", "Back, or close what's open"],
];

// DocPage is a document outside any mission, read: a markdown file in one
// of the workspace's repos, on its default branch or in a capsule, as it's
// rendered. It's edited as a page (e): the page tied to the file, with the
// page's tools, saved back to the repo when you say. Questions and changes
// are asked there, so new threads aren't made here; ones made before are
// kept, and said to be.
//
// A file a mission records is that mission's document: this goes there,
// where its threads are.
//
// Its contents are beside the text where there's room (a rail), else in a
// pill stuck to the top once you've scrolled past its title (Contents.tsx);
// a heading's # copies a link to it (ws:<file>#<heading>).
//
// Its crumbs are its path, each step a menu of what's there, and the
// documents beside it are at its foot (Path.tsx): a space's document is
// kept in its kind's list, if it's of one, else in its folder; any other,
// in its folder of the repo (shelves.ts).
//
// Below them is the ask bar (ask/AskBar.tsx), about the document, or a
// passage picked out of it (ask/Selection.tsx's toolbar, or a): its answer
// is a card over the text, whose citations of its sections scroll there.
// An ask kept is a conversation about the document, listed below its text.
export function DocPage({ docRef }: { docRef: string }) {
  const conversations = useWorkspace((w) => w.conversations);
  const spaces = useWorkspace((w) => w.spaces);
  const spacesSeen = useWorkspace((w) => w.spacesSeen);
  const ref = docRef;
  const navigate = useNavigate();
  const location = useLocation();
  const [a, setA] = useState<ArtifactContent>();
  const [error, setError] = useState<string>();
  // An unsaved edit of it from before it was edited as a page, if there's one.
  const [draft, setDraft] = useState<DocDraft | null>();
  // How many threads were made on it before, while it took them.
  const [earlier, setEarlier] = useState(0);
  const begin = useLatest();
  const load = useCallback(() => {
    const fresh = begin();
    getDoc(ref).then(
      (x) => {
        if (!fresh()) return;
        setA(x);
        setError(undefined);
        if (x.editable && !x.mission) getDraft("", x.ref).then((d) => fresh() && setDraft(d), () => fresh() && setDraft(null));
        else setDraft(null);
        if (!x.mission) getThreads("", x.ref).then((t) => fresh() && setEarlier(t.threads.length), () => fresh() && setEarlier(0));
      },
      (e: Error) => fresh() && setError(e.message),
    );
  }, [ref, begin]);
  useEffect(load, [load]);
  // A mission's: there, where its threads are.
  const mission = a?.mission;
  useEffect(() => {
    if (mission) navigate(`/missions/${mission.slug}/artifact?${new URLSearchParams({ ref: mission.ref })}${location.hash}`, { replace: true });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [mission?.slug, mission?.ref]);

  const where = a?.in;
  const place = useMemo(() => where && placeOf(where), [where?.repo, where?.worktree, where?.path]);
  const files = where ? filesTo(where.repo, where.worktree, where.path) : "/files";
  const name = where ? where.path.slice(where.path.lastIndexOf("/") + 1) : ref;
  useTitle(a ? name : "Document");
  const canEdit = !!a?.editable && !a.mission;
  const edit = () => a && canEdit && navigate(docAddress(a.ref, a.page ?? true));
  // Back goes to its space, for one of a space's documents; else Files.
  const space = where && where.worktree === GROUND ? spaceAt(spaces, where.repo, where.path) : undefined;
  const home = space ? spaceTo(space.repo) : files;
  // What cites a space's document, and what it cites.
  const [cites, setCites] = useState<{ citedBy: Citation[]; cites: Citation[] }>();
  useEffect(() => {
    if (!space || !where) return setCites(undefined);
    let stale = false;
    getCites(space.repo, where.path).then(
      (c) => !stale && setCites(c),
      () => !stale && setCites(undefined),
    );
    return () => {
      stale = true;
    };
  }, [space?.repo, where?.path, spacesSeen]);
  // A space's document says when it's in flight: changed in a capsule. As
  // ws keeps it, and read again when ws has found the space changed.
  const [flights, setFlights] = useState<SpaceFlight[]>([]);
  useEffect(() => {
    if (!space || !where) return setFlights([]);
    let stale = false;
    getSpaceActivity(space.repo).then(
      (a) => !stale && !a.unread && setFlights(a.inFlight.filter((f) => f.docs.some((d) => d.path === where.path))),
      () => {},
    );
    return () => {
      stale = true;
    };
  }, [space?.repo, where?.path, spacesSeen]);
  // The conversations about it: asks that were kept, newest first.
  const asked = a?.ref;
  const talks = useMemo(() => (asked ? conversations.filter((t) => t.doc === asked) : []), [conversations, asked]);
  // Its contents: a rail where it fits, else the pill and its menu.
  const page = useRef<HTMLDivElement>(null);
  const body = useRef<HTMLElement>(null);
  const { title, sections } = useSections(body, a?.text);
  const reading = useReading(body, sections, a?.text);
  const fits = useRailFits(page);
  const [railHidden, toggleRail] = useRailHidden();
  const [menu, setMenu] = useState(false);
  const closeMenu = useCallback(() => setMenu(false), []);
  useEffect(closeMenu, [fits, closeMenu]);
  const pill = useRef<HTMLButtonElement>(null);
  const contentsButton = useRef<HTMLButtonElement>(null);
  const read = !!a && !mission;
  const rail = read && fits;
  const folded = read && !fits;
  const toSection = (anchor: string) => {
    setMenu(false);
    goToSection(body.current, anchor);
  };
  // Where it's kept (its path), and the documents beside it.
  const here = useMemo<Here | undefined>(() => (where ? { path: where.path, title: title ?? name, kind: a?.docKind?.name } : undefined), [where?.path, title, name, a?.docKind?.name]);
  const shelves = useShelves(read ? where : undefined, space, here);
  const on = useMemo(() => (shelves && where ? trail(shelves, where.path) : []), [shelves, where?.path]);
  const sib = useMemo(() => (shelves && where ? siblings(shelves, where.path) : undefined), [shelves, where?.path]);
  // The shelf whose menu is open, if one is: one menu at a time.
  const [folder, setFolder] = useState<string>();
  const closeFolder = useCallback(() => setFolder(undefined), []);
  const openFolder = (id: string | undefined) => {
    setMenu(false);
    setFolder(id);
  };
  const toggleMenu = () => {
    setFolder(undefined);
    setMenu((m) => !m);
  };
  const crumbs = useRef<HTMLElement>(null);
  const column = useRef<HTMLDivElement>(null);
  const toDoc = (p: string) => (where ? docAddress(fileRef(where.repo, where.worktree, p)) : "");
  const openIn = (s: Shelf) => {
    if (!where) return { to: files, label: "Open in Files" };
    if (space) return { to: s.kind ? spaceTo(space.repo, { kind: s.kind }) : s.id ? spaceTo(space.repo, { folder: s.id }) : spaceTo(space.repo), label: "Open in the space" };
    return { to: filesTo(where.repo, where.worktree, s.id ? s.id.slice(0, -1) : undefined), label: "Open in Files" };
  };
  // u opens the shelf it's on; again, the one above.
  const up = () => {
    if (!shelves || !where) return;
    if (folder === undefined) return openFolder(shelves.home.get(where.path));
    const above = shelves.shelves.get(folder)?.up;
    if (above !== undefined) setFolder(above);
  };
  const step = (n: -1 | 1) => {
    if (!sib) return;
    const to = n < 0 ? sib.prev : sib.next;
    if (to?.path) navigate(toDoc(to.path));
    else toast(n < 0 ? `It's the first in ${sib.shelf.name}` : `It's the last in ${sib.shelf.name}`);
  };
  // A link to it, or to one of its headings: its ws: address, which ws
  // reads back wherever it's pasted.
  const repo = where?.repo;
  const worktree = where?.worktree;
  const path = where?.path;
  const copyLink = useCallback(
    (anchor?: string) => {
      if (!repo || !worktree || !path) return;
      const link = wsAddress({ kind: "file", repo, worktree, path, ...(anchor ? { anchor } : {}) } satisfies PageRef);
      if (!navigator.clipboard) return toast("Couldn't copy: the browser said no");
      navigator.clipboard.writeText(link).then(
        () => toast(`Copied ${link}`),
        () => toast("Couldn't copy: the browser said no"),
      );
    },
    [repo, worktree, path],
  );
  // Asking about it: the bar at its foot, and the toolbar over a passage
  // picked out. The card's answer cites its sections, which scroll there.
  const asker = useRef<AskBarHandle<Passage>>(null);
  const tools = useRef<SelectionToolsHandle>(null);
  const others = cites?.citedBy[0] ?? cites?.cites[0];
  const suggest = (p?: Passage) =>
    p
      ? ["Explain this passage", "Why does it say this?"]
      : ["Summarise it", "What's still open?", "Where does it stand: what's built?", ...(others ? [`What does it mean for ${others.title || others.path.split("/").pop()}?`] : [])];
  const cite = (href: string) => {
    const s = citedSection(href, sections);
    if (s) toSection(s);
    return !!s;
  };
  const askAbout = () => {
    const p = passageIn(body.current);
    tools.current?.hide();
    asker.current?.ask(p);
  };
  const back = () => (location.key !== "default" ? navigate(-1) : navigate(home));
  const scroll = (by: number) => pageScroller()?.scrollBy({ top: by });
  useKeys({
    ...(canEdit ? { e: edit } : {}),
    "mod+d": () => scroll(window.innerHeight / 2),
    "mod+u": () => scroll(-window.innerHeight / 2),
    g: () => pageScroller()?.scrollTo({ top: 0 }),
    G: () => pageScroller()?.scrollTo({ top: pageScroller()!.scrollHeight }),
    o: () => navigate(files),
    r: load,
    t: () => (rail ? toggleRail() : folded && toggleMenu()),
    u: up,
    "[": () => step(-1),
    "]": () => step(1),
    "/": () => asker.current?.focus(),
    a: () => read && askAbout(),
    Escape: () => {
      if (folder !== undefined) setFolder(undefined);
      else if (menu) setMenu(false);
      else if (tools.current?.hide()) return;
      else back();
    },
    h: back,
  });

  return (
    <div ref={page} className={`page artifact-page doc-reader${rail ? " with-rail" : ""}${folded ? " folded" : ""}`}>
      <div className="reader-col" ref={column}>
        {folded && <ContentsPill title={title ?? name} sections={sections} reading={reading} open={menu} toggle={toggleMenu} pill={pill} />}
        {read && where ? (
          <PathCrumbs from={space ? { to: "/documents", label: "Documents" } : { to: "/files", label: "Repos" }} on={on} title={title ?? name} open={folder} toggle={(id) => openFolder(folder === id ? undefined : id)} nav={crumbs} />
        ) : (
          <Crumbs steps={[{ to: home, label: space ? space.label : "Repos" }]} />
        )}
        {/* A document that starts with its own title is its own heading. */}
        {a && !/^(---\n[^]*?\n---\n\s*)?#\s/.test(a.text.trimStart()) && (
          <PageHead title={name} />
        )}
        {a && where && !mission && (
          <div className="doc-meta">
            {a.docKind ? (
              <span className="doc-kind" title={kindHint(a.docKind)}>
                {a.docKind.title}
              </span>
            ) : (
              <span className="doc-kind">Document</span>
            )}
            <Chip tone={where.worktree === GROUND ? "done" : "quiet"} title={where.worktree === GROUND ? "As it is on the repo's default branch (its ground)" : `As it is in the capsule ${where.repo}/${where.worktree}`}>
              <Icon name={where.worktree === GROUND ? "layers" : "box"} size={11} /> {where.worktree === GROUND ? "On the default branch" : `In ${where.worktree}`}
            </Chip>
            {a.page && (
              <Link to={docAddress(a.ref, a.page)} className="dim" title="It's edited in this page, which keeps its cells and passes">
                Edited in {a.page}
              </Link>
            )}
            {flights.map((f) => (
              <Link key={f.capsule} to={docAddress(`${where.repo}/${f.capsule}:${where.path}`)} className={chipClass("progress")} title={`It's changed in the capsule ${f.capsule}${f.pr ? `, in pull request #${f.pr.number}` : ""}: read it as it is there`}>
                <Icon name="box" size={11} /> In flight in {f.capsule}
                {f.pr ? ` · #${f.pr.number}` : ""}
              </Link>
            ))}
            <Copyable text={a.ref} className="mono doc-ref" title="Copy its ref">
              {a.ref.replace(":", " · ")}
            </Copyable>
            <span className="doc-actions">
              {where.worktree === GROUND && <PagePin r={{ kind: "file", repo: where.repo, worktree: GROUND, path: where.path }} compact />}
              {folded && sections.length > 1 && (
                <Button size="small" ref={contentsButton} data-contents-toggle aria-expanded={menu} onClick={toggleMenu} title="Its contents (t)">
                  <Icon name="list" size={12} /> Contents <Keys k="t" />
                </Button>
              )}
              <Button size="small" className="accent-button" onMouseDown={(e) => e.preventDefault()} onClick={askAbout} title="Ask about it, or about what's selected (a)">
                <Icon name="sparkle" size={12} /> Ask <Keys k="a" />
              </Button>
              <Button size="small" onClick={edit} unavailable={canEdit ? undefined : `Can't be edited here: ${a.uneditable}`} title="Edit it as a page: cells, questions and passes, saved back to the repo when you say (e)">
                <Icon name="pencil" size={12} /> Edit <Keys k="e" />
              </Button>
              <Link to={files} className={buttonClass({ size: "small" })} title="Open it in Files (o)">
                <Icon name="file" size={12} /> Files <Keys k="o" />
              </Link>
            </span>
          </div>
        )}
        {error && <ErrorCard title="Couldn't open this document" error={error} retry={load} />}
        {!a && !error && <Skeleton rows={10} />}
        {a && !mission && draft && (
          <Callout
            tone="attention"
            icon="pencil"
            title="You've an unsaved edit of this, from before it was edited as a page"
            actions={
              <Button variant="ghost" size="small" onClick={() => dropDraft("", a.ref).then(() => setDraft(null))}>
                Discard it
              </Button>
            }
          >
            Kept <Time iso={draft.at} as="relative" />. It's in <span className="mono">.ws/docs/drafts</span>: copy what you need into its page.
          </Callout>
        )}
        {a && !mission && earlier > 0 && (
          <p className="dim doc-earlier">
            {earlier === 1 ? "A thread was" : `${earlier} threads were`} made on this file before it was edited as a page. {earlier === 1 ? "It's" : "They're"} kept: <span className="mono">ws show {a.ref}</span> lists {earlier === 1 ? "it" : "them"}. Ask and change on its page now.
          </p>
        )}
        {a && !mission && (
          <article className="artifact-body prose" ref={body}>
            <Markdown text={a.text} at={place} headingLink={where ? copyLink : undefined} />
          </article>
        )}
        {a && !mission && cites && (cites.citedBy.length > 0 || cites.cites.length > 0) && (
          <aside className="doc-cites" aria-label="Cited by, and cites">
            {cites.citedBy.length > 0 && <CiteList title="Cited by" list={cites.citedBy} />}
            {cites.cites.length > 0 && <CiteList title="It cites" list={cites.cites} />}
          </aside>
        )}
        {a && !mission && talks.length > 0 && (
          <aside className="doc-cites doc-talks" aria-label="Conversations about it">
            <DocTalks list={talks} />
          </aside>
        )}
        {a?.path && !mission && (
          <p className="doc-path">
            <Copyable text={a.path} className="mono" title="Copy where it is" />
          </p>
        )}
        {read && sib && sib.of > 1 && <Pager sib={sib} to={toDoc} />}
        {read && a && (
          <AskBar
            key={a.ref}
            ref={asker}
            about={aboutDoc(a.ref, title ?? name)}
            suggestions={suggest}
            cite={cite}
            onOpen={() => {
              setFolder(undefined);
              setMenu(false);
            }}
          />
        )}
      </div>
      {read && a && <SelectionTools key={a.ref} ref={tools} within={body} ask={(p) => asker.current?.ask(p)} explain={(p) => asker.current?.ask(p, "Explain this passage")} link={(p) => copyLink(p.section?.anchor)} />}
      {rail && <ContentsRail sections={sections} reading={reading} hidden={railHidden} go={toSection} />}
      {folded && menu && (
        <ContentsMenu
          title={title ?? name}
          sections={sections}
          at={reading.at}
          anchor={() => (reading.past ? pill.current : (contentsButton.current ?? pill.current))}
          go={toSection}
          copy={() => {
            setMenu(false);
            copyLink();
          }}
          close={closeMenu}
        />
      )}
      {shelves && where && folder !== undefined && (
        <FolderMenu
          key={folder}
          shelves={shelves}
          id={folder}
          here={where.path}
          on={on}
          anchor={() => crumbs.current?.querySelector<HTMLElement>(`[data-folder-crumb="${CSS.escape(folder)}"]`) ?? crumbs.current}
          column={() => column.current}
          openIn={openIn}
          go={(p) => navigate(toDoc(p))}
          step={setFolder}
          close={closeFolder}
        />
      )}
    </div>
  );
}

// DocTalks is the conversations about the document: asks that were kept,
// each by its name, with the passage it was asked of, and when it last
// changed.
function DocTalks({ list }: { list: Talk[] }) {
  return (
    <section>
      <GroupHead count={list.length}>
        Conversations about it
      </GroupHead>
      <Rows>
        {list.map((t) => (
          <Row key={t.id}>
            <Link to={talkHref("", t.id)} className={rowLinkClass()} title={t.summary}>
              <Status s={talkStatus(t)} iconOnly size={14} className={rowStatusClass()} />
              <RowMain>
                <RowLine>
                  <RowTitle>{t.title || t.opening}</RowTitle>
                </RowLine>
                {(t.quote || t.section) && (
                  <RowSub className="dim">
                    {t.quote ? `“${clip(t.quote, 90)}”` : ""}
                    {t.section ? `${t.quote ? " in " : ""}§ ${t.section}` : ""}
                  </RowSub>
                )}
              </RowMain>
              <Time iso={t.updated || t.at} as="age" className="age" />
            </Link>
          </Row>
        ))}
      </Rows>
    </section>
  );
}

// CiteList is documents citing this one, or cited by it: each by its
// title, and where (by a link in it, or naming it).
function CiteList({ title, list }: { title: string; list: Citation[] }) {
  return (
    <section>
      <GroupHead count={list.length}>
        {title}
      </GroupHead>
      <Rows>
        {list.map((c) => (
          <Row key={`${c.repo}:${c.path}`}>
            <Link to={docTo(`${c.repo}:${c.path}`)} className={rowLinkClass()}>
              <Icon name="book" size={14} className={rowStatusClass("dim")} />
              <RowMain>
                <RowTitle>{c.title || c.path.split("/").pop()}</RowTitle>
              </RowMain>
              <span className="dim mono doc-cite-at" title={c.how === "link" ? "A link in it" : "It names it"}>
                {c.path.split("/").pop()}:{c.line}
              </span>
            </Link>
          </Row>
        ))}
      </Rows>
    </section>
  );
}
