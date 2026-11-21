import { type ReactNode, useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState, useSyncExternalStore } from "react";
import { Button as AriaButton, Menu, MenuItem, MenuTrigger, Popover } from "react-aria-components";
import { Link, useLocation, useNavigate, useParams, useSearchParams } from "react-router";
import { capsuleAPI, docRef, type FileTree, type FileView, getFile, getTree, pageOpenTo, rawURL, type TreeFile, type Workspace } from "../api";
import { useKeys, useOwnCodeSearch } from "../keys";
import { CodeFinder } from "./CodeFinder";
import { linesAsked, rendersAt } from "./lines";
import { Markdown } from "../markdown/LazyMarkdown";
import { escapeHTML, highlightLines, languageOf, useHighlighter } from "../review/highlight";
import { fuzzy, marked } from "../ui/fuzzy";
import { FileIcon } from "../ui/FileIcon";
import { Icon, type IconName } from "../ui/Icon";
import { Callout, Copyable, Empty, ErrorCard, Keys, Loading, toast, useMedia, useTitle } from "../ui/kit";
import { useLoaded } from "../load";
import { TabbedBrowser } from "./WorktreeTabs";
import { Chip } from "../ui/Chip";
import { Count } from "../ui/Count";
import { Button, buttonClass } from "../ui/Button";
import { GroupHead } from "../ui/GroupHead";

// The file explorer: a capsule's files as they are now, without an editor
// open. A tree on the left (everything, or only what's changed since the
// capsule left its base branch), the file on the right with the lines that
// changed marked, and a fuzzy finder on t (or ⌘P).
//
// FileBrowser is the explorer itself, for any worktree's files: a
// mission's capsule (Explorer, here), or any repo's ground or capsule (the
// Files page, files/Files.tsx).

export const explorerKeys: [string, string][] = [
  ["t / mod+P", "Find a file"],
  ["mod+shift+F", "Search what the capsule's files say"],
  ["j / k", "Next / previous in the tree"],
  ["⏎ / o", "Open a file, or open or close a folder"],
  ["⏎", "On the markdown file that's open: open it as a document, to comment on"],
  ["h / l", "Close / open a folder"],
  ["c", "Only what's changed, or every file"],
  ["[ / ]", "Previous / next changed file"],
  ["m", "Markdown: rendered, or as text"],
  ["v", "Review the capsule's changes"],
  ["y", "Copy the file's path"],
  ["p", "Panes: the tree and the file, or the file alone"],
  ["r", "Reload"],
  ["esc", "Back to the mission"],
];

// A folder or a file in the tree. Folders with one folder in them show as
// one ("internal/web"), as editors do.
interface Node {
  name: string;
  path: string;
  dir: boolean;
  children: Node[];
  file?: TreeFile;
  changed: number; // changed files in it (a folder), for its mark
}

function buildTree(files: TreeFile[]): Node[] {
  const root: Node = { name: "", path: "", dir: true, children: [], changed: 0 };
  const dirs = new Map<string, Node>([["", root]]);
  const dirOf = (path: string): Node => {
    const have = dirs.get(path);
    if (have) return have;
    const cut = path.lastIndexOf("/");
    const parent = dirOf(cut < 0 ? "" : path.slice(0, cut));
    const node: Node = { name: path.slice(cut + 1), path, dir: true, children: [], changed: 0 };
    parent.children.push(node);
    dirs.set(path, node);
    return node;
  };
  for (const f of files) {
    const cut = f.path.lastIndexOf("/");
    const parent = dirOf(cut < 0 ? "" : f.path.slice(0, cut));
    parent.children.push({ name: f.path.slice(cut + 1), path: f.path, dir: false, children: [], file: f, changed: f.status ? 1 : 0 });
  }
  const finish = (n: Node): number => {
    n.children.sort((a, b) => (a.dir !== b.dir ? (a.dir ? -1 : 1) : a.name.localeCompare(b.name, undefined, { numeric: true, sensitivity: "base" })));
    n.changed = n.dir ? n.children.reduce((s, c) => s + finish(c), 0) : n.changed;
    // A folder holding only a folder: one row.
    if (n.dir && n !== root && n.children.length === 1 && n.children[0].dir) {
      const only = n.children[0];
      n.name = `${n.name}/${only.name}`;
      n.path = only.path;
      n.children = only.children;
    }
    return n.changed;
  };
  finish(root);
  return root.children;
}

interface Row {
  node: Node;
  depth: number;
}

function flatten(nodes: Node[], isOpen: (n: Node) => boolean, depth = 0, out: Row[] = []): Row[] {
  for (const n of nodes) {
    out.push({ node: n, depth });
    if (n.dir && isOpen(n)) flatten(n.children, isOpen, depth + 1, out);
  }
  return out;
}

// The folders a path is in: "a", "a/b" for "a/b/c.go".
const ancestors = (path: string) => {
  const parts = path.split("/");
  return parts.slice(0, -1).map((_, i) => parts.slice(0, i + 1).join("/"));
};

const baseName = (p: string) => p.slice(p.lastIndexOf("/") + 1);

