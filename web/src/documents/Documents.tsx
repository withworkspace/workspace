import { useEffect, useMemo, useRef, useState } from "react";
import { Link, useLocation, useNavigate, useParams, useSearchParams } from "react-router";
import { type DocHit, docTo, getSpace, getSpaceActivity, getSpaces, pageAddress, searchDocuments, type SpaceActivity, type SpaceDoc, type SpaceKind, spaceTo, type Workspace } from "../api";
import { useKeys } from "../keys";
import { useLoaded } from "../load";
import { NewAsKind, PageRow, useStartPage } from "../pages/Pages";
import { Icon } from "../ui/Icon";
import { Dialog, Empty, ErrorCard, humanError, Keys, Loading, useMedia, useTitle } from "../ui/kit";
import { TLink } from "../ui/TLink";
import { ActivityFold, Fresh, InFlight, inFlightPaths, Lately } from "./Activity";
import { acrossSpaces } from "./happening";
import { SpaceCard, useSpaceDetails } from "./Cards";
import { HitLine, InFlightChip, KindHead, KindList, libraryID, type Picking, pickOn, sorted, useKindOrder } from "./Library";
import { type Placed, placePages } from "./place";
import { Preview } from "./Preview";
import { hitTo, Results, SearchBox, useDocSearch } from "./Search";
import { Chip } from "../ui/Chip";
import { Count } from "../ui/Count";
import { Button } from "../ui/Button";
import { PageHead } from "../ui/PageHead";
import { GroupHead } from "../ui/GroupHead";

// Documents: where the workspace's writing is read. Local is the pages
// that aren't in a space (written here, on this machine); each space is a
// repo of documents, or a folder of one, read from its default branch. A
// page that will live in a space is listed there, as your draft.
//
// Documents takes the window's width: what changed and what's in flight
// across every space on top, each space as a card below (its kinds and
// how it stands), and Local beside them. A space's page is its list and,
// beside it, the document picked, each scrolling by itself.

export const documentsKeys: [string, string][] = [
  ["n", "Start a page"],
  ["N", "Start a page as one of the workspace's kinds of document"],
  ["/", "Search the documents"],
  ["j / k", "Move between spaces and pages"],
  ["⏎ or o", "Open it"],
];

export const spaceKeys: [string, string][] = [
  ["j / k", "Move between documents: the one picked is shown beside the list"],
  ["⏎ or o", "Read it"],
  ["/", "Search the space"],
  ["n", "Start a page here"],
  ["esc", "Clear the search, or back to Documents"],
];

// A row the cursor can be on: a page, or a space.
type Target = { id: string; to: string };

