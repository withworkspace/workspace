import { useEffect, useMemo, useRef, useState } from "react";
import { Link, useLocation, useNavigate, useParams } from "react-router";
import {
  docTo,
  getSpace,
  getSpaceActivity,
  getSpaces,
  pageAddress,
  type SpaceDoc,
  type SpaceKind,
  type Workspace,
} from "../api";
import { Fresh, InFlight, inFlightPaths, Lately } from "./Activity";
import {
  InFlightChip,
  KindHead,
  KindList,
  libraryID,
  sorted,
  useKindOrder,
} from "./Library";
import { useKeys } from "../keys";
import { useLoaded } from "../load";
import { NewAsKind, PageRow, useStartPage } from "../pages/Pages";
import { Icon } from "../ui/Icon";
import { Dialog, Empty, ErrorCard, Keys, Loading, useTitle } from "../ui/kit";
import { TLink } from "../ui/TLink";
import { type Placed, placePages } from "./place";
import { hitTo, Results, SearchBox, useDocSearch } from "./Search";

// Documents: where the workspace's writing is read. Local is the pages
// that aren't in a space (written here, on this machine); each space is a
// repo of documents, or a folder of one, read from its default branch. A
// page that will live in a space is listed there, as your draft.

export const documentsKeys: [string, string][] = [
  ["n", "Start a page"],
  ["N", "Start a page as one of the workspace's kinds of document"],
  ["j / k", "Move between pages and spaces"],
  ["⏎ or o", "Open it"],
];

export const spaceKeys: [string, string][] = [
  ["j / k", "Move between documents"],
  ["⏎ or o", "Read it"],
  ["esc", "Back to Documents"],
];

// A row the cursor can be on: a page, or a space.
type Target = { id: string; to: string };