const statusIcon: Record<string, IconName> = { added: "plus", deleted: "minus", renamed: "arrow-right", copied: "copy" };
const statusWords: Record<string, string> = { added: "Added", modified: "Modified", deleted: "Deleted", renamed: "Renamed", copied: "Copied", type: "Changed type" };

function StatusMark({ f }: { f: TreeFile }) {
  if (!f.status) return <span className="fmark" aria-hidden />;
  return (
    <span className={`fstatus fmark s-${f.status}`} title={`${statusWords[f.status] ?? f.status} since the branch point${f.uncommitted ? ", not committed" : ""}`}>
      {statusIcon[f.status] ? <Icon name={statusIcon[f.status]} size={10} /> : <span className="fdot" />}
    </span>
  );
}

const images = /\.(png|jpe?g|gif|webp|svg|ico|avif)$/i;
const isMarkdown = (path: string) => /\.(md|markdown|mdx)$/i.test(path);

const sizeWords = (n: number) => (n < 1024 ? `${n} B` : n < 1024 * 1024 ? `${(n / 1024).toFixed(n < 10240 ? 1 : 0)} KB` : `${(n / 1024 / 1024).toFixed(1)} MB`);

// Panes: how many of the explorer's are shown while a file's open. 3 is
// the repos (the Files page's), the tree and the file; 2 drops the repos;
// 1 is the file alone. Yours, in this browser, for every explorer: one
// without the repos shows 2 for 3.
export type Panes = 1 | 2 | 3;
const panesKey = "ws-file-panes";
const panesListeners = new Set<() => void>();
function readPanes(): Panes {
  try {
    const n = Number(localStorage.getItem(panesKey));
    return n === 1 || n === 2 ? n : 3;
  } catch {
    return 3;
  }
}
let panesNow = readPanes();
function setPanes(n: Panes) {
  panesNow = n;
  try {
    if (n === 3) localStorage.removeItem(panesKey);
    else localStorage.setItem(panesKey, String(n));
  } catch {
    // storage off: the choice lasts as long as the page
  }
  panesListeners.forEach((l) => l());
}
function usePanes(): Panes {
  return useSyncExternalStore(
    (l) => {
      panesListeners.add(l);
      return () => panesListeners.delete(l);
    },
    () => panesNow,
  );
}

const paneWords: Record<Panes, { label: string; sub: string }> = {
  3: { label: "3 panes", sub: "Repos, tree and file" },
  2: { label: "2 panes", sub: "Tree and file" },
  1: { label: "1 pane", sub: "The file alone" },
};

// On a tab other than Files, the panes are the repos beside it, or not.
const otherWords: Partial<Record<Panes, { label: string; sub: string }>> = {
  3: { label: "With the repos", sub: "The repos, beside this tab" },
  2: { label: "This tab alone", sub: "The repos folded away" },
};

// PanesMenu picks how many panes show, from the worktree's row: at its
// start, before where it is. On a tab other than Files (other), what it
// picks is whether the repos are beside it.
function PanesMenu({ shown, most, other }: { shown: Panes; most: Panes; other?: boolean }) {
  const words = (n: Panes) => (other ? (otherWords[n] ?? paneWords[n]) : paneWords[n]);
  const choices = ([3, 2, 1] as Panes[]).filter((n) => n <= most && (!other || n > 1));
  const now: Panes = other && shown === 1 ? 2 : shown;
  return (
    <MenuTrigger>
      <AriaButton className="ghost icon-button panes-button" aria-label={`Panes: ${words(now).label}`}>
        <PanesGlyph n={now} />
      </AriaButton>
      <Popover className="select-popover panes-popover" placement="bottom start" offset={4}>
        <Menu className="select-list" aria-label="Panes" onAction={(k) => setPanes(Number(k) as Panes)}>
          {choices.map((n) => (
            <MenuItem key={n} id={String(n)} textValue={words(n).label} className="select-option panes-option">
              <PanesGlyph n={n} />
              <span className="panes-option-words">
                <span>{words(n).label}</span>
                <span className="dim">{words(n).sub}</span>
              </span>
              {n === now && <Icon name="check" size={13} className="panes-check" />}
            </MenuItem>
          ))}
        </Menu>
        <div className="panes-keys dim">
          Switch with <Keys k="p" />
        </div>
      </Popover>
    </MenuTrigger>
  );
}

// PanesGlyph draws n panes as a window split in n, the last (the file)
// widest.
function PanesGlyph({ n }: { n: Panes }) {
  return (
    <svg className="icon panes-glyph" width={15} height={15} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" aria-hidden>
      <rect x={3} y={4} width={18} height={16} rx={2} />
      {n >= 2 && <path d="M9 4v16" />}
      {n === 3 && <path d="M6 4v16" />}
    </svg>
  );
}