export function DocumentsPage({ workspace }: { workspace: Workspace }) {
  useTitle("Documents");
  const navigate = useNavigate();
  // Answered at once from what ws keeps: read again when it's read a
  // space again and found something new (the workspace's spacesSeen).
  const { value, error, reload } = useLoaded(getSpaces, [workspace.spacesSeen], { keep: true });
  const spaces = useMemo(() => value?.spaces ?? [], [value]);
  const kinds = useMemo(() => workspace.kinds ?? [], [workspace.kinds]);
  const placed = useMemo(() => placePages(workspace.pages ?? [], spaces, kinds), [workspace.pages, spaces, kinds]);
  const local = placed.filter((p) => !p.repo);
  const details = useSpaceDetails(spaces, workspace.spacesSeen);
  const across = useMemo(() => {
    const activity = new Map<string, SpaceActivity>();
    for (const [repo, d] of details) if (d.activity) activity.set(repo, d.activity);
    return acrossSpaces(spaces, activity);
  }, [spaces, details]);
  const spaceOf = (repo: string) => (spaces.length > 1 ? spaces.find((s) => s.repo === repo) : undefined);
  const { start, starting } = useStartPage();
  const [asKind, setAsKind] = useState(false);
  const [index, setIndex] = useState(0);
  // Searching: what's found in place of the lists.
  const [q, setQ] = useState("");
  const [kind, setKind] = useState("");
  const [stand, setStand] = useState("");
  const search = useRef<HTMLInputElement>(null);
  const found = useDocSearch(q, kind, stand);
  const searching = q.trim() !== "";
  const [hit, setHit] = useState(0);
  useEffect(() => setHit(0), [found.hits]);
  const moveHit = (i: number) => {
    const n = Math.min(Math.max(i, 0), (found.hits?.length ?? 1) - 1);
    setHit(n);
    document.getElementById(`hit-${n}`)?.scrollIntoView({ block: "nearest" });
  };
  const searchKeys = (e: React.KeyboardEvent) => {
    const go: Record<string, () => void> = {
      ArrowDown: () => moveHit(hit + 1),
      ArrowUp: () => moveHit(hit - 1),
      Enter: () => found.hits?.[hit] && navigate(hitTo(found.hits[hit])),
      Escape: () => (q ? setQ("") : search.current?.blur()),
    };
    if (go[e.key]) {
      e.preventDefault();
      go[e.key]();
    }
  };

  // The cursor goes down the spaces, then Local, as they read.
  const targets: Target[] = [
    ...spaces.map((s) => ({ id: `space-${s.repo}`, to: spaceTo(s.repo) })),
    ...local.map((p) => ({ id: `page-${p.page.id}`, to: pageAddress(p.page) })),
  ];
  const move = (i: number) => {
    const n = Math.min(Math.max(i, 0), targets.length - 1);
    setIndex(n);
    document.getElementById(targets[n]?.id ?? "")?.scrollIntoView({ block: "nearest" });
  };
  const open = () => targets[index] && navigate(targets[index].to);
  useKeys({
    n: () => start(),
    ...(kinds.length ? { N: () => setAsKind(true) } : {}),
    "/": () => search.current?.focus(),
    ...(searching
      ? {
          j: () => moveHit(hit + 1),
          k: () => moveHit(hit - 1),
          Enter: () => found.hits?.[hit] && navigate(hitTo(found.hits[hit])),
          o: () => found.hits?.[hit] && navigate(hitTo(found.hits[hit])),
          Escape: () => setQ(""),
        }
      : {
          j: () => move(index + 1),
          ArrowDown: () => move(index + 1),
          k: () => move(index - 1),
          ArrowUp: () => move(index - 1),
          Enter: open,
          o: open,
        }),
  });
  const filling = local.reduce((n, p) => n + (p.page.filling ? 1 : 0), 0);

  return (
    <div className="page documents-page">
      <PageHead
        title="Documents"
        actions={
          <>
            {kinds.length > 0 && (
              <Button
                onClick={() => setAsKind(true)}
                disabled={starting}
                title="Start a page as one of the workspace's kinds of document: it's written as its guide says, and filed where the kind says"
              >
                New as… <Keys k="N" />
              </Button>
            )}
            <Button variant="primary" onClick={() => start()} disabled={starting}>
              <Icon name="plus" /> New page <Keys k="n" />
            </Button>
          </>
        }
      />
      {asKind && <NewAsKind kinds={kinds} busy={starting} onStart={(title, kind) => start(title, kind)} onClose={() => setAsKind(false)} />}
      {spaces.length > 0 && (
        <SearchBox q={q} setQ={setQ} inputRef={search} spaces={spaces} kind={kind} setKind={setKind} state={stand} setState={setStand} onKey={searchKeys} />
      )}
      {searching ? (
        <Results hits={found.hits} more={found.more} error={found.error} index={hit} setIndex={setHit} />
      ) : (
        <>
          {(across.lately.length > 0 || across.inFlight.length > 0) && (
            <div className="two-up documents-across">
              <Lately
                lately={across.lately}
                spaceOf={spaceOf}
                label={
                  <>
                    Changed lately{spaces.length > 1 && <span className="dim documents-sub"> · across {spaces.length} spaces</span>}
                  </>
                }
              />
              <InFlight flights={across.inFlight} spaceOf={spaceOf} />
            </div>
          )}
          <div className="documents-grid">
            <section aria-label="Spaces">
              <GroupHead count={spaces.length}>
                Spaces
              </GroupHead>
              {error && !value && <ErrorCard title="Couldn't read the spaces" error={error} retry={reload} />}
              {value?.problem && <div className="banner tone-attention">Some of it may be behind: {value.problem}</div>}
              {!value && !error && <Loading what="Reading the spaces…" />}
              {value && spaces.length === 0 && (
                <Empty icon="file" title="No spaces of documents">
                  A repo of documents (ADRs, proposals, guides) is read here, by its titles, rather than in Files: say <code>documents = true</code> under its{" "}
                  <code>[repos.&lt;name&gt;]</code> in <code>ws.toml</code>, or name it as <code>[missions] records</code>.
                </Empty>
              )}
              {spaces.length > 0 && (
                <div className={`space-cards${spaces.length <= 2 ? " few" : ""}`}>
                  {spaces.map((s, i) => (
                    <SpaceCard
                      key={s.repo}
                      id={`space-${s.repo}`}
                      space={s}
                      detail={details.get(s.repo)}
                      drafts={placed.filter((p) => p.repo === s.repo).length}
                      selected={i === index}
                      onHover={() => i !== index && setIndex(i)}
                    />
                  ))}
                </div>
              )}
            </section>
            <section aria-label="Local">
              <GroupHead title="Pages written here, on this machine, that aren't in a space: no one else sees them. File as… is the way out.">
                Local <Count>{local.length}</Count>
                {filling > 0 && <span className="dim documents-sub">· {filling} filling</span>}
              </GroupHead>
              {local.length === 0 ? (
                <p className="dim documents-empty">
                  No pages here. A page is somewhere to write a problem down and work it out:{" "}
                  <Button variant="link" onClick={() => start()}>
                    start one
                  </Button>
                  .
                </p>
              ) : (
                <ul className="rows pages" role="listbox" aria-label="Local pages">
                  {local.map((p, j) => {
                    const i = spaces.length + j;
                    return (
                      <PageRow
                        key={p.page.id}
                        page={p.page}
                        kinds={kinds}
                        selected={i === index}
                        onHover={() => i !== index && setIndex(i)}
                        note={
                          p.page.source && (
                            <Chip tone="quiet" title="Its file isn't in a space of documents">
                              {p.page.source.ref.split(":")[0]}
                            </Chip>
                          )
                        }
                      />
                    );
                  })}
                </ul>
              )}
            </section>
          </div>
        </>
      )}
    </div>
  );
}

