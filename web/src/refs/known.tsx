import { createContext, type ReactNode, useCallback, useContext, useEffect, useMemo, useRef, useState } from "react";
import { getRepos, getTree, type Plugin, type Repo, type Workspace, worktreeAPI } from "../api";
import { useLoaded } from "../load";
import { GROUND, type Known, type Named, reposNamed } from "./refs";

// What there is to name, held once for the whole web UI, so a name is
// found the same in a page's editor, a conversation, a thread, a pass or a
// handoff: the repos (with their aliases and capsules), the missions, the
// pages, the conversations and the artifacts, and the kinds of thing the
// workspace's plugins have (a Jira issue), each named as its plugin
// registered. A repo's files are read the first time something shown names
// the repo (demo/…), and kept.

export interface Refs {
  known: Known;
  // repos are the workspace's, as ws labels them: how a repo's drawn.
  repos: Map<string, Repo>;
  // heads are each repo's default branch's commit, as its files were read:
  // what a link to lines is pinned to.
  heads: Map<string, string>;
  // want reads the files of the repos a text names, so they can be found
  // in it.
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

export function RefsProvider({ workspace, plugins = noPlugins, children }: { workspace: Workspace; plugins?: Plugin[]; children: ReactNode }) {
  // A capsule lifted or burned changes what a capsule's file can be.
  const capsules = workspace.missions.flatMap((m) => m.capsules.map((c) => `${c.repo}/${c.name}`)).join(",");
  const repos = useLoaded(() => getRepos().then((rs) => rs.filter((r) => r.cloned)), [capsules], { keep: true }).value ?? noRepos;
  const [trees, setTrees] = useState<Map<string, Set<string>>>(new Map());
  const [heads, setHeads] = useState<Map<string, string>>(new Map());
  const asked = useRef(new Set<string>());
  const names = useMemo(() => {
    const m = new Map<string, string>();
    for (const r of repos) {
      m.set(r.name, r.name);
      for (const a of r.aliases ?? []) if (!m.has(a)) m.set(a, r.name);
    }
    return m;
  }, [repos]);
  const want = useCallback(
    (text: string) => {
      for (const name of reposNamed(text, [...names.keys()])) {
        const repo = names.get(name)!;
        if (asked.current.has(repo)) continue;
        asked.current.add(repo);
        getTree(worktreeAPI(repo, GROUND)).then(
          (t) => {
            setTrees((was) => new Map(was).set(repo, new Set(t.files.map((f) => f.path))));
            if (t.head) setHeads((was) => new Map(was).set(repo, t.head!));
          },
          () => asked.current.delete(repo),
        );
      }
    },
    [names],
  );
  // The plugins are read again whenever one says anything: what there is
  // to name changes only when what they registered does.
  const named = JSON.stringify(plugins.map((p) => [p.name, p.refs ?? []]));
  const things = useMemo<Named[]>(
    () => thingsNamed(plugins),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [named],
  );
  const naming = [
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
      repos: new Map(repos.map((r) => [r.name, trees.get(r.name)])),
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
    [repos, trees, naming, things],
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
// what a plugin's thing points at). It reads no repo's files: what it's
// given is what's found.
export function RefsScope({ workspace, known, children }: { workspace: Workspace; known: Known; children: ReactNode }) {
  const value = useMemo<Refs>(() => ({ known, repos: new Map(), heads: new Map(), want: () => {} }), [known]);
  return (
    <WorkspaceContext.Provider value={workspace}>
      <RefsContext.Provider value={value}>{children}</RefsContext.Provider>
    </WorkspaceContext.Provider>
  );
}

// useWanted reads the files of the repos a text names, when it's shown.
export function useWanted(text: string) {
  const want = useRefs()?.want;
  useEffect(() => want?.(text), [want, text]);
}