export function DocumentsPage({ workspace }: { workspace: Workspace }) {
  useTitle("Documents");
  const navigate = useNavigate();
  const { value, error, reload } = useLoaded(getSpaces, [], { keep: true });
  const spaces = useMemo(() => value?.spaces ?? [], [value]);
  const kinds = useMemo(() => workspace.kinds ?? [], [workspace.kinds]);
  const placed = useMemo(
    () => placePages(workspace.pages ?? [], spaces, kinds),
    [workspace.pages, spaces, kinds],
  );
  const local = placed.filter((p) => !p.repo);
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

  const targets: Target[] = [
    ...local.map((p) => ({ id: `page-${p.page.id}`, to: pageAddress(p.page) })),
    ...spaces.map((s) => ({
      id: `space-${s.repo}`,
      to: `/documents/${encodeURIComponent(s.repo)}`,
    })),
  ];
  const move = (i: number) => {
    const n = Math.min(Math.max(i, 0), targets.length - 1);
    setIndex(n);
    document
      .getElementById(targets[n]?.id ?? "")
      ?.scrollIntoView({ block: "nearest" });
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
    <div className="page narrow documents-page">
      <header className="page-head">
        <h1>Documents</h1>
        <div className="page-actions">
          {kinds.length > 0 && (
            <button
              onClick={() => setAsKind(true)}
              disabled={starting}
              title="Start a page as one of the workspace's kinds of document: it's written as its guide says, and filed where the kind says"
            >
              New as… <Keys k="N" />
            </button>
          )}
          <button
            className="primary"
            onClick={() => start()}
            disabled={starting}
          >
            <Icon name="plus" /> New page <Keys k="n" />
          </button>
        </div>
      </header>
      {asKind && (
        <NewAsKind
          kinds={kinds}
          busy={starting}
          onStart={(title, kind) => start(title, kind)}
          onClose={() => setAsKind(false)}
        />
      )}
      {spaces.length > 0 && (
        <SearchBox
          q={q}
          setQ={setQ}
          inputRef={search}
          spaces={spaces}
          kind={kind}
          setKind={setKind}
          state={stand}
          setState={setStand}
          onKey={searchKeys}
        />
      )}
      {searching ? (
        <Results
          hits={found.hits}
          more={found.more}
          error={found.error}
          index={hit}
          setIndex={setHit}
        />
      ) : (
        <>
          <section aria-label="Local">
            <h2
              className="group"
              title="Pages written here, on this machine, that aren't in a space: no one else sees them. File as… is the way out."
            >
              Local <span className="count">{local.length}</span>
              {filling > 0 && (
                <span className="dim documents-sub">· {filling} filling</span>
              )}
            </h2>
            {local.length === 0 ? (
              <p className="dim documents-empty">
                No pages here. A page is somewhere to write a problem down and
                work it out:{" "}
                <button className="link" onClick={() => start()}>
                  start one
                </button>
                .
              </p>
            ) : (
              <ul
                className="rows pages"
                role="listbox"
                aria-label="Local pages"
              >
                {local.map((p, i) => (
                  <PageRow
                    key={p.page.id}
                    page={p.page}
                    kinds={kinds}
                    selected={i === index}
                    onHover={() => i !== index && setIndex(i)}
                    note={
                      p.page.source && (
                        <span
                          className="chip tone-quiet"
                          title="Its file isn't in a space of documents"
                        >
                          {p.page.source.ref.split(":")[0]}
                        </span>
                      )
                    }
                  />
                ))}
              </ul>
            )}
          </section>

          <section aria-label="Spaces">
            {error && !value && (
              <ErrorCard
                title="Couldn't read the spaces"
                error={error}
                retry={reload}
              />
            )}
            {value?.problem && (
              <div className="banner tone-attention">
                Some of it may be behind: {value.problem}
              </div>
            )}
            {!value && !error && <Loading what="Reading the spaces…" />}
            {value && spaces.length === 0 && (
              <Empty icon="file" title="No spaces of documents">
                A repo of documents (ADRs, proposals, guides) is read here, by
                its titles, rather than in Files: say{" "}
                <code>documents = true</code> under its{" "}
                <code>[repos.&lt;name&gt;]</code> in <code>ws.toml</code>, or
                name it as <code>[missions] records</code>.
              </Empty>
            )}
            {spaces.map((s, i) => {
              const at = local.length + i;
              const drafts = placed.filter((p) => p.repo === s.repo);
              return (
                <div key={s.repo} className="space-block">
                  <h2 className="group">
                    <span
                      className="repo-dot"
                      style={{ background: s.color }}
                    />{" "}
                    {s.label}
                    {s.folder && <span className="mono">{s.folder}</span>}
                    {s.label !== s.repo && (
                      <span className="dim mono">{s.repo}</span>
                    )}
                  </h2>
                  <ul className="rows" role="listbox" aria-label={s.label}>
                    <li
                      id={`space-${s.repo}`}
                      role="option"
                      aria-selected={at === index}
                      className={`row${at === index ? " selected" : ""}`}
                      onMouseMove={() => at !== index && setIndex(at)}
                    >
                      <TLink
                        to={`/documents/${encodeURIComponent(s.repo)}`}
                        className="row-link"
                      >
                        <Icon name="layers" size={16} className="row-status" />
                        <span className="row-main">
                          <span className="row-line">
                            <span className="title">
                              {s.cloned
                                ? `${s.count} ${s.count === 1 ? "document" : "documents"}`
                                : "Not cloned"}
                            </span>
                            {drafts.length > 0 && (
                              <span className="chip tone-attention">
                                {drafts.length === 1
                                  ? "1 draft of yours"
                                  : `${drafts.length} drafts of yours`}
                              </span>
                            )}
                          </span>
                          {s.folders.length > 0 && (
                            <span className="row-line dim space-folders">
                              {s.folders.map((f, j) => (
                                <span key={f.name}>
                                  {j > 0 && " · "}
                                  {f.name}{" "}
                                  <span className="count">{f.count}</span>
                                </span>
                              ))}
                            </span>
                          )}
                        </span>
                        <Icon name="chevron-right" size={14} className="dim" />
                      </TLink>
                    </li>
                  </ul>
                </div>
              );
            })}
          </section>
        </>
      )}
    </div>
  );
}