// SpacePage is a space: its list down the left (your drafts in it first,
// then its documents by kind, the rest by folder) and, beside it, the
// document picked, as the reader shows it. Above the list, a search of
// the space, what's happening in it folded to a line, and a tab for each
// kind that jumps the list there. The page doesn't scroll: the list and
// the preview do, each by itself, and the edge between them is dragged.
// On a narrow window it's the list alone, each document opened to read.
export function SpacePage({ workspace }: { workspace: Workspace }) {
  const { repo = "" } = useParams();
  const navigate = useNavigate();
  // Both answered at once from what ws keeps, and read again when it's
  // found the space other than it said (the workspace's spacesSeen). What's
  // happening that's not been read yet is nothing to show.
  const { value, error, reload } = useLoaded(() => getSpace(repo), [repo, workspace.spacesSeen], { keep: true });
  const { value: said, error: activityError, reload: reloadActivity } = useLoaded(() => getSpaceActivity(repo), [repo, workspace.spacesSeen], { keep: true });
  const activity = said?.unread ? undefined : said;
  const flights = useMemo(() => inFlightPaths(activity), [activity]);
  const space = value?.space;
  useTitle(space ? space.label : repo);
  const kinds = useMemo(() => workspace.kinds ?? [], [workspace.kinds]);
  const drafts = useMemo(() => (space ? placePages(workspace.pages ?? [], [space], kinds).filter((p) => p.repo === space.repo) : []), [workspace.pages, space, kinds]);
  const wide = useMedia("(min-width: 1021px)");
  // Searching the space: the lists keep to what's found, each row saying
  // where in it.
  const [q, setQ] = useState("");
  const search = useRef<HTMLInputElement>(null);
  const found = useSpaceSearch(repo, q);
  const hits = q.trim() ? found.hits : undefined;
  const matches = (d: SpaceDoc) => !hits || hits.has(d.path);
  // Its library: a list for each kind whose path is in it, and the rest by
  // folder, under Other.
  const library = useMemo(() => space?.kinds ?? [], [space]);
  const order0 = useKindOrder();
  const byKind = useMemo(() => {
    const m = new Map<string, SpaceDoc[]>();
    for (const d of value?.docs ?? []) if (d.kind && library.some((k) => k.name === d.kind)) m.set(d.kind, [...(m.get(d.kind) ?? []), d]);
    return m;
  }, [value, library]);
  const shown = (k: SpaceKind) =>
    sorted(
      (byKind.get(k.name) ?? []).filter((d) => (!order0.standOf(k.name) || d.state === order0.standOf(k.name)) && matches(d)),
      order0.sortOf(k.name),
      k.states,
    );
  const others = useMemo(() => (value?.docs ?? []).filter((d) => !(d.kind && library.some((k) => k.name === d.kind))), [value, library]);
  const folders = useMemo(() => byFolder(others.filter(matches), space?.folder ?? ""), [others, space, hits]); // eslint-disable-line react-hooks/exhaustive-deps
  const kindsShown = library.filter((k) => !hits || shown(k).length > 0);
  const docsShown: SpaceDoc[] = [...kindsShown.flatMap((k) => shown(k)), ...folders.flatMap(([, docs]) => docs)];
  // What the cursor goes down: your drafts, then the documents as listed.
  const order: (Target & { doc?: SpaceDoc })[] = [
    ...drafts.map((p) => ({ id: `page-${p.page.id}`, to: pageAddress(p.page) })),
    ...docsShown.map((d) => ({ id: d.kind && library.some((k) => k.name === d.kind) ? libraryID(d.path) : docID(d.path), to: docTo(`${repo}:${d.path}`), doc: d })),
  ];
  // The document picked: in the address (?doc=), else the first listed.
  const [params, setParams] = useSearchParams();
  const asked = params.get("doc") ?? "";
  const picked = docsShown.find((d) => d.path === asked) ?? docsShown[0];
  const pick = (d: SpaceDoc) =>
    setParams(
      (p) => {
        const n = new URLSearchParams(p);
        n.set("doc", d.path);
        return n;
      },
      { replace: true },
    );
  const { start, starting } = useStartPage();
  const [asKind, setAsKind] = useState(false);
  const [index, setIndex] = useState(0);
  // The cursor is on the document picked, when it's moved there some other way.
  const pickedAt = picked ? order.findIndex((o) => o.doc?.path === picked.path) : -1;
  useEffect(() => {
    if (wide && pickedAt >= 0 && order[index]?.doc?.path !== picked?.path) setIndex(pickedAt);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pickedAt, wide]);
  const move = (i: number) => {
    const n = Math.min(Math.max(i, 0), order.length - 1);
    setIndex(n);
    const d = order[n]?.doc;
    if (wide && d) pick(d);
    document.getElementById(order[n]?.id ?? "")?.scrollIntoView({ block: "nearest" });
  };
  const open = () => order[index] && navigate(order[index].to);
  const back = () => (q ? setQ("") : navigate("/documents"));
  useKeys({
    j: () => move(index + 1),
    ArrowDown: () => move(index + 1),
    k: () => move(index - 1),
    ArrowUp: () => move(index - 1),
    Enter: open,
    o: open,
    "/": () => search.current?.focus(),
    Escape: back,
    n: () => setAsKind(true),
  });
  const searchKeys = (e: React.KeyboardEvent<HTMLInputElement>) => {
    const go: Record<string, () => void> = {
      ArrowDown: () => move(index + 1),
      ArrowUp: () => move(index - 1),
      Enter: open,
      Escape: () => (q ? setQ("") : search.current?.blur()),
    };
    if (go[e.key]) {
      e.preventDefault();
      go[e.key]();
    }
  };
  const picking: Picking | undefined = wide
    ? {
        picked: picked?.path,
        hits,
        onPick: (d) => {
          const i = order.findIndex((o) => o.doc?.path === d.path);
          if (i >= 0) setIndex(i);
          pick(d);
        },
        onOpen: (d) => navigate(docTo(`${repo}:${d.path}`)),
      }
    : hits && { hits, onPick: (d) => navigate(docTo(`${repo}:${d.path}`)), onOpen: (d) => navigate(docTo(`${repo}:${d.path}`)) };
  // The list and the edge between it and the preview.
  const list = useRef<HTMLDivElement>(null);
  const [listWidth, setListWidth] = useListWidth();
  const [foldOpen, setFoldOpen] = useState(false);
  // The tabs: a kind each (Other, for what's of none), else a folder each.
  const tabs: { id: string; label: string; n: number; total: number }[] = library.length
    ? [
        ...library.map((k) => ({ id: listID("kind", k.name), label: k.title, n: shown(k).length, total: (byKind.get(k.name) ?? []).length })),
        ...(others.length ? [{ id: "list-other", label: "Other", n: folders.reduce((n, [, d]) => n + d.length, 0), total: others.length }] : []),
      ]
    : folders.map(([f, docs]) => ({ id: listID("folder", f), label: f || "(top)", n: docs.length, total: docs.length }));
  const [tab, setTab] = useState(0);
  const spy = () => {
    const box = list.current;
    if (!box) return;
    const top = box.getBoundingClientRect().top;
    let at = 0;
    tabs.forEach((t, i) => {
      const el = document.getElementById(t.id);
      if (el && el.getBoundingClientRect().top - top <= 12) at = i;
    });
    // At the foot of a list that scrolls, the last that can be reached.
    if (box.scrollHeight > box.clientHeight + 2 && box.scrollTop + box.clientHeight >= box.scrollHeight - 2) at = tabs.map((t) => t.n > 0).lastIndexOf(true);
    setTab(Math.max(0, at));
  };
  const jump = (id: string) => {
    const el = document.getElementById(id);
    const box = list.current;
    if (!el || !box) return;
    const still = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    if (box.scrollHeight > box.clientHeight) box.scrollTo({ top: el.getBoundingClientRect().top - box.getBoundingClientRect().top + box.scrollTop, behavior: still ? "auto" : "smooth" });
    else el.scrollIntoView({ block: "start", behavior: still ? "auto" : "smooth" });
  };
  // Opened at one of its lists (spaceTo: the reader's crumbs, "Open in the
  // space", a space's card): there, its first document picked. A folder
  // with no documents of its own is the first folder below it that has
  // some.
  const { hash } = useLocation();
  const wentTo = useRef("");
  useEffect(() => {
    if (!value || !space || !hash || wentTo.current === hash) return;
    wentTo.current = hash;
    const want = new URLSearchParams(hash.slice(1));
    const kind = library.find((k) => k.name === want.get("kind"));
    const folder = want.get("folder")?.slice((space.folder ?? "").length);
    let to: { id: string; first?: SpaceDoc } | undefined;
    if (kind) to = { id: listID("kind", kind.name), first: shown(kind)[0] };
    else if (folder !== undefined) {
      const f = folders.find(([x]) => x === folder) ?? folders.find(([x]) => x.startsWith(folder));
      if (f) to = { id: listID("folder", f[0]), first: f[1][0] };
    }
    if (!to) return;
    if (to.first) {
      const i = order.findIndex((o) => o.doc?.path === to.first!.path);
      if (i >= 0) setIndex(i);
      if (wide) pick(to.first);
    }
    const id = to.id;
    requestAnimationFrame(() => jump(id));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value, hash]);
  useEffect(spy, [value, hits, foldOpen]); // eslint-disable-line react-hooks/exhaustive-deps
  // Opened at a document (?doc=), the list's scrolled to it, once.
  const shownAt = useRef("");
  useEffect(() => {
    // Once what's over the list is there too, so it doesn't move it after.
    if (!value || (!activity && !activityError) || !asked || shownAt.current === repo || pickedAt < 0) return;
    shownAt.current = repo;
    requestAnimationFrame(() => document.getElementById(order[pickedAt].id)?.scrollIntoView({ block: "center" }));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value, activity, activityError, pickedAt]);

  if (error && !value) {
    return (
      <div className="page narrow">
        <ErrorCard title={`Couldn't read ${repo}`} error={error} retry={reload} />
        <p>
          <Link to="/documents">Back to Documents</Link>
        </p>
      </div>
    );
  }
  if (!value || !space) return <Loading what="Reading the space…" />;
  let at = drafts.length;
  const kindAt = (k: SpaceKind) => {
    const first = at;
    at += shown(k).length;
    return first;
  };
  const searched = !!hits;
  return (
    <div className="page documents-page space-page">
      <PageHead
        className="space-head"
        lead={
          <>
            <Link to="/documents" className="space-back" title="Back to Documents (esc)">
              <Icon name="arrow-left" size={13} /> Documents
            </Link>
            <span className="space-slash" aria-hidden>
              /
            </span>
          </>
        }
        title={
          <>
            <span className="repo-dot" style={{ background: space.color }} /> {space.label}
            {space.folder && <span className="mono dim"> {space.folder}</span>}
          </>
        }
        actions={
          <Button
            variant="primary"
            onClick={() => setAsKind(true)}
            disabled={starting || !space.cloned}
            title={library.length ? `Start a page as one of ${space.label}'s kinds of document: written as its guide says, and filed here` : `Start a page to file in one of ${space.label}'s folders`}
          >
            <Icon name="plus" /> New <Keys k="n" />
          </Button>
        }
      >
        <span className="space-count" title="As on its default branch">
          {space.count} {space.count === 1 ? "document" : "documents"}
        </span>
        {activity && (
          <Fresh
            repo={repo}
            fresh={activity.fresh}
            onMoved={() => {
              reload();
              reloadActivity();
            }}
          />
        )}
      </PageHead>
      {asKind && library.length > 0 && (
        <NewAsKind kinds={kinds.filter((k) => library.some((x) => x.name === k.name))} busy={starting} onStart={(title, kind) => start(title, kind)} onClose={() => setAsKind(false)} />
      )}
      {asKind && library.length === 0 && (
        <NewInFolder repo={repo} label={space.label} root={space.folder ?? ""} folders={folders.map(([f]) => f)} busy={starting} onStart={(title, to) => start(title, "", to)} onClose={() => setAsKind(false)} />
      )}
      {value.problem && <div className="banner tone-attention">It may be behind: {value.problem}</div>}
      {!space.cloned ? (
        <Empty icon="box" title="Not cloned here">
          <code>ws setup {space.repo}</code> clones it.
        </Empty>
      ) : (value.docs ?? []).length === 0 ? (
        <Empty icon="file" title="No documents yet">
          It has no markdown{space.folder ? ` under ${space.folder}` : ""} on its default branch.
        </Empty>
      ) : (
        <div className="space-split" style={listWidth ? ({ "--space-list-w": `${listWidth}px` } as React.CSSProperties) : undefined}>
          <div className="space-list">
            <div className="space-list-top">
              <label className="doc-search-box space-search">
                <Icon name="search" size={14} className="dim" />
                <input ref={search} value={q} placeholder={`Search ${space.label}: what its documents say`} aria-label={`Search ${space.label}`} onChange={(e) => setQ(e.target.value)} onKeyDown={searchKeys} />
                {q.trim() ? (
                  <span className="dim space-search-count">{found.error ? "Couldn't search" : !found.hits ? "Searching…" : `${found.hits.size} of ${space.count}`}</span>
                ) : (
                  <Keys k="/" />
                )}
              </label>
              {activity && <ActivityFold repo={repo} activity={activity} open={foldOpen} setOpen={setFoldOpen} />}
              {activityError && !activity && <p className="dim documents-empty">Couldn't read what's happening in it.</p>}
              {tabs.length > 1 && (
                <nav className="segmented space-tabs" aria-label={library.length ? "Jump to a kind" : "Jump to a folder"}>
                  {tabs.map((t, i) => (
                    <Button key={t.id} aria-pressed={i === tab} disabled={t.n === 0} onClick={() => jump(t.id)}>
                      {t.label} <Count className="space-tab-count">{searched && t.n !== t.total ? `${t.n}/${t.total}` : t.total}</Count>
                    </Button>
                  ))}
                </nav>
              )}
            </div>
            <div className="space-list-scroll" ref={list} onScroll={spy}>
              {drafts.length > 0 && (
                <section aria-label="Your drafts">
                  <GroupHead title="Pages of yours that will live here: only you see them until they're saved or filed" count={drafts.length}>
                    Your drafts
                  </GroupHead>
                  <ul className="rows pages" role="listbox" aria-label="Your drafts">
                    {drafts.map((p, i) => (
                      <PageRow key={p.page.id} page={p.page} kinds={kinds} selected={i === index} onHover={() => {}} note={<DraftNote p={p} />} />
                    ))}
                  </ul>
                </section>
              )}
              {searched && !docsShown.length && <p className="dim documents-empty">Nothing in {space.label} says that.</p>}
              {kindsShown.map((k) => (
                <section key={k.name} id={listID("kind", k.name)} aria-label={k.title}>
                  <KindHead
                    kind={k}
                    docs={byKind.get(k.name) ?? []}
                    sort={order0.sortOf(k.name)}
                    setSort={(s) => order0.setSort(k.name, s)}
                    stand={order0.standOf(k.name)}
                    setStand={(s) => order0.setStand(k.name, s)}
                    found={searched ? shown(k).length : undefined}
                  />
                  <KindList repo={repo} kind={k} docs={shown(k)} at={kindAt(k)} index={index} setIndex={wide ? undefined : setIndex} flights={flights} picking={picking} />
                </section>
              ))}
              {kindsShown.length > 0 && folders.length > 0 && (
                <h2 className="other-head" id="list-other">
                  Other
                </h2>
              )}
              {/* Where Other's tab goes when no kind's list is shown above it. */}
              {library.length > 0 && kindsShown.length === 0 && folders.length > 0 && <span id="list-other" />}
              {folders.map(([folder, docs]) => (
                <section key={folder} id={listID("folder", folder)} aria-label={folder || "Top"}>
                  <GroupHead count={docs.length}>
                    <span className="mono">{folder || "(top)"}</span>
                  </GroupHead>
                  <ul className="rows space-docs" role="listbox" aria-label={folder || "Top"}>
                    {docs.map((d) => {
                      const i = at++;
                      const hit = hits?.get(d.path);
                      return (
                        <li
                          key={d.path}
                          id={docID(d.path)}
                          role="option"
                          aria-selected={i === index}
                          className={`row${i === index ? " selected" : ""}${picking?.picked === d.path ? " picked" : ""}`}
                          onMouseMove={wide ? undefined : () => i !== index && setIndex(i)}
                        >
                          <TLink to={docTo(`${repo}:${d.path}`)} className="row-link" onClick={picking && pickOn(picking, d)} onDoubleClick={picking && (() => picking.onOpen(d))}>
                            <Icon name="file" size={16} className="row-status" />
                            <span className="row-main">
                              <span className="row-line">
                                <span className="title">{d.title}</span>
                                {flights.has(d.path) && <InFlightChip capsules={flights.get(d.path)!} />}
                              </span>
                              {hit && <HitLine hit={hit} />}
                            </span>
                            <span className="dim mono doc-file">{d.path.split("/").pop()}</span>
                          </TLink>
                        </li>
                      );
                    })}
                  </ul>
                </section>
              ))}
            </div>
          </div>
          {wide && (
            <>
              <ListEdge width={listWidth} setWidth={setListWidth} />
              {picked ? (
                <Preview
                  key={picked.path}
                  repo={repo}
                  doc={picked}
                  kind={library.find((k) => k.name === picked.kind)}
                  activity={activity}
                  flights={flights.get(picked.path)}
                  anchor={hits?.get(picked.path)?.anchor}
                />
              ) : (
                <aside className="doc-preview doc-preview-none">
                  <p className="dim">{searched ? "Nothing found to show." : "Nothing to show."}</p>
                </aside>
              )}
            </>
          )}
        </div>
      )}
    </div>
  );
}

