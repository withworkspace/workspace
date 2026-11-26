import { useEffect, useMemo, useRef, useState } from "react";
import { Link, useLocation, useNavigate, useParams, useSearchParams } from "react-router";
import { docTo, getSpace, getSpaceActivity, pageAddress, type SpaceDoc, type SpaceKind } from "../api";
import { useKeys, useShortcut } from "../keys";
import { useLoaded } from "../load";
import { NewAsKind, PageRow, useStartPage } from "../pages/Pages";
import { Banner } from "../ui/Banner";
import { Button } from "../ui/Button";
import { Empty } from "../ui/Empty";
import { ErrorCard } from "../ui/ErrorCard";
import { GroupHead } from "../ui/GroupHead";
import { Icon } from "../ui/Icon";
import { hintOnClick } from "../ui/keyHint";
import { Keys } from "../ui/Keys";
import { Loading } from "../ui/Loading";
import { Rows } from "../ui/Rows";
import { useMedia } from "../ui/useMedia";
import { useTitle } from "../ui/useTitle";
import { useWorkspace } from "../workspace/store";
import { ActivityFold } from "./ActivityFold";
import { DocumentsEmpty } from "./DocumentsEmpty";
import { DocumentsSearchBox } from "./DocumentsSearchBox";
import { DraftNote } from "./DraftNote";
import { FolderList } from "./FolderList";
import { Fresh } from "./Fresh";
import { inFlightPaths } from "./inFlightPaths";
import { KindHead } from "./KindHead";
import { KindList } from "./KindList";
import type { Picking } from "./picking";
import { placePages } from "./place";
import { Preview } from "./Preview";
import { sorted } from "./sorted";
import { byFolder, docID, libraryID, listID } from "./space";
import { SpaceEdge } from "./SpaceEdge";
import { SpaceFrame } from "./SpaceFrame";
import { SpaceHead } from "./SpaceHead";
import { SpaceList } from "./SpaceList";
import { SpaceNewInFolder } from "./SpaceNewInFolder";
import { SpaceOtherHead } from "./SpaceOtherHead";
import { SpacePreview } from "./SpacePreview";
import { SpaceSplit } from "./SpaceSplit";
import { SpaceTabs } from "./SpaceTabs";
import { useKindOrder } from "./useKindOrder";
import { useListWidth } from "./useListWidth";
import { useSpaceSearch } from "./useSpaceSearch";
import { Text } from "../ui/Text";

// A row the cursor can be on: a page, or a document.
type Target = { id: string; to: string };