// Explorer is a mission's capsule's files.
export function Explorer({ workspace }: { workspace: Workspace }) {
  const { slug = "", repo = "", name = "" } = useParams();
  const navigate = useNavigate();
  const mission = workspace.missions.find((m) => m.slug === slug);
  const api = capsuleAPI(slug, repo, name);
  return (
    <TabbedBrowser
      at={{ repo, worktree: name }}
      key={api}
      api={api}
      label={`${repo}/${name}`}
      updated={`${mission?.updated ?? ""}:${workspace.readsSeen ?? ""}`}
      reviewTo={`/missions/${slug}/review/${repo}/${name}`}
      docOf={(path) => docRef(repo, name, path)}
      search={{ repo, worktree: name }}
      onEscape={() => navigate(`/missions/${slug}?tab=work`)}
      empty="The capsule is empty."
      head={
        <>
          <ExplorerCrumbs slug={slug} title={mission?.title} />
          <div className="explorer-capsule">
            <Icon name="box" size={14} className="dim" />
            <span className="mono">
              {repo}/{name}
            </span>
          </div>
        </>
      }
    />
  );
}

export interface BrowserProps {
  // api is where its files are read (capsuleAPI, worktreeAPI); label names
  // it, for the tab's title ("demo/greet").
  api: string;
  label: string;
  // head is above the tree and the file, beside tabs: where you are, and
  // how to go elsewhere.
  head: ReactNode;
  // tabs are what else there is of the worktree beside its files (its
  // runs, its apps), across the top of the tree and the file. other is
  // what's shown in place of both, when one of those is picked.
  tabs?: ReactNode;
  other?: ReactNode;
  // before is a column before the tree (the Files page's repos).
  before?: ReactNode;
  // The files are read again when updated changes (crew at work).
  updated?: unknown;
  // reviewTo is where its changes are reviewed (v), if they are.
  reviewTo?: string;
  // docOf names one of its files as a document (repo:path, or
  // repo/capsule:path): a markdown file opens as one, to comment on.
  docOf?: (path: string) => string;
  // search is the worktree its own search box searches, whole (a
  // capsule's): it takes ⌘⇧F from Find.
  search?: { repo: string; worktree: string };
  onEscape: () => void;
  // empty is what's said of it with no files.
  empty: string;
}