// useSpaceSearch is what a search of one space finds, by document (each
// led to the section that says it best), as it's typed (a moment after
// the last key).
function useSpaceSearch(repo: string, q: string) {
  const [hits, setHits] = useState<Map<string, DocHit>>();
  const [error, setError] = useState<string>();
  useEffect(() => {
    if (!q.trim()) {
      setHits(undefined);
      setError(undefined);
      return;
    }
    let stale = false;
    const t = setTimeout(() => {
      searchDocuments(q, { repos: [repo], limit: 500 }).then(
        (r) => {
          if (stale) return;
          setHits(new Map(r.hits.filter((h) => h.repo === repo).map((h) => [h.path, h])));
          setError(undefined);
        },
        (e: Error) => !stale && setError(humanError(e.message)),
      );
    }, 200);
    return () => {
      stale = true;
      clearTimeout(t);
    };
  }, [repo, q]);
  return { hits, error };
}

// The list's width, as dragged (kept in this browser); none, as it was
// laid out: five parts to the preview's seven.
const LIST_MIN = 280;
const PREVIEW_MIN = 380;

function useListWidth(): [number | undefined, (w: number | undefined) => void] {
  const [w, setW] = useState<number | undefined>(() => {
    try {
      const n = Number(localStorage.getItem("ws:space-list-w"));
      return n >= LIST_MIN ? n : undefined;
    } catch {
      return undefined;
    }
  });
  const set = (n: number | undefined) => {
    setW(n);
    try {
      if (n) localStorage.setItem("ws:space-list-w", `${n}`);
      else localStorage.removeItem("ws:space-list-w");
    } catch {
      // fine
    }
  };
  return [w, set];
}

