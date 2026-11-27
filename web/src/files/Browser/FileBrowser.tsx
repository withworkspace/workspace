import { type ReactNode, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useLocation, useNavigate, useSearchParams } from "react-router";
import { type FileTree, getTree, pageOpenTo, type TreeFile } from "../../api";
import { useOwnCodeSearch } from "../../keys";
import type { Command } from "../../keymap/commands";
import { useCommands } from "../../keymap/useCommands";
import { filesKeys } from "../keys";
import { useHighlighter } from "../../review/highlight";
import { Button } from "../../ui/Button";
import { Empty } from "../../ui/Empty";
import { ErrorCard } from "../../ui/ErrorCard";
import { Loading } from "../../ui/Loading";
import { toast } from "../../ui/toast";
import { Tree, type TreeControl } from "../../ui/Tree";
import { foldersOf, leavesOf, type TreeNode } from "../../ui/treeNodes";
import { useMedia } from "../../ui/useMedia";
import { useTitle } from "../../ui/useTitle";
import { CodeFinder } from "../CodeFinder";
import { rendersAt } from "../lines";
import { FileFinder } from "./FileFinder";
import { FilePane } from "./FilePane";
import { FilesBar } from "./FilesBar";
import { FilesBody, FilesOther } from "./FilesBody";
import { FilesExplorer } from "./FilesExplorer";
import { FileOverview } from "./FileOverview";
import { FilesPanes } from "./FilesPanes";
import { FilesTree, filesTreeClass } from "./FilesTree";
import { filesTreeLook } from "./filesTreeLook";
import { FilesTreeTools } from "./FilesTreeTools";
import { FilesView } from "./FilesView";
import { FilesOverview } from "./FilesOverview";
import { otherWords, paneChoices, paneWords, type Panes, setPanes, usePanes } from "./panes";
import { ancestors, baseName, changedIn, filesTree } from "./tree";
import { branchName, isMarkdown } from "./words";
import { Text } from "../../ui/Text";

// The file explorer: a worktree's files as they are now, without an
// editor open. A tree on the left (everything, or only what's changed
// since it left its base branch), the file on the right with the lines
// that changed marked, and a fuzzy finder on t (or ⌘P).
//
// FileBrowser is the explorer itself, for any worktree's files: a
// mission's capsule (files/Explorer.tsx), or any repo's ground or capsule
// (the Files page, files/Files.tsx). It knows what's open and where the
// keys are, and gives the Files… parts what to draw.

export interface BrowserProps {
  // api is where its files are read (capsuleAPI, worktreeAPI); label names
  // it, for the tab's title ("demo/greet").
  api: string;
  label: string;
  // head is above the tree and the file, beside tabs: where you are, and
  // how to go elsewhere.
  head: ReactNode;
  // aside is at the row's end, after the tabs: what it's part of (a
  // capsule's mission).
  aside?: ReactNode;
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
  // escape is what esc says it does, onEscape: closing the file (the
  // Files page's), unless it's given another (filesKeys.backToMission).
  escape?: Command;
  // empty is what's said of it with no files.
  empty: string;
}