// SpacePage is a space: your drafts in it first, then its documents by
// folder, each by its title (in the order of their paths), read in the
// reader.
export function SpacePage({ workspace }: { workspace: Workspace }) {
  const { repo = "" } = useParams();
  const navigate = useNavigate();
  const { value, error, reload } = useLoaded(() => getSpace(repo), [repo], {
    keep: true,
  });
  const {
    value: activity,
    error: activityError,
    reload: reloadActivity,
  } = useLoaded(
    () => getSpaceActivity(repo),
    [repo],
    { keep: true },
  );
  const flights = useMemo(() => inFlightPaths(activity), [activity]);
  const space = value?.space;
  useTitle(space ? space.label : repo);
  const kinds = useMemo(() => workspace.kinds ?? [], [workspace.kinds]);
  const drafts = useMemo(
    () =>
      space
        ? placePages(workspace.pages ?? [], [space], kinds).filter(
            (p) => p.repo === space.repo,
          )
        : [],
    [workspace.pages, space, kinds],
  );
  // Its library: a list for each kind whose path is in it, and the rest by
  // folder, under Other.
  const library = useMemo(() => space?.kinds ?? [], [space]);
  const order0 = useKindOrder();
  const byKind = useMemo(() => {
    const m = new Map<string, SpaceDoc[]>();
    for (const d of value?.docs ?? [])
      if (d.kind && library.some((k) => k.name === d.kind))
        m.set(d.kind, [...(m.get(d.kind) ?? []), d]);
    return m;
  }, [value, library]);
  const shown = (k: SpaceKind) =>
    sorted(
      (byKind.get(k.name) ?? []).filter(
        (d) => !order0.standOf(k.name) || d.state === order0.standOf(k.name),
      ),
      order0.sortOf(k.name),
      k.states,
    );
  const folders = useMemo(
    () =>
      byFolder(
        (value?.docs ?? []).filter(
          (d) => !(d.kind && library.some((k) => k.name === d.kind)),
        ),
        space?.folder ?? "",
      ),
    [value, space, library],
  );
  const order: Target[] = [
    ...drafts.map((p) => ({
      id: `page-${p.page.id}`,
      to: pageAddress(p.page),
    })),
    ...library.flatMap((k) =>
      shown(k).map((d) => ({
        id: libraryID(d.path),
        to: docTo(`${repo}:${d.path}`),
      })),
    ),
    ...folders.flatMap(([, docs]) =>
      docs.map((d) => ({ id: docID(d.path), to: docTo(`${repo}:${d.path}`) })),
    ),
  ];
  // New starts a page as one of its kinds.
  const ownKinds = kinds.filter((k) => library.some((x) => x.name === k.name));
  const { start, starting } = useStartPage();
  const [asKind, setAsKind] = useState(false);
  const [index, setIndex] = useState(0);
  const move = (i: number) => {
    const n = Math.min(Math.max(i, 0), order.length - 1);
    setIndex(n);
    document
      .getElementById(order[n]?.id ?? "")
      ?.scrollIntoView({ block: "nearest" });
  };
  const open = () => order[index] && navigate(order[index].to);
  useKeys({
    j: () => move(index + 1),
    ArrowDown: () => move(index + 1),
    k: () => move(index - 1),
    ArrowUp: () => move(index - 1),
    Enter: open,
    o: open,
    Escape: () => navigate("/documents"),
    n: () => setAsKind(true),
  });
  // Opened at one of its lists (spaceTo: the reader's crumbs, "Open in the
  // space"): there, its first document chosen. A folder with no documents
  // of its own is the first folder below it that has some. Once what's
  // above its lists is there (what changed lately), so it stays in sight.
  const { hash } = useLocation();
  const wentTo = useRef("");
  useEffect(() => {
    if (!value || !space || !hash || wentTo.current === hash) return;
    if (!activity && !activityError) return;
    wentTo.current = hash;
    const want = new URLSearchParams(hash.slice(1));
    const kind = library.find((k) => k.name === want.get("kind"));
    const folder = want.get("folder")?.slice((space.folder ?? "").length);
    let list: { id: string; first?: SpaceDoc } | undefined;
    if (kind) list = { id: listID("kind", kind.name), first: shown(kind)[0] };
    else if (folder !== undefined) {
      const f =
        folders.find(([x]) => x === folder) ??
        folders.find(([x]) => x.startsWith(folder));
      if (f) list = { id: listID("folder", f[0]), first: f[1][0] };
    }
    if (!list) return;
    const first = list.first;
    const i = first
      ? order.findIndex(
          (o) => o.id === libraryID(first.path) || o.id === docID(first.path),
        )
      : -1;
    if (i >= 0) setIndex(i);
    document.getElementById(list.id)?.scrollIntoView({ block: "start" });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value, activity, activityError, hash]);

  if (error && !value) {
    return (
      <div className="page narrow">
        <ErrorCard
          title={`Couldn't read ${repo}`}
          error={error}
          retry={reload}
        />
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
  return (
    <div className="page narrow documents-page space-page">
      <nav className="crumbs" aria-label="Breadcrumb">
        <Link to="/documents">
          <Icon name="arrow-left" size={13} /> Documents
        </Link>
      </nav>
      <header className="page-head">
        <h1>
          <span className="repo-dot" style={{ background: space.color }} />{" "}
          {space.label}
          {space.folder && <span className="mono dim"> {space.folder}</span>}
        </h1>
        <span className="dim">
          {space.count} {space.count === 1 ? "document" : "documents"}, as on
          its default branch
        </span>
        <div className="page-actions">
          <button
            className="primary"
            onClick={() => setAsKind(true)}
            disabled={starting || !space.cloned}
            title={
              ownKinds.length
                ? `Start a page as one of ${space.label}'s kinds of document: written as its guide says, and filed here`
                : `Start a page to file in one of ${space.label}'s folders`
            }
          >
            <Icon name="plus" /> New <Keys k="n" />
          </button>
        </div>
      </header>
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
      {asKind && ownKinds.length > 0 && (
        <NewAsKind
          kinds={ownKinds}
          busy={starting}
          onStart={(title, kind) => start(title, kind)}
          onClose={() => setAsKind(false)}
        />
      )}
      {asKind && ownKinds.length === 0 && (
        <NewInFolder
          repo={repo}
          label={space.label}
          root={space.folder ?? ""}
          folders={folders.map(([f]) => f)}
          busy={starting}
          onStart={(title, to) => start(title, "", to)}
          onClose={() => setAsKind(false)}
        />
      )}
      {value.problem && (
        <div className="banner tone-attention">
          It may be behind: {value.problem}
        </div>
      )}
      {drafts.length > 0 && (
        <section aria-label="Your drafts">
          <h2
            className="group"
            title="Pages of yours that will live here: only you see them until they're saved or filed"
          >
            Your drafts <span className="count">{drafts.length}</span>
          </h2>
          <ul className="rows pages" role="listbox" aria-label="Your drafts">
            {drafts.map((p, i) => (
              <PageRow
                key={p.page.id}
                page={p.page}
                kinds={kinds}
                selected={i === index}
                onHover={() => i !== index && setIndex(i)}
                note={<DraftNote p={p} />}
              />
            ))}
          </ul>
        </section>
      )}
      {activity && <InFlight repo={repo} activity={activity} />}
      {activity && <Lately repo={repo} activity={activity} />}
      {!space.cloned && (
        <Empty icon="box" title="Not cloned here">
          <code>ws setup {space.repo}</code> clones it.
        </Empty>
      )}
      {space.cloned && folders.length === 0 && (
        <Empty icon="file" title="No documents yet">
          It has no markdown{space.folder ? ` under ${space.folder}` : ""} on
          its default branch.
        </Empty>
      )}
      {library.map((k) => (
        <section
          key={k.name}
          id={listID("kind", k.name)}
          aria-label={k.title}
        >
          <KindHead
            kind={k}
            docs={byKind.get(k.name) ?? []}
            sort={order0.sortOf(k.name)}
            setSort={(s) => order0.setSort(k.name, s)}
            stand={order0.standOf(k.name)}
            setStand={(s) => order0.setStand(k.name, s)}
          />
          <KindList
            repo={repo}
            kind={k}
            docs={shown(k)}
            at={kindAt(k)}
            index={index}
            setIndex={setIndex}
            flights={flights}
          />
        </section>
      ))}
      {library.length > 0 && folders.length > 0 && (
        <h2 className="other-head">Other</h2>
      )}
      {folders.map(([folder, docs]) => (
        <section
          key={folder}
          id={listID("folder", folder)}
          aria-label={folder || "Top"}
        >
          <h2 className="group">
            <span className="mono">{folder || "(top)"}</span>{" "}
            <span className="count">{docs.length}</span>
          </h2>
          <ul
            className="rows space-docs"
            role="listbox"
            aria-label={folder || "Top"}
          >
            {docs.map((d) => {
              const i = at++;
              return (
                <li
                  key={d.path}
                  id={docID(d.path)}
                  role="option"
                  aria-selected={i === index}
                  className={`row${i === index ? " selected" : ""}`}
                  onMouseMove={() => i !== index && setIndex(i)}
                >
                  <TLink to={docTo(`${repo}:${d.path}`)} className="row-link">
                    <Icon name="file" size={16} className="row-status" />
                    <span className="row-main">
                      <span className="row-line">
                        <span className="title">{d.title}</span>
                        {flights.has(d.path) && (
                          <InFlightChip capsules={flights.get(d.path)!} />
                        )}
                      </span>
                    </span>
                    <span className="dim mono doc-file">
                      {d.path.split("/").pop()}
                    </span>
                  </TLink>
                </li>
              );
            })}
          </ul>
        </section>
      ))}
    </div>
  );
}

function DraftNote({ p }: { p: Placed }) {
  switch (p.why) {
    case "to":
      return (
        <span
          className="chip tone-quiet"
          title={`To be filed in ${p.path || "its top"}, not filed yet`}
        >
          Not filed · {p.path || "top"}
        </span>
      );
    case "unsaved":
      return (
        <span
          className="chip tone-attention"
          title={`Your edits to ${p.path}, not saved to the repo yet`}
        >
          Unsaved
        </span>
      );
    case "filed":
      return (
        <span
          className="chip tone-progress"
          title={`Filed as ${p.path}, in a capsule: it's here once it lands`}
        >
          Filed, not landed
        </span>
      );
    default:
      return (
        <span
          className="chip tone-quiet"
          title="Started as one of this space's kinds, not filed yet"
        >
          Not filed
        </span>
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
        <button className="ghost" onClick={onClose}>
          Cancel
        </button>
        <button
          className="primary"
          onClick={go}
          disabled={busy || !title.trim()}
        >
          Start it <Keys k="⏎" />
        </button>
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