// ListEdge is the edge between a space's list and its preview: dragged,
// or ←/→ on it, to make the list wider or narrower; a double click puts
// it back.
function ListEdge({ width, setWidth }: { width?: number; setWidth: (w: number | undefined) => void }) {
  const clamp = (split: HTMLElement, w: number) => Math.round(Math.min(Math.max(w, LIST_MIN), split.getBoundingClientRect().width - PREVIEW_MIN - 24));
  const listOf = (edge: HTMLElement) => edge.parentElement!.querySelector<HTMLElement>(".space-list")!;
  const drag = (e: React.PointerEvent<HTMLDivElement>) => {
    e.preventDefault();
    const edge = e.currentTarget;
    const split = edge.parentElement!;
    const left = split.getBoundingClientRect().left;
    edge.setPointerCapture(e.pointerId);
    edge.classList.add("dragging");
    let w: number | undefined;
    const move = (ev: PointerEvent) => {
      w = clamp(split, ev.clientX - left - 12);
      split.style.setProperty("--space-list-w", `${w}px`);
    };
    const up = () => {
      edge.removeEventListener("pointermove", move);
      edge.classList.remove("dragging");
      if (w) setWidth(w);
    };
    edge.addEventListener("pointermove", move);
    edge.addEventListener("pointerup", up, { once: true });
    edge.addEventListener("pointercancel", up, { once: true });
  };
  const key = (e: React.KeyboardEvent<HTMLDivElement>) => {
    if (e.key !== "ArrowLeft" && e.key !== "ArrowRight") return;
    e.preventDefault();
    e.stopPropagation();
    const now = width ?? listOf(e.currentTarget).getBoundingClientRect().width;
    setWidth(clamp(e.currentTarget.parentElement!, now + (e.key === "ArrowRight" ? 24 : -24)));
  };
  return (
    <div
      className="space-edge"
      role="separator"
      aria-orientation="vertical"
      aria-label="The list's width"
      tabIndex={0}
      title="Drag to make the list wider or narrower; double-click to put it back"
      onPointerDown={drag}
      onDoubleClick={() => {
        setWidth(undefined);
      }}
      onKeyDown={key}
    />
  );
}

