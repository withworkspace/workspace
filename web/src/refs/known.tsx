import { createContext, type ReactNode, useCallback, useContext, useEffect, useMemo, useRef, useState } from "react";
import { checkRefPaths, getRepos, type Plugin, type Repo, type Workspace } from "../api";
import { useLoaded } from "../load";
import { type Files, groundPaths, type Known, type Named } from "./refs";

// What there is to name, held once for the whole web UI, so a name is
// found the same in a page's editor, a conversation, a thread, a pass or a
// handoff: the repos (with their aliases and capsules), the missions, the
// pages, the conversations and the artifacts, and the kinds of thing the
// workspace's plugins have (a Jira issue), each named as its plugin
// registered. A repo's files aren't read here: a path something shown
// names on one (demo/main.go) is checked when it's shown, against what the
// daemon keeps of the repo, and the answer's kept for the page's life
// (checked again when the workspace says the repos' files have moved).
// Until it's answered, a path that looks like a file is taken for one.

export interface Refs {
  known: Known;
  // repos are the workspace's, as ws labels them: how a repo's drawn.
  repos: Map<string, Repo>;
  // heads are each repo's default branch's commit, as its files were
  // checked: what a link to lines is pinned to.
  heads: Map<string, string>;
  // want checks the paths a text names on the repos' default branches, so
  // they can be found in it.
  want(text: string): void;
}

// Apart from the workspace, which changes with every event: what's drawn
// as text is found again only when what there is to name changes, and a
// ref's label (where a mission stands) follows the workspace by itself.
const RefsContext = createContext<Refs | undefined>(undefined);
const WorkspaceContext = createContext<Workspace | undefined>(undefined);

// useRefs is what there is to name; undefined outside RefsProvider, where
// nothing's found.
export const useRefs = () => useContext(RefsContext);

// NoRefs is somewhere names are only words: inside a card, which mustn't
// open cards of its own.
export function NoRefs({ children }: { children: ReactNode }) {
  return <RefsContext.Provider value={undefined}>{children}</RefsContext.Provider>;
}

// useRefsWorkspace is the workspace, for what a ref's tag and card say.
export const useRefsWorkspace = () => useContext(WorkspaceContext);

const noRepos: Repo[] = [];

const noPlugins: Plugin[] = [];

// thingsNamed is how the plugins' things are named, as each registered:
// what a name is read by (RAM-123), with what one holds, how it's drawn,
// and what can be done to one.
export const thingsNamed = (plugins: Plugin[]): Named[] =>
  plugins.flatMap((p) =>
    (p.refs ?? []).map((r) => ({
      plugin: p.name,
      what: r.kind,
      prefixes: r.prefixes ?? [],
      remote: r.remote,
      tail: r.tail,
      address: r.address,
      path: r.path,
      title: r.title,
      icon: r.icon,
      fields: r.fields,
      views: r.views,
      actions: r.actions,
    })),
  );

// Checked is what's been checked of a repo's default branch: the paths
// answered, and of those the files and the folders.
interface Checked {
  answered: Set<string>;
  files: Set<string>;
  dirs: Set<string>;
}

// filesOf is a repo's files as checked: a path not answered yet is
// neither known to be there nor not.
const filesOf = (c?: Checked): Files => ({
  has: (p) => (c?.answered.has(p) ? c.files.has(p) : undefined),
  hasDir: (p) => (c?.answered.has(p) ? c.dirs.has(p) : undefined),
});

// checkMax is the most paths a check asks about at once.
const checkMax = 2000;