export function FileBrowser({ api, label, head, tabs, other, before, updated, reviewTo, docOf, search, onEscape, empty }: BrowserProps) {
  const [params, setParams] = useSearchParams();
  const navigate = useNavigate();
  const path = params.get("path") ?? "";
  const changedOnly = params.get("only") === "changed";
  const narrow = useMedia("(max-width: 860px)");
  useTitle(path ? `${baseName(path)} · ${label}` : `Files · ${label}`);
  const most: Panes = before ? 3 : 2;
  const shown = Math.min(usePanes(), most) as Panes;

  // The tree, again whenever what it's in moves (crew at work in it), or
  // the daemon's read it again and found it changed. One that's never been
  // read and is being is still to come: updated moves when it has.
  const [tree, setTree] = useState<FileTree>();
  const [error, setError] = useState<string>();
  const loadTree = useCallback(
    () =>
      getTree(api).then(
        (t) => {
          if (t.unread) return;
          setTree(t);
          setError(undefined);
        },
        (e: Error) => setError(e.message),
      ),
    [api],
  );
  useEffect(() => {
    loadTree();
  }, [loadTree, updated]);

  const ready = useHighlighter();
  const [reload, setReload] = useState(0);

  const files = tree?.files ?? [];
  const changed = useMemo(() => files.filter((f) => f.status), [files]);
  const listed = changedOnly ? changed : files;
  const nodes = useMemo(() => buildTree(listed), [listed]);
  const byPath = useMemo(() => new Map(files.map((f) => [f.path, f])), [files]);

  // Folders open: those you've opened, and every one a file you open is
  // in. Showing only what's changed, all are open: it's a short list.
  const [open, setOpen] = useState<Set<string>>(() => new Set(ancestors(path)));
  useEffect(() => {
    if (path) setOpen((o) => (ancestors(path).every((a) => o.has(a)) ? o : new Set([...o, ...ancestors(path)])));
  }, [path]);
  const isOpen = useCallback((n: Node) => changedOnly || open.has(n.path), [changedOnly, open]);
  const rows = useMemo(() => flatten(nodes, isOpen), [nodes, isOpen]);

  // The tree's cursor: a row, by its path.
  const [cursor, setCursor] = useState(path);
  useEffect(() => {
    if (path) setCursor(path);
  }, [path]);
  const at = Math.max(0, rows.findIndex((r) => r.node.path === cursor));
  const treeRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    treeRef.current?.querySelector(`[data-path="${CSS.escape(cursor)}"]`)?.scrollIntoView({ block: "nearest" });
  }, [cursor, rows.length]);

  const select = (p: string) => {
    if (p === path) return;
    const next = new URLSearchParams(params);
    next.set("path", p);
    // A file outside the changed ones shows everything, so it's in the tree.
    if (changedOnly && !byPath.get(p)?.status) next.delete("only");
    setParams(next);
  };
  const toggle = (n: Node, want?: boolean) =>
    setOpen((o) => {
      const s = new Set(o);
      const opening = want ?? !s.has(n.path);
      if (opening) s.add(n.path);
      else s.delete(n.path);
      return s;
    });
  // A markdown file opens as a page, tied to it: edited with the page's
  // tools, and saved back when you say. ⏎ on the file that's open goes there.
  const docFor = (p: string) => (docOf && isMarkdown(p) && byPath.has(p) ? pageOpenTo(docOf(p)) : undefined);
  const activate = (r?: Row, again = false) => {
    if (!r) return;
    setCursor(r.node.path);
    if (r.node.dir) return toggle(r.node);
    const doc = again && r.node.path === path ? docFor(path) : undefined;
    if (doc) navigate(doc);
    else select(r.node.path);
  };
  const setChangedOnly = (on: boolean) => {
    const next = new URLSearchParams(params);
    if (on) next.set("only", "changed");
    else next.delete("only");
    setParams(next, { replace: true });
  };
  // The changed files in the tree's order, for [ and ].
  const changedOrder = useMemo(() => flatten(buildTree(changed), () => true).filter((r) => !r.node.dir).map((r) => r.node.path), [changed]);
  const stepChanged = (d: number) => {
    if (!changedOrder.length) return;
    const i = changedOrder.indexOf(path);
    select(changedOrder[i < 0 ? (d > 0 ? 0 : changedOrder.length - 1) : (i + d + changedOrder.length) % changedOrder.length]);
  };

  const [finder, setFinder] = useState(false);
  const [searching, setSearching] = useState(false);
  useOwnCodeSearch(search ? () => setSearching(true) : undefined);
  // A file at a line: a hit of the search.
  const openAt = (p: string, line: number) => {
    const next = new URLSearchParams(params);
    next.set("path", p);
    if (changedOnly && !byPath.get(p)?.status) next.delete("only");
    setCursor(p);
    navigate({ search: `?${next}`, hash: `#L${line}` });
  };
  // Markdown shows rendered, unless a line's asked for (a code search hit,
  // Find's :12, a line's link): only its text has lines, so it opens as
  // text there. m still switches.
  const location = useLocation();
  const [preview, setPreview] = useState(() => rendersAt(location.hash));
  useEffect(() => {
    if (!rendersAt(location.hash)) setPreview(false);
  }, [location.hash, path]);
  const closeOrUp = () => {
    const r = rows[at];
    if (!r) return;
    if (r.node.dir && isOpen(r.node) && !changedOnly) return toggle(r.node, false);
    const up = rows.slice(0, at).findLast((x) => x.depth < r.depth);
    if (up) setCursor(up.node.path);
  };
  // Another tab picked: the tree's keys aren't the page's.
  const treeKeys = {
    t: () => setFinder(true),
    "mod+p": () => setFinder(true),
    j: () => setCursor(rows[Math.min(at + 1, rows.length - 1)]?.node.path ?? cursor),
    ArrowDown: () => setCursor(rows[Math.min(at + 1, rows.length - 1)]?.node.path ?? cursor),
    k: () => setCursor(rows[Math.max(at - 1, 0)]?.node.path ?? cursor),
    ArrowUp: () => setCursor(rows[Math.max(at - 1, 0)]?.node.path ?? cursor),
    Enter: () => activate(rows[at], true),
    o: () => activate(rows[at]),
    l: () => rows[at]?.node.dir && toggle(rows[at].node, true),
    ArrowRight: () => rows[at]?.node.dir && toggle(rows[at].node, true),
    h: () => closeOrUp(),
    ArrowLeft: () => closeOrUp(),
    c: () => setChangedOnly(!changedOnly),
    "[": () => stepChanged(-1),
    "]": () => stepChanged(1),
    m: () => setPreview((p) => !p),
    v: () => reviewTo && navigate(reviewTo),
    y: () => path && navigator.clipboard?.writeText(path).then(() => toast(`Copied ${path}`)),
    p: () => {
      if (narrow) return;
      const next = (shown === 1 ? most : shown - 1) as Panes;
      setPanes(next);
      toast(paneWords[next].label, { ms: 1200 });
    },
    r: () => {
      loadTree();
      setReload((n) => n + 1);
    },
    Escape: onEscape,
  };
  // On another tab, p shows or folds away the repos beside it.
  const otherKeys = {
    Escape: onEscape,
    p: () => {
      if (narrow || !before) return;
      const next: Panes = shown === 3 ? 2 : 3;
      setPanes(next);
      toast(otherWords[next]!.label, { ms: 1200 });
    },
  };
  useKeys(other ? otherKeys : treeKeys);

  // A folder asked for (a document's link to one): its README, as GitHub
  // shows a folder, or the folder itself, open in the tree.
  const folder = !!tree && !!path && !byPath.has(path) && files.some((f) => f.path.startsWith(`${path}/`));
  useEffect(() => {
    if (!folder) return;
    const readme = files.find((f) => f.path.startsWith(`${path}/`) && /^(readme|index)\.(md|markdown)$/i.test(f.path.slice(path.length + 1)));
    const next = new URLSearchParams(params);
    if (readme) next.set("path", readme.path);
    else {
      next.delete("path");
      setOpen((o) => new Set([...o, ...ancestors(`${path}/x`)]));
      setCursor(path);
    }
    setParams(next, { replace: true });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [folder, path]);
  // A file asked for that isn't in this tree (one open in another
  // capsule, before switching to this one).
  const missing = !!tree && !!path && !byPath.has(path) && !folder;
  // The panes are picked from the row's start, on any tab where it
  // changes something: another tab has only the repos to show or not.
  const panesMenu = !narrow && (!other || !!before) ? <PanesMenu shown={shown} most={most} other={!!other} /> : undefined;
  const top = (
    <div className="explorer-top">
      {panesMenu}
      <div className="explorer-where">{head}</div>
      {tabs}
    </div>
  );
  if (error && !tree && !other) {
    return (
      <div className={`explorer${narrow ? " narrow" : ""}${before ? " with-before" : ""}`}>
        {before}
        <div className="explorer-main">
          {top}
          <div className="page narrow">
            <ErrorCard title="Couldn't read the files" error={error} retry={loadTree} />
          </div>
        </div>
      </div>
    );
  }
  // With a file open, the panes are as you've picked (narrow, one at a
  // time); with none, the tree's always there, to pick one from.
  const fileOpen = !!path && !folder;
  const alone = !narrow && fileOpen && shown === 1;
  // Another tab has the room of the tree and the file both; the repos
  // beside it are as you've picked the panes.
  const withBefore = !!before && !narrow && (shown === 3 || (!fileOpen && !other));
  const showTree = narrow ? !path : !alone;
  const showFile = !narrow || !!path;
  return (
    <div className={`explorer${narrow ? " narrow" : ""}${withBefore ? " with-before" : ""}`}>
      {withBefore && before}
      <div className="explorer-main">
        {top}
        {other ? (
          <div className="explorer-other">{other}</div>
        ) : (
          <div className={`explorer-body${alone ? " alone" : ""}`}>
            {showTree && (
              <aside className="explorer-tree" aria-label="Files">
                <div className="explorer-tree-head">
                  {tree?.branch && (
                    <div className="explorer-branch dim">
                      <Icon name="branch" size={12} /> {tree.branch}
                      {tree.base && <> · left {tree.baseRef.replace(/^origin\//, "")} at <span className="mono">{tree.base.slice(0, 7)}</span></>}
                    </div>
                  )}
                  <div className="explorer-tools">
                    {tree && !tree.baseRef ? (
                      // Nothing to compare with (a repo's ground): just its files.
                      <span className="dim explorer-count">
                        {files.length} {files.length === 1 ? "file" : "files"}
                      </span>
                    ) : (
                      <div className="segmented" role="group" aria-label="Show">
                        <button aria-pressed={changedOnly} onClick={() => setChangedOnly(true)} title="Only what's changed since the branch point (c)" disabled={!tree?.base}>
                          Changed <span className="count">{changed.length}</span>
                        </button>
                        <button aria-pressed={!changedOnly} onClick={() => setChangedOnly(false)} title="Every file (c)">
                          All <span className="count">{files.length}</span>
                        </button>
                      </div>
                    )}
                    <button className="ghost icon-button small" onClick={() => setFinder(true)} title="Find a file (t)" aria-label="Find a file">
                      <Icon name="search" size={14} />
                    </button>
                    {search && (
                      <button className="ghost icon-button small" onClick={() => setSearching(true)} title="Search what the files say (⌘⇧F)" aria-label="Search what the files say">
                        <Icon name="code" size={14} />
                      </button>
                    )}
                  </div>
                </div>
                <div className="explorer-rows" ref={treeRef} role="tree" aria-label={label}>
                  {!tree && <Loading what="Reading the files…" />}
                  {tree && !rows.length && (
                    <Empty icon={changedOnly ? "check-circle" : "file"} title={changedOnly ? "Nothing's changed yet" : "No files"}>
                      {changedOnly ? `Nothing here differs from ${tree.baseRef.replace(/^origin\//, "")}, committed or not.` : empty}
                    </Empty>
                  )}
                  {rows.map((r) => (
                    <TreeRow
                      key={r.node.path + (r.node.dir ? "/" : "")}
                      row={r}
                      open={r.node.dir && isOpen(r.node)}
                      cursor={r.node.path === cursor}
                      current={r.node.path === path}
                      changedOnly={changedOnly}
                      onClick={() => activate(r)}
                    />
                  ))}
                  {tree?.truncated && <div className="explorer-note">Only the first {files.length} files are listed.</div>}
                </div>
              </aside>
            )}
            {showFile && (
              <section className="explorer-file" aria-label="File">
                {missing ? (
                  <div className="explorer-overview">
                    <Empty icon="file" title="Not here" action={<button onClick={() => setParams(changedOnly ? { only: "changed" } : {})}>Pick another file</button>}>
                      There's no <span className="mono">{path}</span> in {label}.
                    </Empty>
                  </div>
                ) : path && !folder ? (
                  <FilePane
                    api={api}
                    path={path}
                    entry={byPath.get(path)}
                    baseRef={tree?.baseRef ?? ""}
                    updated={`${String(updated)}:${reload}`}
                    preview={preview}
                    setPreview={setPreview}
                    ready={ready}
                    reviewTo={reviewTo}
                    docTo={docFor(path)}
                    onBack={narrow ? () => setParams(changedOnly ? { only: "changed" } : {}) : undefined}
                    onReveal={(p) => {
                      setOpen((o) => new Set([...o, ...ancestors(p + "/x")]));
                      setCursor(p);
                    }}
                  />
                ) : (
                  <Overview tree={tree} changed={changed} onOpen={select} onFind={() => setFinder(true)} />
                )}
              </section>
            )}
          </div>
        )}
      </div>
      {searching && search && (
        <CodeFinder
          repo={search.repo}
          worktree={search.worktree}
          onPick={(p, line) => {
            setSearching(false);
            openAt(p, line);
          }}
          onClose={() => setSearching(false)}
        />
      )}
      {finder && tree && (
        <FileFinder
          files={files}
          onPick={(p) => {
            setFinder(false);
            select(p);
          }}
          onClose={() => setFinder(false)}
        />
      )}
    </div>
  );
}

function ExplorerCrumbs({ slug, title }: { slug: string; title?: string }) {
  return (
    <nav className="crumbs" aria-label="Breadcrumb">
      <Link to={`/missions/${slug}?tab=work`}>
        <Icon name="arrow-left" size={13} /> {title ?? slug}
      </Link>
    </nav>
  );
}

function TreeRow({ row, open, cursor, current, changedOnly, onClick }: { row: Row; open: boolean; cursor: boolean; current: boolean; changedOnly: boolean; onClick: () => void }) {
  const n = row.node;
  return (
    <div
      role="treeitem"
      aria-expanded={n.dir ? open : undefined}
      aria-selected={current}
      data-path={n.path}
      className={`tree-row${n.dir ? " dir" : ""}${cursor ? " cursor" : ""}${current ? " current" : ""}${n.file?.status === "deleted" ? " deleted" : ""}`}
      style={{ paddingLeft: 8 + row.depth * 14 }}
      onClick={onClick}
      title={n.path}
    >
      {n.dir ? (
        <>
          <Icon name={open ? "chevron-down" : "chevron-right"} size={12} className="tree-chevron" />
          <span className="tree-name">{n.name}</span>
          {!changedOnly && n.changed > 0 && <span className="tree-changed" title={`${n.changed} changed`} />}
        </>
      ) : (
        <>
          <StatusMark f={n.file!} />
          <FileIcon path={n.path} />
          <span className="tree-name">{n.name}</span>
          {n.file?.uncommitted && <span className="dirty-dot" title="Not committed" />}
        </>
      )}
    </div>
  );
}

// Overview is the capsule before a file's open: what's changed, a click
// each, and how to find the rest.
function Overview({ tree, changed, onOpen, onFind }: { tree?: FileTree; changed: TreeFile[]; onOpen: (p: string) => void; onFind: () => void }) {
  if (!tree) return <Loading what="Reading the files…" />;
  return (
    <div className="explorer-overview">
      <Empty
        icon="file"
        title="Pick a file"
        action={
          <Button onClick={onFind}>
            <Icon name="search" /> Find a file <Keys k="t" />
          </Button>
        }
      >
        {tree.files.length} files{tree.base ? `; ${changed.length} changed since the capsule left ${tree.baseRef.replace(/^origin\//, "")}` : ""}.
      </Empty>
      {changed.length > 0 && (
        <section>
          <GroupHead count={changed.length}>
            Changed
          </GroupHead>
          <ul className="overview-changed">
            {changed.slice(0, 40).map((f) => (
              <li key={f.path}>
                <Button variant="link" className="overview-file" onClick={() => onOpen(f.path)}>
                  <StatusMark f={f} />
                  <FileIcon path={f.path} />
                  <span className="mono">{f.path}</span>
                  {f.uncommitted && <Chip tone="attention">Not committed</Chip>}
                </Button>
              </li>
            ))}
          </ul>
        </section>
      )}
    </div>
  );
}

// FilePane is the open file: where it is, how it's changed, and its text
// with the changed lines marked (or rendered, for markdown; shown, for an
// image).
function FilePane({
  api,
  path,
  entry,
  baseRef,
  updated,
  preview,
  setPreview,
  ready,
  reviewTo,
  docTo,
  onBack,
  onReveal,
}: {
  api: string;
  path: string;
  entry?: TreeFile;
  baseRef: string;
  updated: string;
  preview: boolean;
  setPreview: (fn: (p: boolean) => boolean) => void;
  ready: boolean;
  reviewTo?: string;
  docTo?: string; // where it opens as a document, for markdown
  onBack?: () => void;
  onReveal: (folder: string) => void;
}) {
  // What's read is shown only while it's the file that's open: its text
  // mustn't be drawn as another's (its images read from the wrong folder)
  // for the moment between picking one and reading it.
  const [tries, setTries] = useState(0);
  const { value: read, error } = useLoaded<FileView>(() => getFile(api, path), [api, path, updated, tries], { keep: true });
  const file = read?.path === path ? read : undefined;

  // Where the file is, for the reader: a link in it to another file opens
  // that one here, in the same tree.
  const location = useLocation();
  const place = useMemo(
    () => ({
      api,
      path,
      to: (p: string) => {
        const next = new URLSearchParams(location.search);
        next.set("path", p);
        next.delete("only");
        return `${location.pathname}?${next}`;
      },
    }),
    [api, path, location.pathname, location.search],
  );
  const parts = path.split("/");
  const md = isMarkdown(path);
  const image = images.test(path);
  const base = baseRef.replace(/^origin\//, "");
  return (
    <div className="file-pane">
      <header className="file-pane-head">
        {onBack && (
          <Button variant="ghost" square onClick={onBack} aria-label="Back to the files">
            <Icon name="arrow-left" size={15} />
          </Button>
        )}
        <div className="file-crumbs" title={path}>
          {parts.map((p, i) =>
            i < parts.length - 1 ? (
              <span key={i}>
                <Button variant="link" className="crumb" onClick={() => onReveal(parts.slice(0, i + 1).join("/"))}>
                  {p}
                </Button>
                <span className="sep">/</span>
              </span>
            ) : (
              <strong key={i} className="file-crumb-name">
                <FileIcon path={path} />
                {p}
              </strong>
            ),
          )}
          <Copyable text={path} className="file-copy" title="Copy its path (y)">
            {""}
          </Copyable>
        </div>
        <div className="file-pane-tools">
          {md && file && !file.binary && !file.tooLarge && (
            <div className="segmented" role="group" aria-label="Markdown">
              <Button aria-pressed={preview} onClick={() => setPreview(() => true)} title="Rendered (m)">
                Rendered
              </Button>
              <Button aria-pressed={!preview} onClick={() => setPreview(() => false)} title="As text (m)">
                Text
              </Button>
            </div>
          )}
          {docTo && file && !file.binary && !file.tooLarge && !file.deleted && (
            <Link to={docTo} className={buttonClass({ size: "small" })} title="Edit it as a page: cells, questions and passes, and saved back to the repo when you say (⏎)">
              <Icon name="pencil" size={12} /> Edit as a page <Keys k="⏎" />
            </Link>
          )}
          {reviewTo && entry?.status && entry.status !== "added" && (
            <Link to={reviewTo} className={buttonClass({ size: "small" })} title="Review the capsule's changes (v)">
              <Icon name="eye" size={12} /> Changes
            </Link>
          )}
        </div>
      </header>
      <div className="file-meta">
        {entry?.status ? (
          <Chip tone={entry.status === "deleted" ? "danger" : entry.status === "added" ? "done" : "attention"}>
            {statusWords[entry.status]} since {base}
          </Chip>
        ) : (
          base && <Chip tone="quiet">Unchanged since {base}</Chip>
        )}
        {entry?.oldPath && <span className="dim">from {entry.oldPath}</span>}
        {entry?.uncommitted && <Chip tone="attention">Not committed</Chip>}
        {file && image && !file.deleted && <span className="dim num">{sizeWords(file.size)}</span>}
        {file && !image && !file.binary && !file.tooLarge && (
          <span className="dim num">
            {file.lines.length} {file.lines.length === 1 ? "line" : "lines"}
            {file.size > 0 && ` · ${sizeWords(file.size)}`}
          </span>
        )}
        {file && !image && file.changes.length > 0 && !(md && preview) && (
          <span className="change-legend" title="Marked beside the line numbers">
            {file.changes.some((c) => c.kind === "added") && (
              <span>
                <i className="m-added" /> added
              </span>
            )}
            {file.changes.some((c) => c.kind === "modified") && (
              <span>
                <i className="m-modified" /> changed
              </span>
            )}
            {file.changes.some((c) => c.kind === "deleted") && (
              <span>
                <i className="m-deleted" /> removed after
              </span>
            )}
          </span>
        )}
      </div>
      {error && <ErrorCard title="Couldn't open this file" error={error} retry={() => setTries((n) => n + 1)} />}
      {!file && !error && <Loading what="Opening it…" />}
      {file && file.deleted && (
        <Callout tone="danger" icon="minus" title={`Deleted since it left ${base}`}>
          This is how it was there.
        </Callout>
      )}
      {file &&
        (image && !file.deleted ? (
          <div className="file-image">
            <img src={rawURL(api, path)} alt={path} data-viewable={path} title="Open it larger" />
          </div>
        ) : file.binary ? (
          <Empty icon="file" title="A binary file">
            {sizeWords(file.size)}: there's no text to show.
          </Empty>
        ) : file.tooLarge ? (
          <Empty icon="file" title="Too large to show">
            It's {sizeWords(file.size)}. Open it in an editor: <Copyable text={path} className="mono" />
          </Empty>
        ) : !file.lines.length ? (
          <Empty icon="file" title="An empty file" />
        ) : md && preview ? (
          <article className="artifact-body prose file-markdown">
            <Markdown text={file.lines.join("\n")} at={place} />
          </article>
        ) : (
          <Code file={file} path={path} ready={ready} />
        ))}
    </div>
  );
}

// Code is a file's lines, highlighted, numbered, the changed ones marked
// in the gutter. A line number links to the line (#L12; shift-click for a
// range).
function Code({ file, path, ready }: { file: FileView; path: string; ready: boolean }) {
  const location = useLocation();
  const navigate = useNavigate();
  const lang = languageOf(path);
  // Plain until the highlighter's loaded (ready), then drawn again.
  const html = useMemo(() => (ready && file.lines.length <= 20000 ? highlightLines(file.lines, lang) : file.lines.map(escapeHTML)), [file, lang, ready]);
  const marks = useMemo(() => {
    const m = new Map<number, string>();
    for (const c of file.changes) {
      if (c.kind === "deleted") {
        m.set(c.start, m.get(c.start) ? m.get(c.start)! : "deleted-after");
        continue;
      }
      for (let n = c.start; n <= (c.end ?? c.start); n++) m.set(n, c.kind);
    }
    return m;
  }, [file]);
  const sel = linesAsked(location.hash);
  const from = sel?.from ?? 0;
  const to = sel?.to ?? 0;
  const body = useRef<HTMLDivElement>(null);
  // To the line asked for (a link to it), not each time you click one.
  const clicked = (location.state as { clicked?: boolean } | null)?.clicked;
  useLayoutEffect(() => {
    if (from && !clicked) body.current?.querySelector(`[data-line="${from}"]`)?.scrollIntoView({ block: "center" });
  }, [file.path, from, clicked]);
  const pick = (n: number, e: React.MouseEvent) => {
    const hash = e.shiftKey && from ? `#L${Math.min(from, n)}-L${Math.max(from, n)}` : `#L${n}`;
    navigate({ search: location.search, hash }, { replace: true, state: { clicked: true } });
  };
  return (
    <div className="code-view" ref={body}>
      {html.map((h, i) => {
        const n = i + 1;
        const mark = marks.get(n);
        return (
          <div key={n} data-line={n} className={`code-line${mark ? ` m-${mark}` : ""}${n >= from && n <= to ? " picked" : ""}`}>
            <button className="code-ln" onClick={(e) => pick(n, e)} tabIndex={-1} aria-label={`Line ${n}`}>
              {n}
            </button>
            <span className="code-mark" aria-hidden />
            <code dangerouslySetInnerHTML={{ __html: h || " " }} />
          </div>
        );
      })}
    </div>
  );
}

// FileFinder finds a file by a few of its letters: t or ⌘P.
function FileFinder({ files, onPick, onClose }: { files: TreeFile[]; onPick: (path: string) => void; onClose: () => void }) {
  const [q, setQ] = useState("");
  const [sel, setSel] = useState(0);
  const list = useRef<HTMLUListElement>(null);
  useKeys({ Escape: onClose }, { modal: true });
  const matches = useMemo(() => {
    if (!q.trim()) {
      // Before you type: what's changed first, then the rest.
      return [...files]
        .sort((a, b) => Number(!!b.status) - Number(!!a.status))
        .slice(0, 60)
        .map((f) => ({ f, at: [] as number[] }));
    }
    const out: { f: TreeFile; at: number[]; score: number }[] = [];
    for (const f of files) {
      const m = fuzzy(f.path, q);
      if (m) out.push({ f, at: m.at, score: m.score + (f.status ? 1.5 : 0) });
    }
    return out.sort((a, b) => b.score - a.score).slice(0, 60);
  }, [files, q]);
  const at = Math.min(sel, Math.max(0, matches.length - 1));
  const move = (d: number) => {
    const n = Math.max(0, Math.min(at + d, matches.length - 1));
    setSel(n);
    list.current?.children[n]?.scrollIntoView({ block: "nearest" });
  };
  return (
    <div className="overlay palette-overlay" onMouseDown={onClose}>
      <div className="palette" role="dialog" aria-modal="true" aria-label="Find a file" onMouseDown={(e) => e.stopPropagation()}>
        <div className="palette-input">
          <Icon name="search" size={16} />
          <input
            autoFocus
            value={q}
            placeholder="Find a file by a few of its letters…"
            aria-label="Find a file"
            onChange={(e) => {
              setQ(e.target.value);
              setSel(0);
            }}
            onKeyDown={(e) => {
              if (e.key === "ArrowDown" || (e.ctrlKey && e.key === "n")) {
                e.preventDefault();
                move(1);
              } else if (e.key === "ArrowUp" || (e.ctrlKey && e.key === "p")) {
                e.preventDefault();
                move(-1);
              } else if (e.key === "Enter" && matches[at]) {
                e.preventDefault();
                onPick(matches[at].f.path);
              }
            }}
          />
        </div>
        <ul className="palette-list file-finder" ref={list} role="listbox">
          {matches.map((m, i) => {
            const cut = m.f.path.lastIndexOf("/") + 1;
            const runs = marked(m.f.path, m.at);
            let pos = 0;
            return (
              <li key={m.f.path} role="option" aria-selected={i === at} className={i === at ? "on" : ""} onMouseMove={() => i !== at && setSel(i)} onClick={() => onPick(m.f.path)}>
                <StatusMark f={m.f} />
                <FileIcon path={m.f.path} />
                <span className="finder-path">
                  {runs.map((r, j) => {
                    const start = pos;
                    pos += r.text.length;
                    const inName = start >= cut;
                    return (
                      <span key={j} className={`${r.hit ? "hit" : ""}${inName ? " name" : ""}`}>
                        {r.text}
                      </span>
                    );
                  })}
                </span>
                {m.f.uncommitted && <span className="dirty-dot" title="Not committed" />}
              </li>
            );
          })}
          {!matches.length && <li className="palette-none">No file matches “{q}”.</li>}
        </ul>
      </div>
    </div>
  );
}