// SpacePage is a space: its list down the left (your drafts in it first,
// then its documents by kind, the rest by folder) and, beside it, the
// document picked, as the reader shows it. Above the list, a search of
// the space, what's happening in it folded to a line, and a tab for each
// kind that jumps the list there. The page doesn't scroll: the list and
// the preview do, each by itself, and the edge between them is dragged.
// On a narrow window it's the list alone, each document opened to read.
//
// It's what the page knows and does (the space and what's happening in
// it, its search, the document picked, the keys' cursor, the tab the list
// is scrolled to); what it draws are the Space… parts.
export function SpacePage() {
  const workspace = useWorkspace();
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
  });
  const newHere = useShortcut("n", () => setAsKind(true));
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
          <Link to="/documents" onClick={hintOnClick("esc")}>
            Back to Documents
          </Link>
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
  const folderAt = (docs: SpaceDoc[]) => {
    const first = at;
    at += docs.length;
    return first;
  };
  const searched = !!hits;
  return (
    <SpaceFrame>
      <SpaceHead
        label={space.label}
        color={space.color}
        folder={space.folder}
        count={space.count}
        fresh={
          activity && (
            <Fresh
              repo={repo}
              fresh={activity.fresh}
              onMoved={() => {
                reload();
                reloadActivity();
              }}
            />
          )
        }
        actions={
          <Button
            variant="primary"
            onClick={newHere}
            disabled={starting || !space.cloned}
            title={library.length ? `Start a page as one of ${space.label}'s kinds of document: written as its guide says, and filed here` : `Start a page to file in one of ${space.label}'s folders`}
          >
          <Icon name="plus" /> New <Keys k="n" />
        </Button>
      }
    />
    {asKind && library.length > 0 && (
      <NewAsKind kinds={kinds.filter((k) => library.some((x) => x.name === k.name))} busy={starting} onStart={(title, kind) => start(title, kind)} onClose={() => setAsKind(false)} />
    )}
    {asKind && library.length === 0 && (
      <SpaceNewInFolder repo={repo} label={space.label} root={space.folder ?? ""} folders={folders.map(([f]) => f)} busy={starting} onStart={(title, to) => start(title, "", to)} onClose={() => setAsKind(false)} />
    )}
    {value.problem && <Banner tone="attention">It may be behind: {value.problem}</Banner>}
    {!space.cloned ? (
      <Empty icon="box" title="Not cloned here">
        <code>ws setup {space.repo}</code> clones it.
      </Empty>
    ) : (value.docs ?? []).length === 0 ? (
      <Empty icon="file" title="No documents yet">
        It has no markdown{space.folder ? ` under ${space.folder}` : ""} on its default branch.
      </Empty>
    ) : (
      <SpaceSplit listWidth={listWidth}>
        <SpaceList
          scrollRef={list}
          onScroll={spy}
          top={
            <>
              <DocumentsSearchBox
                space
                inputRef={search}
                q={q}
                setQ={setQ}
                placeholder={`Search ${space.label}: what its documents say`}
                label={`Search ${space.label}`}
                onKey={searchKeys}
                found={found.error ? "Couldn't search" : !found.hits ? "Searching…" : `${found.hits.size} of ${space.count}`}
              />
              {activity && <ActivityFold repo={repo} activity={activity} open={foldOpen} setOpen={setFoldOpen} />}
              {activityError && !activity && <DocumentsEmpty>Couldn't read what's happening in it.</DocumentsEmpty>}
              {tabs.length > 1 && (
                <SpaceTabs
                  tabs={tabs.map((t) => ({ id: t.id, label: t.label, count: searched && t.n !== t.total ? `${t.n}/${t.total}` : String(t.total), empty: t.n === 0 }))}
                  value={tabs[tab]?.id ?? ""}
                  onChange={jump}
                  label={library.length ? "Jump to a kind" : "Jump to a folder"}
                />
              )}
            </>
          }
          >
            {drafts.length > 0 && (
              <section aria-label="Your drafts">
                <GroupHead title="Pages of yours that will live here: only you see them until they're saved or filed" count={drafts.length}>
                  Your drafts
                </GroupHead>
                <Rows role="listbox" aria-label="Your drafts">
                  {drafts.map((p, i) => (
                    <PageRow key={p.page.id} page={p.page} kinds={kinds} selected={i === index} onHover={() => {}} note={<DraftNote p={p} />} />
                  ))}
                </Rows>
              </section>
            )}
            {searched && !docsShown.length && <DocumentsEmpty>Nothing in {space.label} says that.</DocumentsEmpty>}
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
            {kindsShown.length > 0 && folders.length > 0 && <SpaceOtherHead id="list-other" />}
            {/* Where Other's tab goes when no kind's list is shown above it. */}
            {library.length > 0 && kindsShown.length === 0 && folders.length > 0 && <span id="list-other" />}
            {folders.map(([folder, docs]) => (
              <section key={folder} id={listID("folder", folder)} aria-label={folder || "Top"}>
                <GroupHead count={docs.length}>
                  <Text mono>{folder || "(top)"}</Text>
                </GroupHead>
                <FolderList repo={repo} label={folder || "Top"} docs={docs} at={folderAt(docs)} index={index} setIndex={wide ? undefined : setIndex} flights={flights} hits={hits} picking={picking} />
              </section>
            ))}
          </SpaceList>
          {wide && (
            <>
              <SpaceEdge width={listWidth} setWidth={setListWidth} />
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
                <SpacePreview none>
                  <Text as="p" dim>{searched ? "Nothing found to show." : "Nothing to show."}</Text>
                </SpacePreview>
              )}
            </>
          )}
        </SpaceSplit>
      )}
    </SpaceFrame>
  );
}