export function RefsProvider({ workspace, plugins = noPlugins, children }: { workspace: Workspace; plugins?: Plugin[]; children: ReactNode }) {
  // A capsule lifted or burned changes what a capsule's file can be.
  const capsules = workspace.missions.flatMap((m) => m.capsules.map((c) => `${c.repo}/${c.name}`)).join(",");
  const repos = useLoaded(() => getRepos().then((rs) => rs.filter((r) => r.cloned)), [capsules], { keep: true }).value ?? noRepos;
  const [checked, setChecked] = useState<Map<string, Checked>>(new Map());
  const [heads, setHeads] = useState<Map<string, string>>(new Map());
  // asked is every path that's been asked about, by repo; queued, those
  // to ask about next, together.
  const asked = useRef(new Map<string, Set<string>>());
  const queued = useRef(new Map<string, Set<string>>());
  const flushing = useRef(false);
  const flush = useCallback(() => {
    if (flushing.current) return;
    flushing.current = true;
    // What's shown together is asked about together: once what's being
    // drawn has said what it names.
    setTimeout(() => {
      flushing.current = false;
      const ask: Record<string, string[]> = {};
      let n = 0;
      for (const [repo, paths] of queued.current) {
        for (const p of paths) {
          if (n >= checkMax) break;
          (ask[repo] ??= []).push(p);
          paths.delete(p);
          n++;
        }
        if (!paths.size) queued.current.delete(repo);
      }
      if (queued.current.size) flush();
      if (!n) return;
      checkRefPaths(ask).then(
        (got) => {
          setChecked((was) => {
            const next = new Map(was);
            for (const [repo, found] of Object.entries(got.repos)) {
              const c = was.get(repo);
              const answered = new Set(c?.answered);
              const files = new Set(c?.files);
              const dirs = new Set(c?.dirs);
              for (const p of ask[repo] ?? []) {
                answered.add(p);
                files.delete(p);
                dirs.delete(p);
              }
              for (const p of found.files) files.add(p);
              for (const p of found.dirs) dirs.add(p);
              next.set(repo, { answered, files, dirs });
            }
            return next;
          });
          setHeads((was) => {
            const next = new Map(was);
            for (const [repo, found] of Object.entries(got.repos)) if (found.head) next.set(repo, found.head);
            return next;
          });
        },
        () => {
          // Asked again next time they're shown.
          for (const [repo, paths] of Object.entries(ask)) for (const p of paths) asked.current.get(repo)?.delete(p);
        },
      );
    }, 0);
  }, []);
  const askAbout = useCallback(
    (repo: string, paths: Iterable<string>) => {
      const q = queued.current.get(repo) ?? new Set<string>();
      queued.current.set(repo, q);
      for (const p of paths) q.add(p);
      flush();
    },
    [flush],
  );
  // The repos as they're named, for finding what a text names of them.
  const naming = useMemo<Known>(
    () => ({
      repos: new Map(repos.map((r) => [r.name, undefined])),
      aliases: new Map(repos.flatMap((r) => (r.aliases ?? []).map((a) => [a, r.name] as [string, string]))),
      root: workspace.root,
      missions: [],
      pages: [],
      talks: [],
    }),
    [repos, workspace.root],
  );
  const want = useCallback(
    (text: string) => {
      for (const [repo, paths] of groundPaths(text, naming)) {
        const had = asked.current.get(repo) ?? new Set<string>();
        asked.current.set(repo, had);
        const fresh = [...paths].filter((p) => !had.has(p));
        if (!fresh.length) continue;
        for (const p of fresh) had.add(p);
        askAbout(repo, fresh);
      }
    },
    [naming, askAbout],
  );
  // The repos' files have moved (or been read for the first time): what's
  // been asked is asked again, and answered as it was until it is.
  const groundsSeen = workspace.groundsSeen;
  const lastSeen = useRef(groundsSeen);
  useEffect(() => {
    if (groundsSeen === lastSeen.current) return;
    lastSeen.current = groundsSeen;
    for (const [repo, paths] of asked.current) if (paths.size) askAbout(repo, paths);
  }, [groundsSeen, askAbout]);
  // The plugins are read again whenever one says anything: what there is
  // to name changes only when what they registered does.
  const named = JSON.stringify(plugins.map((p) => [p.name, p.refs ?? []]));
  const things = useMemo<Named[]>(
    () => thingsNamed(plugins),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [named],
  );
  const names = [
    workspace.root,
    workspace.org,
    workspace.missions.map((m) => m.slug).join(","),
    (workspace.pages ?? []).map((p) => p.id).join(","),
    workspace.conversations.map((t) => t.id).join(","),
    (workspace.artifacts ?? []).map((a) => a.id).join(","),
    (workspace.spaces ?? []).map((s) => `${s.repo}:${s.folder ?? ""}`).join(","),
  ].join("|");
  const known = useMemo<Known>(
    () => ({
      repos: new Map(repos.map((r) => [r.name, filesOf(checked.get(r.name))])),
      aliases: new Map(repos.flatMap((r) => (r.aliases ?? []).map((a) => [a, r.name] as [string, string]))),
      capsules: new Map(repos.map((r) => [r.name, r.capsules])),
      root: workspace.root,
      missions: workspace.missions.map((m) => m.slug),
      pages: (workspace.pages ?? []).map((p) => p.id),
      talks: workspace.conversations.map((t) => t.id),
      artifacts: (workspace.artifacts ?? []).map((a) => a.id),
      org: workspace.org,
      defaultBranch: workspace.defaultBranch,
      branches: new Map(repos.map((r) => [r.name, new Map(Object.entries(r.branches ?? {}).map(([capsule, branch]) => [branch, capsule]))])),
      spaces: (workspace.spaces ?? []).map((s) => ({ repo: s.repo, folder: s.folder })),
      things,
    }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [repos, checked, names, things],
  );
  const byName = useMemo(() => new Map(repos.map((r) => [r.name, r])), [repos]);
  const value = useMemo(() => ({ known, repos: byName, heads, want }), [known, byName, heads, want]);
  return (
    <WorkspaceContext.Provider value={workspace}>
      <RefsContext.Provider value={value}>{children}</RefsContext.Provider>
    </WorkspaceContext.Provider>
  );
}

// RefsScope is what there is to name, as it's given: for somewhere
// outside the pages that draws refs of its own (Find's pane, which names
// what a plugin's thing points at). It checks no repo's files: what it's
// given is what's found.
export function RefsScope({ workspace, known, children }: { workspace: Workspace; known: Known; children: ReactNode }) {
  const value = useMemo<Refs>(() => ({ known, repos: new Map(), heads: new Map(), want: () => {} }), [known]);
  return (
    <WorkspaceContext.Provider value={workspace}>
      <RefsContext.Provider value={value}>{children}</RefsContext.Provider>
    </WorkspaceContext.Provider>
  );
}

// useWanted checks the paths a text names on the repos' default branches,
// when it's shown.
export function useWanted(text: string) {
  const want = useRefs()?.want;
  useEffect(() => want?.(text), [want, text]);
}