export function FileBrowser({ api, label, head, aside, tabs, other, before, updated, reviewTo, docOf, search, onEscape, escape = filesKeys.close, empty }: BrowserProps) {
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
  const nodes = useMemo(() => filesTree(listed), [listed]);
  const folders = useMemo(() => foldersOf(nodes), [nodes]);
  const byPath = useMemo(() => new Map(files.map((f) => [f.path, f])), [files]);

  // Folders open: those you've opened, and every one a file you open is
  // in. Showing only what's changed, all are open but those you've
  // folded: it's a short list.
  const [open, setOpen] = useState<ReadonlySet<string>>(() => new Set(ancestors(path)));
  const [shut, setShut] = useState<ReadonlySet<string>>(new Set());
  useEffect(() => {
    if (path) setOpen((o) => (ancestors(path).every((a) => o.has(a)) ? o : new Set([...o, ...ancestors(path)])));
  }, [path]);
  const opened = useMemo(() => (changedOnly ? new Set(folders.filter((f) => !shut.has(f))) : open), [changedOnly, folders, shut, open]);
  const setOpened = (next: Set<string>) => (changedOnly ? setShut(new Set(folders.filter((f) => !next.has(f)))) : setOpen(next));
  // A folder says how many changed files are in it, among all of them.
  const counts = useMemo(() => (changedOnly ? undefined : changedIn(nodes)), [changedOnly, nodes]);
  const look = useCallback((n: TreeNode<TreeFile>, at: { open: boolean }) => filesTreeLook(n, at, counts), [counts]);

  // The tree's keys are its own (ui/Tree) while the focus is in it; the
  // page's keys for them (j, k, o…) press them there from anywhere.
  const treeControl = useRef<TreeControl>(null);
  const press = (key: string) => () => treeControl.current?.press(key);
  // A folder as the tree has it: one that holds only a folder is a row
  // with it, under the inner one's path.
  const rowOf = (folder: string) => folders.find((f) => f === folder || f.startsWith(`${folder}/`)) ?? folder;
  const reveal = (folder: string) => {
    const row = rowOf(folder);
    setOpen((o) => new Set([...o, ...ancestors(`${row}/x`)]));
    treeControl.current?.go(row);
  };

  const select = (p: string) => {
    if (p === path) return;
    const next = new URLSearchParams(params);
    next.set("path", p);
    // A file outside the changed ones shows everything, so it's in the tree.
    if (changedOnly && !byPath.get(p)?.status) next.delete("only");
    setParams(next);
  };
  // A markdown file opens as a page, tied to it: edited with the page's
  // tools, and saved back when you say. ⏎ on the file that's open goes there.
  const docFor = (p: string) => (docOf && isMarkdown(p) && byPath.has(p) ? pageOpenTo(docOf(p)) : undefined);
  // A file picked in the tree opens; ⏎ on the one that's open goes to it
  // as a page. (o and a press don't: entered says which it was.)
  const entered = useRef(false);
  const pick = (n: TreeNode<TreeFile>) => {
    const doc = entered.current && n.id === path ? docFor(path) : undefined;
    if (doc) navigate(doc);
    else select(n.id);
  };
  const enter = () => {
    entered.current = true;
    treeControl.current?.press("Enter");
    entered.current = false;
  };
  const setChangedOnly = (on: boolean) => {
    const next = new URLSearchParams(params);
    if (on) next.set("only", "changed");
    else next.delete("only");
    setParams(next, { replace: true });
  };
  // The changed files in the tree's order, for [ and ].
  const changedOrder = useMemo(() => leavesOf(filesTree(changed)).map((n) => n.id), [changed]);
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
    navigate({ search: `?${next}`, hash: `#L${line}` });
  };
  // Following a name opens its definition here; b comes back, as many
  // times as names were followed (the browser's back does too).
  const followed = useRef(0);
  const openDefinition = (p: string, line: number) => {
    followed.current++;
    openAt(p, line);
  };
  const backFromDefinition = () => {
    if (followed.current <= 0) return false;
    followed.current--;
    navigate(-1);
    return true;
  };
  // Markdown shows rendered, unless a line's asked for (a code search hit,
  // Find's :12, a line's link): only its text has lines, so it opens as
  // text there. m still switches.
  const location = useLocation();
  const [preview, setPreview] = useState(() => rendersAt(location.hash));
  useEffect(() => {
    if (!rendersAt(location.hash)) setPreview(false);
  }, [location.hash, path]);
  // Another tab picked: the tree's keys aren't the page's.
  // On another tab (other), the tree's keys aren't the page's.
  const onTree = !other;
  useCommands(filesKeys, {
    find: onTree && (() => setFinder(true)),
    next: onTree && press("ArrowDown"),
    prev: onTree && press("ArrowUp"),
    enter: onTree && enter,
    open: onTree && press("Enter"),
    openFolder: onTree && press("ArrowRight"),
    closeFolder: onTree && press("ArrowLeft"),
    changed: onTree && (() => setChangedOnly(!changedOnly)),
    prevChanged: onTree && (() => stepChanged(-1)),
    nextChanged: onTree && (() => stepChanged(1)),
    markdown: onTree && (() => setPreview((p) => !p)),
    review: onTree && (() => reviewTo && navigate(reviewTo)),
    copyPath: onTree && (() => path && navigator.clipboard?.writeText(path).then(() => toast(`Copied ${path}`))),
    // On another tab, p shows or folds away the repos beside it.
    panes: other
      ? () => {
          if (narrow || !before) return;
          const next: Panes = shown === 3 ? 2 : 3;
          setPanes(next);
          toast(otherWords[next]!.label, { ms: 1200 });
        }
      : () => {
          if (narrow) return;
          const next = (shown === 1 ? most : shown - 1) as Panes;
          setPanes(next);
          toast(paneWords[next].label, { ms: 1200 });
        },
    reload: onTree && (() => {
      loadTree();
      setReload((n) => n + 1);
    }),
    close: escape === filesKeys.close && onEscape,
    backToMission: escape === filesKeys.backToMission && onEscape,
  });

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
      reveal(path);
    }
    setParams(next, { replace: true });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [folder, path]);
  // A file asked for that isn't in this tree (one open in another
  // capsule, before switching to this one).
  const missing = !!tree && !!path && !byPath.has(path) && !folder;
  // The panes are picked from the row's start, on any tab where it
  // changes something: another tab has only the repos to show or not.
  const panes = paneChoices(shown, most, !!other);
  const panesMenu = !narrow && (!other || !!before) ? <FilesPanes now={panes.now} choices={panes.choices} onPick={setPanes} /> : undefined;
  const top = <FilesBar panes={panesMenu} where={head} tabs={tabs} aside={aside} />;
  if (error && !tree && !other) {
    return (
      <FilesExplorer before={before}>
        {top}
        <div className="page narrow">
          <ErrorCard title="Could not load files" error={error} retry={loadTree} keys={filesKeys.reload} />
        </div>
      </FilesExplorer>
    );
  }
  // With a file open, the panes are as you've picked (narrow, one at a
  // time); with none, the tree's always there, to pick one from.
  const fileOpen = !!path && !folder;
  const alone = !narrow && fileOpen && shown === 1;
  // The repos beside it are as you've picked the panes, with a file open
  // or not: the menu and p always move something. (Another tab has the
  // room of the tree and the file both.)
  const withBefore = !!before && !narrow && shown === 3;
  const showTree = narrow ? !path : !alone;
  const showFile = !narrow || !!path;
  const unpick = () => setParams(changedOnly ? { only: "changed" } : {});
  return (
    <FilesExplorer
      before={withBefore ? before : undefined}
      after={
        <>
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
        </>
      }
    >
      {top}
      {other ? (
        <FilesOther>{other}</FilesOther>
      ) : (
        <FilesBody single={narrow || alone}>
          {showTree && (
            <FilesTree
              panel={withBefore}
              rows={{
                onKeyDownCapture: (e) => {
                  if (e.nativeEvent.isTrusted) entered.current = e.key === "Enter";
                },
                onPointerDownCapture: () => {
                  entered.current = false;
                },
              }}
              tools={
                <FilesTreeTools
                  all={files.length}
                  changed={changed.length}
                  // Nothing to compare with (a repo's ground): just its files.
                  counted={!!tree && !tree.baseRef}
                  changedOnly={changedOnly}
                  canChange={!!tree?.base}
                  title={tree?.base ? `${tree.branch}, branched from ${branchName(tree.baseRef)} at ${tree.base.slice(0, 7)}` : undefined}
                  onShow={setChangedOnly}
                  onFind={() => setFinder(true)}
                  onSearch={search ? () => setSearching(true) : undefined}
                />
              }
              note={tree?.truncated && `Showing first ${files.length} files.`}
            >
              {!tree && <Loading what="Loading files…" />}
              {tree && !nodes.length && (
                <Empty icon={changedOnly ? "check-circle" : "file"} title={changedOnly ? "No changes yet" : "No files"}>
                  {changedOnly ? `No differences from ${branchName(tree.baseRef)}, committed or not.` : empty}
                </Empty>
              )}
              {tree && nodes.length > 0 && (
                <Tree
                  aria-label={label}
                  className={filesTreeClass}
                  nodes={nodes}
                  look={look}
                  open={opened}
                  onOpen={setOpened}
                  current={path || undefined}
                  onPick={pick}
                  control={treeControl}
                  typeahead={false}
                  marks
                  mono
                  virtual
                />
              )}
            </FilesTree>
          )}
          {showFile && (
            <FilesView solid={!narrow && !alone}>
              {missing ? (
                <FilesOverview>
                  <Empty icon="file" title="File not found" action={<Button onClick={unpick}>Pick another file</Button>}>
                    No <Text mono>{path}</Text> in {label}.
                  </Empty>
                </FilesOverview>
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
                  onDefinition={openDefinition}
                  onBackFromDefinition={backFromDefinition}
                  onBack={narrow ? unpick : undefined}
                  onReveal={reveal}
                />
              ) : (
                <FileOverview tree={tree} changed={changed} onOpen={select} onFind={() => setFinder(true)} />
              )}
            </FilesView>
          )}
        </FilesBody>
      )}
    </FilesExplorer>
  );
}