function DraftNote({ p }: { p: Placed }) {
  switch (p.why) {
    case "to":
      return (
        <Chip tone="quiet" title={`To be filed in ${p.path || "its top"}, not filed yet`}>
          Not filed · {p.path || "top"}
        </Chip>
      );
    case "unsaved":
      return (
        <Chip tone="attention" title={`Your edits to ${p.path}, not saved to the repo yet`}>
          Unsaved
        </Chip>
      );
    case "filed":
      return (
        <Chip tone="progress" title={`Filed as ${p.path}, in a capsule: it's here once it lands`}>
          Filed, not landed
        </Chip>
      );
    default:
      return (
        <Chip tone="quiet" title="Started as one of this space's kinds, not filed yet">
          Not filed
        </Chip>
      );
  }
}

const docID = (path: string) => `doc-${path.replace(/[^\w-]/g, "_")}`;

// listID is a list's section in a space's page: a kind's, or a folder's
// (below the space's own folder, with its slash; "" for its top).
const listID = (of: "kind" | "folder", name: string) =>
  `list-${of}-${name.replace(/[^\w-]/g, "_")}`;

// NewInFolder starts a page to be filed in one of a space's folders (a
// space with no kinds of document): a folder it has, or a new one, and a
// title its file's named from. File as… files it there.
function NewInFolder({
  repo,
  label,
  root,
  folders,
  busy,
  onStart,
  onClose,
}: {
  repo: string;
  label: string;
  root: string;
  folders: string[];
  busy: boolean;
  onStart: (title: string, to: string) => void;
  onClose: () => void;
}) {
  const [folder, setFolder] = useState(folders.find((f) => f !== "") ?? "");
  const [title, setTitle] = useState("");
  const clean = folder
    .trim()
    .replace(/^\/+/, "")
    .replace(/\/*$/, folder.trim() ? "/" : "");
  const go = () =>
    title.trim() && onStart(title.trim(), `${repo}:${root}${clean}`);
  const name =
    title
      .toLowerCase()
      .split(/[^a-z0-9]+/)
      .filter(Boolean)
      .join("-") || "its-title";
  return (
    <Dialog title={`New in ${label}`} onClose={onClose} className="new-as" wide>
      <div className="file-as-fields">
        <label>
          <span>Folder</span>
          <input
            list="space-folders"
            value={folder}
            placeholder="(its top)"
            aria-label="Folder"
            onChange={(e) => setFolder(e.target.value)}
          />
          <datalist id="space-folders">
            {folders.filter(Boolean).map((f) => (
              <option key={f} value={f} />
            ))}
          </datalist>
        </label>
        <label>
          <span>Title</span>
          <input
            autoFocus
            value={title}
            placeholder="What it's about"
            aria-label="Title"
            onChange={(e) => setTitle(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                go();
              }
            }}
          />
        </label>
      </div>
      <p className="dim file-as-what">
        Once it's written, <em>File as…</em> files it at{" "}
        <span className="mono">{`${repo} · ${root}${clean}${name}.md`}</span>,
        as {label} lands its changes.
      </p>
      <div className="dialog-actions">
        <Button variant="ghost" onClick={onClose}>
          Cancel
        </Button>
        <Button
          variant="primary"
          onClick={go}
          disabled={busy || !title.trim()}
        >
          Start it <Keys k="⏎" />
        </Button>
      </div>
    </Dialog>
  );
}

// byFolder is a space's documents in their folders (relative to the
// space's own), the top first, then by name; each folder's by title.
function byFolder(docs: SpaceDoc[], root: string): [string, SpaceDoc[]][] {
  const by = new Map<string, SpaceDoc[]>();
  for (const d of docs) {
    const rel = d.path.slice(root.length);
    const at = rel.lastIndexOf("/");
    const folder = at < 0 ? "" : rel.slice(0, at + 1);
    by.set(folder, [...(by.get(folder) ?? []), d]);
  }
  // By path: a numbered or dated document's name keeps them in order.
  for (const list of by.values())
    list.sort((a, b) =>
      a.path.localeCompare(b.path, undefined, { numeric: true }),
    );
  return [...by].sort(([a], [b]) =>
    a === "" ? -1 : b === "" ? 1 : a.localeCompare(b),
  );
}
