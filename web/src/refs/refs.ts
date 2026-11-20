// Refs: the things words name, found where they're shown, so each can be
// gone to and said more about without being linked by hand. One finder,
// for a page's editor and for every reader (a conversation, a thread, a
// pass, a handoff), so a name is the same thing wherever it's written.
// Agents are told to write these (internal/prompt/naming.go).
//
//   demo/main.go                         a file on a repo's default branch
//   demo/main.go:12-20                   the same, at those lines
//   demo/token-cache:internal/cache.go   a file in a capsule (and :12-20)
//   demo/internal/                       a folder (its slash is optional)
//   demo                                 a repo, by its name
//   demo#12                              a pull request
//   fix-token-refresh                    a mission, by its slug
//   p3, w4                               another page, a conversation
//   a3, a3.v2                            an artifact, a version of one
//   RAM-123                              a plugin's thing, by a name it registered
//
// It also reads what agents write instead: a repo's alias for the repo,
// and a file as it is on disk (repos/demo/token-cache/internal/cache.go,
// and that under the workspace's folder, /Users/…/repos/demo/.ground/…).
//
// A ref is found only where it names something that's there: a file in its
// repo's tree, a capsule the repo has, a mission the workspace has. So
// words that only look like one (and/or, a path in some other project) are
// left as words.

export const GROUND = ".ground";

export type PageRef =
  | { kind: "file"; repo: string; worktree: string; path: string; line?: number; endLine?: number; commit?: string; anchor?: string }
  | { kind: "dir"; repo: string; worktree: string; path: string; commit?: string }
  | { kind: "repo"; repo: string }
  | { kind: "pr"; repo: string; number: number }
  | { kind: "mission"; slug: string }
  | { kind: "page"; id: string }
  | { kind: "talk"; id: string }
  | { kind: "artifact"; id: string; version?: number }
  // Something a plugin has (a Jira issue): the plugin, its word for the
  // kind of thing, the thing's name, and where it is outside ws, when the
  // plugin said where its things are.
  | { kind: "thing"; plugin: string; what: string; id: string; url?: string };

// ThingField is one of the fields a kind of plugin's thing has: its name,
// what it is (title, state, text, time, markdown, or ref and refs, whose
// value names other things; one ws doesn't know is drawn as text), and
// what it's called where it's said beside its value.
export interface ThingField {
  name: string;
  type: string;
  label?: string;
}

// ThingBlock is a piece of how a plugin's thing is drawn (a title, a
// subtitle, a badge, a field with its label, or markdown), with the field
// whose value it shows: key is the thing's own name. label is a field's,
// in place of the one it was declared with; lines is how many lines its
// markdown is kept to. A block ws doesn't know isn't drawn.
export interface ThingBlock {
  block: string;
  field: string;
  label?: string;
  lines?: number;
}

// ThingAction is something a plugin does when asked: to one of its
// things (a kind's actions), or of its own (the plugin's). hint is more
// words it's found by.
export interface ThingAction {
  id: string;
  title: string;
  icon?: string;
  hint?: string;
}

// ThingViews are how a kind of plugin's thing is drawn, as its plugin
// declared, where it's drawn: the card under its name, and the pane beside
// the list in Find.
export interface ThingViews {
  card?: ThingBlock[];
  pane?: ThingBlock[];
}

// Named is a kind of thing a plugin has, and how one is named, as the
// plugin registered it: a name is one of prefixes and then its tail (a
// number, when it doesn't say). address is where one is outside ws, and
// path where one opens in the plugin's app, {ref} its name in each. title
// and icon are how ws draws one; fields are what one holds, views how the
// plugin has it drawn from them (refs/thing.ts), and actions what can be
// done to one.
export interface Named {
  plugin: string;
  what: string;
  prefixes: string[];
  tail?: string;
  address?: string;
  path?: string;
  title?: string;
  icon?: string;
  fields?: ThingField[];
  views?: ThingViews;
  actions?: ThingAction[];
  // Its things can live in a repo: one that does is named by its folder
  // there (records/sketches/release-flow/), and opens where the kind's
  // do, by that name.
  remote?: boolean;
}

// Files is a repo's files on its default branch, as far as they're known:
// all of them (a Set of their paths), or what's been checked of them so far
// (refs/known.tsx), which says undefined of a path that hasn't been.
export interface Files {
  has(path: string): boolean | undefined;
  hasDir?(path: string): boolean | undefined;
}

// Known is what there is to name: each repo with the files on its default
// branch (undefined while none are known), the missions' slugs, and
// the pages, the conversations and the artifacts by id. With aliases (alias to repo), a
// repo's alias names it; with capsules (repo to its capsules), a capsule's
// file is named repo/capsule:path; with root (the workspace's folder), a
// path on disk under it is read as the file it is.
export interface Known {
  repos: Map<string, Files | undefined>;
  aliases?: Map<string, string>;
  capsules?: Map<string, string[]>;
  root?: string;
  missions: string[];
  pages: string[];
  talks: string[];
  artifacts?: string[];
  // For reading addresses: the workspace's GitHub org, its default branch,
  // and each repo's capsules by the branch they're on.
  org?: string;
  defaultBranch?: string;
  branches?: Map<string, Map<string, string>>;
  // The workspace's spaces of documents: a file that's one of their
  // documents goes to its address in Documents.
  spaces?: { repo: string; folder?: string }[];
  // The kinds of thing plugins have, and how each is named: the first to
  // name some words has them.
  things?: Named[];
}

export interface Found {
  from: number;
  to: number;
  ref: PageRef;
}

const esc = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
const alt = (names: string[]) =>
  [...names]
    .sort((a, b) => b.length - a.length)
    .map(esc)
    .join("|");

const seg = "[\\w.@+~-]+";
const filePath = `${seg}(?:/${seg})*`;
const lineRange = "(?::(\\d+)(?:-(\\d+))?)?";
const before = "(?<![\\w./@#~-])";

// What follows a prefix in a plugin's thing's name, by the tail its kind
// registered. One with a tail that isn't here isn't found in words.
const tails: Record<string, string> = { number: "[1-9]\\d{0,8}" };

// thingRef is a plugin's thing by its name, at its address if its kind
// has one.
function thingRef(of: Named, id: string): PageRef {
  return { kind: "thing", plugin: of.plugin, what: of.what, id, ...(of.address ? { url: of.address.replace("{ref}", id) } : {}) };
}

// thingNamed is the plugin's thing a whole name is, if it's one.
function thingNamed(name: string, known: Known): PageRef | undefined {
  for (const t of patternsOf(known).things) if (t.whole.test(name)) return thingRef(t.of, name);
  return undefined;
}

// A folder in a repo, as ws names one: records/sketches/release-flow/.
const folderName = /^[\w.-]+\/(?:[^/]+\/)+$/;

// names is whether a kind's page is of the thing called name: one of the
// kind's names, or, for a kind whose things can live in a repo, the
// folder one lives in.
const names = (t: { of: Named; whole: RegExp }, name: string) => t.whole.test(name) || (!!t.of.remote && folderName.test(name));

// thingAtPath is the plugin's thing whose page in ws is at path (as its
// plugin registered where one is, /x/jira/issue/RAM-123), if it's one.
export function thingAtPath(path: string, known: Known): PageRef | undefined {
  for (const t of patternsOf(known).things) {
    const where = t.of.path;
    const hole = where?.indexOf("{ref}") ?? -1;
    if (!where || hole < 0) continue;
    const before = `/x/${encodeURIComponent(t.of.plugin)}${where.slice(0, hole)}`;
    const after = where.slice(hole + "{ref}".length);
    if (path.length <= before.length + after.length || !path.startsWith(before) || !path.endsWith(after)) continue;
    const name = decodeURIComponent(path.slice(before.length, path.length - after.length));
    if (names(t, name)) return thingRef(t.of, name);
  }
  return undefined;
}

// refNamed is what a name, as ws writes it, names: a plugin's thing by
// how its plugin names them (RAM-123), anything else by its form.
export function refNamed(name: string, known?: Known): PageRef | undefined {
  return (known && thingNamed(name, known)) || parseName(name);
}

type Lines = { line?: number; endLine?: number };
const linesOf = (from?: string, to?: string): Lines => (from ? { line: Number(from), ...(to ? { endLine: Number(to) } : {}) } : {});

// dirsOf is the folders a repo's files are in, worked out once per tree.
const dirsCache = new WeakMap<Set<string>, Set<string>>();
function dirsOf(tree: Set<string>): Set<string> {
  let dirs = dirsCache.get(tree);
  if (!dirs) {
    dirs = new Set();
    for (const f of tree) for (let i = f.indexOf("/"); i > 0; i = f.indexOf("/", i + 1)) dirs.add(f.slice(0, i));
    dirsCache.set(tree, dirs);
  }
  return dirs;
}

// isDir is whether a repo's files have a folder at path: undefined when
// that hasn't been checked.
function isDir(tree: Files, path: string): boolean | undefined {
  if (tree.hasDir) return tree.hasDir(path);
  return tree instanceof Set ? dirsOf(tree).has(path) : undefined;
}

// looksLikeFile is a path in a capsule, which isn't checked against a tree:
// it has to look like a file, or name lines. A sentence's full stop is
// taken off it.
function looksLikeFile(path: string, lines: Lines): string | undefined {
  if (lines.line) return path;
  const bare = path.replace(/\.+$/, "");
  return /\.[\w+-]+$/.test(bare) ? bare : undefined;
}

// Patterns are a Known's regular expressions, made once for it: a log of
// hundreds of lines is looked through with the same ones.
interface Patterns {
  ground?: RegExp;
  inCapsule?: RegExp;
  onDisk?: RegExp;
  bareRepo?: RegExp;
  prs?: RegExp;
  missions?: RegExp;
  repoAtEnd?: RegExp;
  // Each kind of plugin's thing that's found in words, with what finds
  // one in a run of text, and what a whole name is.
  things: { of: Named; find: RegExp; whole: RegExp }[];
}
const patternsCache = new WeakMap<Known, Patterns>();
function patternsOf(known: Known): Patterns {
  let p = patternsCache.get(known);
  if (p) return p;
  p = { things: [] };
  for (const of of known.things ?? []) {
    const tail = tails[of.tail ?? "number"];
    if (!tail || !of.prefixes.length) continue;
    const name = `(?:${alt(of.prefixes)})${tail}`;
    p.things.push({ of, find: new RegExp(`(?<![\\w/.#-])(${name})(?![\\w/#-])`, "g"), whole: new RegExp(`^${name}$`) });
  }
  const repos = [...known.repos.keys()];
  const aliases = [...(known.aliases?.keys() ?? [])].filter((a) => !known.repos.has(a) && known.repos.has(known.aliases!.get(a)!));
  const names = alt([...repos, ...aliases]);
  if (repos.length) {
    p.ground = new RegExp(`${before}(${names})/(${filePath})${lineRange}`, "g");
    const capsules = [...new Set([...(known.capsules?.values() ?? [])].flat())];
    if (capsules.length) p.inCapsule = new RegExp(`${before}(${names})/(${alt(capsules)}):(${filePath})${lineRange}`, "g");
    const root = known.root ? `(?:${esc(known.root.replace(/\/+$/, ""))}/)?` : "";
    p.onDisk = new RegExp(`${before}${root}repos/(${alt(repos)})/(${seg})/(${filePath})${lineRange}`, "g");
    p.bareRepo = new RegExp(`(?<![\\w./@#:~-])(${alt(repos)})(?![\\w/#:@~-]|\\.\\w)`, "g");
    p.prs = new RegExp(`(?<![\\w./@#-])(${names})#([1-9]\\d{0,6})(?![\\w#])`, "g");
    p.repoAtEnd = new RegExp(`(?<![\\w./@#:~-])(${names})\\s+$`);
  }
  const slugs = known.missions.filter((s) => s.length >= 3);
  if (slugs.length) p.missions = new RegExp(`(?<![\\w/.#-])(${alt(slugs)})(?![\\w/#-])`, "g");
  patternsCache.set(known, p);
  return p;
}

// findRefs is the refs in a run of text, in order, none over another.
export function findRefs(text: string, known: Known): Found[] {
  const out: Found[] = [];
  const add = (from: number, to: number, ref: PageRef) => out.push({ from, to, ref });
  const pat = patternsOf(known);
  const repoOf = (name: string) => (known.repos.has(name) ? name : (known.aliases?.get(name) ?? name));
  if (pat.ground) {
    // A file on the default branch: checked against its tree.
    const ground = pat.ground;
    for (const m of text.matchAll(ground)) {
      const at = m.index ?? 0;
      const repo = repoOf(m[1]);
      const tree = known.repos.get(repo);
      if (!tree) continue;
      let path = m[2];
      let lines = linesOf(m[3], m[4]);
      let to = at + m[0].length;
      const bare = path.replace(/\.+$/, "");
      const file = tree.has(path);
      if (file !== true) {
        // A folder: named with its slash after it, or without.
        const dir = !m[3] && [path, bare].find((p) => isDir(tree, p) === true);
        if (dir) {
          to = at + m[1].length + 1 + dir.length + (text[at + m[1].length + 1 + dir.length] === "/" ? 1 : 0);
          add(at, to, { kind: "dir", repo, worktree: GROUND, path: dir });
          continue;
        }
        // A sentence's full stop, taken for the path's.
        const fileBare = !m[3] && bare !== path ? tree.has(bare) : undefined;
        // Not checked yet: what looks like a file is taken for one, as a
        // capsule's is, until it's checked: the words don't change when
        // it's found there.
        const looks = file === undefined && fileBare !== false ? looksLikeFile(path, lines) : undefined;
        const named = fileBare === true ? bare : looks;
        if (!named) continue;
        if (named !== path) {
          to -= path.length - named.length;
          path = named;
          lines = {};
        }
      }
      add(at, to, { kind: "file", repo, worktree: GROUND, path, ...lines });
    }
    // A file in a capsule, by its name: repo/capsule:path.
    if (pat.inCapsule) {
      const inCapsule = pat.inCapsule;
      for (const m of text.matchAll(inCapsule)) {
        const repo = repoOf(m[1]);
        if (!known.capsules?.get(repo)?.includes(m[2])) continue;
        const lines = linesOf(m[4], m[5]);
        const path = looksLikeFile(m[3], lines);
        if (!path) continue;
        const at = m.index ?? 0;
        add(at, at + m[0].length - (m[3].length - path.length), { kind: "file", repo, worktree: m[2], path, ...lines });
      }
    }
    // A file as it is on disk: repos/<repo>/<worktree>/<path>, maybe under
    // the workspace's folder. A path in some other folder isn't one.
    for (const m of text.matchAll(pat.onDisk!)) {
      const at = m.index ?? 0;
      const repo = m[1];
      const worktree = m[2];
      let path: string | undefined = m[3];
      const lines = linesOf(m[4], m[5]);
      if (worktree === GROUND) {
        const tree = known.repos.get(repo);
        if (!tree) continue;
        const file = tree.has(path);
        const bare = path.replace(/\.+$/, "");
        const fileBare = !lines.line && bare !== path ? tree.has(bare) : undefined;
        // Not checked yet: taken for a file if it looks like one.
        path = file ? path : fileBare ? bare : file === undefined && fileBare !== false ? looksLikeFile(path, lines) : undefined;
      } else {
        path = looksLikeFile(path, lines);
      }
      if (!path) continue;
      add(at, at + m[0].length - (m[3].length - path.length), { kind: "file", repo, worktree, path, ...lines });
    }
    // A repo by its name alone (not an alias: those are short, and words).
    for (const m of text.matchAll(pat.bareRepo!)) add(m.index ?? 0, (m.index ?? 0) + m[0].length, { kind: "repo", repo: m[1] });
    for (const m of text.matchAll(pat.prs!)) add(m.index ?? 0, (m.index ?? 0) + m[0].length, { kind: "pr", repo: repoOf(m[1]), number: Number(m[2]) });
  }
  if (pat.missions) {
    for (const m of text.matchAll(pat.missions)) add(m.index ?? 0, (m.index ?? 0) + m[0].length, { kind: "mission", slug: m[1] });
  }
  for (const m of text.matchAll(/(?<![\w/.#-])([pw])([1-9]\d{0,4})(?![\w/#-])/g)) {
    const id = m[1] + m[2];
    if (m[1] === "p" && known.pages.includes(id)) add(m.index ?? 0, (m.index ?? 0) + m[0].length, { kind: "page", id });
    if (m[1] === "w" && known.talks.includes(id)) add(m.index ?? 0, (m.index ?? 0) + m[0].length, { kind: "talk", id });
  }
  // An artifact (a3), or a version of one (a3.v2): the artifact has to be
  // there; whether the version is, its card says.
  if (known.artifacts?.length) {
    for (const m of text.matchAll(/(?<![\w/.#-])(a[1-9]\d{0,6})(?:\.v([1-9]\d{0,4}))?(?![\w/#-])/g)) {
      if (known.artifacts.includes(m[1])) add(m.index ?? 0, (m.index ?? 0) + m[0].length, artifactRef(m[1], m[2]));
    }
  }
  // A plugin's thing, by a name its kind registered: the first kind to
  // name some words has them.
  for (const t of pat.things) {
    for (const m of text.matchAll(t.find)) add(m.index ?? 0, (m.index ?? 0) + m[0].length, thingRef(t.of, m[1]));
  }
  out.sort((a, b) => a.from - b.from || b.to - a.to);
  return out.filter((f, i) => i === 0 || f.from >= out[i - 1].to);
}

// refAt is the ref that's the whole of a text (a link's address, a name in
// backticks), if it is one: an address, or a name that's there.
export function refAt(text: string, known: Known): PageRef | undefined {
  const t = text.trim();
  const a = parseAddress(t, known);
  if (a) return a;
  const f = findRefs(t, known);
  return f.length === 1 && f[0].from === 0 && f[0].to === t.length ? f[0].ref : undefined;
}

// repoThenPath is a repo named at the end of some words with a path in it
// after them, in backticks: cr-records `bp/proposed/plan.md`, as a writer
// names a file in the repo they've just named. It's the file (or folder, or
// a capsule's file), from the repo's name on. A path that isn't there
// leaves the name as the repo.
export function repoThenPath(before: string, code: string, known: Known): { from: number; ref: PageRef } | undefined {
  const m = patternsOf(known).repoAtEnd?.exec(before);
  if (!m) return undefined;
  const ref = refAt(`${m[1]}/${code.trim().replace(/^\/+/, "")}`, known);
  return ref && (ref.kind === "file" || ref.kind === "dir") ? { from: m.index, ref } : undefined;
}

// --- Pinned: names as they were when they were said

// Pinned is a name as it was when a turn said it (ref.Pinned): what it
// named then, with the commit its lines were of, and the branch a
// capsule's file was on. Drawn from that, a conversation says what it
// meant after the capsule's burned or the lines have moved.
export interface Pinned {
  said: string;
  ref: PageRef;
  branch?: string;
  pushed?: boolean;
}

const notBefore = /[\w./@#~-]/;
const notAfter = /[\w/#@~-]/;

// findPinned is where a turn's pinned names are in its words, as words:
// the longest first, none over another, and only whole (demo/main.go
// isn't found in demo/main.go:3).
export function findPinned(text: string, pinned: Pinned[]): Found[] {
  const out: Found[] = [];
  const taken = (from: number, to: number) => out.some((f) => from < f.to && to > f.from);
  for (const p of [...pinned].sort((a, b) => b.said.length - a.said.length)) {
    if (!p.said) continue;
    for (let at = text.indexOf(p.said); at >= 0; at = text.indexOf(p.said, at + 1)) {
      const to = at + p.said.length;
      if (at > 0 && notBefore.test(text[at - 1]) && !p.said.startsWith("/")) continue;
      if (to < text.length && (notAfter.test(text[to]) || (text[to] === ":" && /\d/.test(text[to + 1] ?? "")))) continue;
      if (!taken(at, to)) out.push({ from: at, to, ref: p.ref });
    }
  }
  return out.sort((a, b) => a.from - b.from);
}

// findAll is the refs in words with what a turn pinned first: what it
// said is what it meant, and the rest is found as it is now.
export function findAll(text: string, known: Known, pinned?: Pinned[]): Found[] {
  if (!pinned?.length) return findRefs(text, known);
  const kept = findPinned(text, pinned);
  for (const f of findRefs(text, known)) if (!kept.some((k) => f.from < k.to && f.to > k.from)) kept.push(f);
  return kept.sort((a, b) => a.from - b.from);
}

// --- Names and addresses, as internal/ref has them (the same cases test
// both: internal/ref/testdata/cases.json).

const lineSuffix = (line?: number, end?: number) => (!line ? "" : end && end > line ? `:${line}-${end}` : `:${line}`);
const inCapsule = (r: { worktree: string }) => !!r.worktree && r.worktree !== GROUND;

// refName is how a ref is written.
export function refName(r: PageRef): string {
  switch (r.kind) {
    case "file":
    case "dir": {
      const where = inCapsule(r) ? `${r.repo}/${r.worktree}:` : `${r.repo}/`;
      return r.kind === "dir" ? `${where}${r.path}/` : `${where}${r.path}${lineSuffix(r.line, r.endLine)}`;
    }
    case "repo":
      return r.repo;
    case "pr":
      return `${r.repo}#${r.number}`;
    case "mission":
      return r.slug;
    case "page":
    case "talk":
      return r.id;
    case "artifact":
      return r.version ? `${r.id}.v${r.version}` : r.id;
    case "thing":
      return r.id;
  }
}

// Where is what an address needs that a ref doesn't say.
export interface Where {
  org: string;
  defaultBranch: string;
  branch?: string; // a capsule's, once it's pushed
}

const escapePath = (p: string) => p.split("/").map(encodeURIComponent).join("/");

// refAddress is where a ref is outside ws: on GitHub, where a plugin said
// its thing is, or ws:<name> for what only ws has.
export function refAddress(r: PageRef, w: Where): string {
  if (r.kind === "thing") return r.url ?? `ws:${r.id}`;
  const gh = `https://github.com/${w.org}/${"repo" in r ? r.repo : ""}`;
  switch (r.kind) {
    case "repo":
      return gh;
    case "pr":
      return `${gh}/pull/${r.number}`;
    case "file":
    case "dir": {
      let rev = w.defaultBranch;
      if (inCapsule(r)) {
        if (!w.branch) return wsAddress(r);
        rev = w.branch;
      }
      if (r.commit) rev = r.commit;
      if (r.kind === "dir") return `${gh}/tree/${escapePath(rev)}/${escapePath(r.path)}`;
      const frag = r.line ? `#L${r.line}${r.endLine && r.endLine > r.line ? `-L${r.endLine}` : ""}` : r.anchor ? `#${r.anchor}` : "";
      return `${gh}/blob/${escapePath(rev)}/${escapePath(r.path)}${frag}`;
    }
  }
  return `ws:${refName(r)}`;
}

// wsAddress is a ref's ws: address, at its heading if it names one: how
// the reader's links to a document's sections are written
// (ws:demo/adr/0026.md#decision), read back by parseAddress.
export function wsAddress(r: PageRef): string {
  const heading = r.kind === "file" && !r.line && r.anchor ? `#${r.anchor}` : "";
  return `ws:${refName(r)}${heading}`;
}

const prName = /^([\w.-]+)#([1-9]\d{0,8})$/;
// An artifact, or a version of one, as internal/artifact names them.
const artifactName = /^(a[1-9]\d{0,6})(?:\.v([1-9]\d{0,4}))?$/;
const artifactRef = (id: string, version?: string): PageRef => (version ? { kind: "artifact", id, version: Number(version) } : { kind: "artifact", id });
const githubURL = /^https?:\/\/(?:www\.)?github\.com\/([^/\s?#]+)\/([^/\s?#]+)(?:\/([^\s?#]*))?(?:\?[^\s#]*)?(?:#(\S*))?$/;
const linesFrag = /^L(\d+)(?:C\d+)?(?:-L(\d+)(?:C\d+)?)?$/;
const isCommit = (rev: string) => /^[0-9a-f]{7,40}$/.test(rev);

function splitLines(s: string): [string, number?, number?] {
  const m = /^(.*?):(\d+)(?:-(\d+))?$/.exec(s);
  if (!m || Number(m[2]) === 0) return [s];
  const line = Number(m[2]);
  const end = m[3] && Number(m[3]) > line ? Number(m[3]) : undefined;
  return [m[1], line, end];
}

// A heading's name, as GitHub makes one (markdown/links.ts).
const anchorName = /^[\p{L}\p{M}\p{N}\p{Pc}-]+$/u;

// parseName reads one name by its form alone, without knowing whether
// what it names is there (a ws: address). A file's name may end in a
// heading in it (demo/adr/0026.md#decision).
export function parseName(s: string): PageRef | undefined {
  s = s.trim();
  const cut = s.indexOf("#");
  if (cut >= 0 && anchorName.test(s.slice(cut + 1))) {
    const r = nameOf(s.slice(0, cut));
    if (r?.kind === "file" && !r.line) return { ...r, anchor: s.slice(cut + 1) };
  }
  return nameOf(s);
}

function nameOf(s: string): PageRef | undefined {
  if (!s || /\s/.test(s)) return undefined;
  const pr = prName.exec(s);
  if (pr) return { kind: "pr", repo: pr[1], number: Number(pr[2]) };
  const id = /^([pw])([1-9]\d{0,4})$/.exec(s);
  if (id) return id[1] === "p" ? { kind: "page", id: s } : { kind: "talk", id: s };
  const art = artifactName.exec(s);
  if (art) return artifactRef(art[1], art[2]);
  const slash = s.indexOf("/");
  // demo:README.md or records:adr/0001.md:12-20, as a document's ref is
  // stored: a colon before any slash, and what's after it is the path,
  // folders and all. Not https://…, whose colon is followed by a slash.
  const stored = s.indexOf(":");
  if (stored > 0 && stored < s.length - 1 && (slash < 0 || stored < slash) && s[stored + 1] !== "/") {
    const repo = s.slice(0, stored);
    const p = s.slice(stored + 1);
    if (p.endsWith("/")) return { kind: "dir", repo, worktree: GROUND, path: p.slice(0, -1) };
    const [path, line, endLine] = splitLines(p);
    return withLines({ kind: "file", repo, worktree: GROUND, path }, line, endLine);
  }
  if (slash < 0) return /^[a-z0-9][a-z0-9-]*$/.test(s) ? { kind: "mission", slug: s } : undefined;
  const repo = s.slice(0, slash);
  const rest = s.slice(slash + 1);
  if (!repo || !rest) return undefined;
  const colon = rest.indexOf(":");
  if (colon > 0 && !rest.slice(0, colon).includes("/")) {
    const p = rest.slice(colon + 1);
    if (p && !/^\d/.test(p.split("-")[0])) {
      const capsule = rest.slice(0, colon);
      if (p.endsWith("/")) return { kind: "dir", repo, worktree: capsule, path: p.slice(0, -1) };
      const [path, line, endLine] = splitLines(p);
      return withLines({ kind: "file", repo, worktree: capsule, path }, line, endLine);
    }
  }
  if (rest.endsWith("/")) return { kind: "dir", repo, worktree: GROUND, path: rest.slice(0, -1) };
  const [path, line, endLine] = splitLines(rest);
  return withLines({ kind: "file", repo, worktree: GROUND, path }, line, endLine);
}

function withLines(r: Extract<PageRef, { kind: "file" }>, line?: number, endLine?: number): PageRef {
  return { ...r, ...(line ? { line } : {}), ...(endLine ? { endLine } : {}) };
}

// parseAddress reads an address back into what it names: one of the
// workspace's on GitHub, or a ws: one. What it names needn't be there now:
// it was named on purpose.
export function parseAddress(addr: string, known: Known): PageRef | undefined {
  if (addr.startsWith("ws:")) return thingNamed(addr.slice(3), known) ?? parseName(addr.slice(3));
  const thing = thingAt(addr, known);
  if (thing) return thing;
  const m = githubURL.exec(addr);
  if (!m) return undefined;
  const [, org, repoRaw, restRaw, frag] = m;
  const repo = repoRaw.replace(/\.git$/, "");
  if ((known.org && org.toLowerCase() !== known.org.toLowerCase()) || !known.repos.has(repo)) return undefined;
  const segs = (restRaw ?? "")
    .replace(/^\/+|\/+$/g, "")
    .split("/")
    .filter((x) => x !== "")
    .map((x) => {
      try {
        return decodeURIComponent(x);
      } catch {
        return x;
      }
    });
  const what = segs.shift() ?? "";
  if (what === "") return segs.length === 0 ? { kind: "repo", repo } : undefined;
  if (what === "pull") {
    const n = Number(segs[0]);
    return Number.isInteger(n) && n > 0 ? { kind: "pr", repo, number: n } : undefined;
  }
  if (what !== "blob" && what !== "tree") return undefined;
  const branches = known.branches?.get(repo);
  const lines = frag ? linesFrag.exec(frag) : null;
  const pick = (rev: string, path: string): PageRef => {
    let worktree = GROUND;
    let commit: string | undefined;
    if (isCommit(rev)) commit = rev;
    else if (rev !== known.defaultBranch && branches?.get(rev)) worktree = branches.get(rev)!;
    if (what === "tree") return { kind: "dir", repo, worktree, path, ...(commit ? { commit } : {}) };
    const line = lines ? Number(lines[1]) : undefined;
    const end = lines?.[2] && Number(lines[2]) > Number(lines[1]) ? Number(lines[2]) : undefined;
    const anchor = frag && !lines ? frag : undefined;
    return { kind: "file", repo, worktree, path, ...(line ? { line } : {}), ...(end ? { endLine: end } : {}), ...(commit ? { commit } : {}), ...(anchor ? { anchor } : {}) };
  };
  for (let i = 1; i < segs.length; i++) {
    const rev = segs.slice(0, i).join("/");
    if (isCommit(rev) || rev === known.defaultBranch || branches?.get(rev)) return pick(rev, segs.slice(i).join("/"));
  }
  return segs.length > 1 ? pick(segs[0], segs.slice(1).join("/")) : undefined;
}

// thingAt is the plugin's thing an address is, where its kind said its
// things are: the kind's address with one of its names where {ref} is.
// What follows after a ? or a # is beside the point (?focusedCommentId=…),
// unless the kind's address has one itself.
function thingAt(addr: string, known: Known): PageRef | undefined {
  const cut = addr.search(/[?#]/);
  const bare = cut < 0 ? addr : addr.slice(0, cut);
  for (const t of patternsOf(known).things) {
    const where = t.of.address;
    const hole = where?.indexOf("{ref}") ?? -1;
    if (!where || hole < 0) continue;
    const at = /[?#]/.test(where) ? addr : bare;
    const before = where.slice(0, hole);
    const after = where.slice(hole + "{ref}".length);
    if (at.length <= before.length + after.length || !at.startsWith(before) || !at.endsWith(after)) continue;
    const name = at.slice(before.length, at.length - after.length);
    if (t.whole.test(name)) return thingRef(t.of, name);
  }
  return undefined;
}

// thingOpenAt is the plugin's thing a page of its app is open on: the path
// in the app (after /x/<plugin>) where its kind said one opens, its name
// where {ref} is. A page of the app that isn't one thing's is none.
export function thingOpenAt(plugin: string, path: string, known: Known): Extract<PageRef, { kind: "thing" }> | undefined {
  for (const t of patternsOf(known).things) {
    const where = t.of.path;
    const hole = where?.indexOf("{ref}") ?? -1;
    if (t.of.plugin !== plugin || !where || hole < 0) continue;
    const before = where.slice(0, hole);
    const after = where.slice(hole + "{ref}".length);
    if (path.length <= before.length + after.length || !path.startsWith(before) || !path.endsWith(after)) continue;
    const name = decodeURIComponent(path.slice(before.length, path.length - after.length));
    if (names(t, name)) return thingRef(t.of, name) as Extract<PageRef, { kind: "thing" }>;
  }
  return undefined;
}

// reposNamed are the repos a text may name files of (demo/…, or demo
// `main.go`): those whose files are worth reading, to tell.
export function reposNamed(text: string, repos: string[]): string[] {
  return repos.filter((r) => text.includes(r + "/") || new RegExp(`${esc(r)}\\s+\``).test(text));
}

// groundPaths are the paths a text may name on repos' default branches,
// by repo (demo/main.go, demo/internal/, a path on disk in its ground, or
// demo `main.go`): what's to be checked of each to tell whether it names
// a file or a folder there. A sentence's full stop after one is checked
// without it too.
export function groundPaths(text: string, known: Known): Map<string, Set<string>> {
  const out = new Map<string, Set<string>>();
  const pat = patternsOf(known);
  const repoOf = (name: string) => (known.repos.has(name) ? name : (known.aliases?.get(name) ?? name));
  const note = (repo: string, path: string) => {
    if (!known.repos.has(repo)) return;
    const paths = out.get(repo) ?? new Set<string>();
    out.set(repo, paths);
    for (const p of [path, path.replace(/\.+$/, "")]) if (p) paths.add(p);
  };
  if (!pat.ground) return out;
  for (const m of text.matchAll(pat.ground)) note(repoOf(m[1]), m[2]);
  for (const m of text.matchAll(pat.onDisk!)) if (m[2] === GROUND) note(m[1], m[3]);
  const names = alt([...known.repos.keys(), ...(known.aliases?.keys() ?? [])]);
  for (const m of text.matchAll(new RegExp(`(?<![\\w./@#:~-])(${names})\\s+\`([^\`\\n]+)\``, "g"))) {
    note(repoOf(m[1]), m[2].trim().replace(/^\/+/, "").replace(/(?::\d+(?:-\d+)?)$/, "").replace(/\/+$/, ""));
  }
  return out;
}

// refHref is where a ref goes: a file in Files at its lines (a space's
// document, as a whole or at a heading, in Documents: known says which
// repos are spaces), a pull request's review, a mission, a page, a
// conversation, an artifact (at a version). A plugin's thing goes to where
// its kind opens one in the plugin's app; one whose kind opens nowhere
// goes to its address outside ws (isOutside), or to the plugin.
export function refHref(ref: PageRef, known?: Known): string {
  switch (ref.kind) {
    case "file": {
      const space = !ref.line && /\.(md|markdown|mdx)$/i.test(ref.path) && known?.spaces?.find((s) => s.repo === ref.repo && ref.path.startsWith(s.folder ?? ""));
      if (space) {
        const at = `/documents/${encodeURIComponent(ref.repo)}/${ref.path.split("/").map(encodeURIComponent).join("/")}`;
        return at + (ref.worktree === GROUND ? "" : `?in=${encodeURIComponent(ref.worktree)}`) + (ref.anchor ? `#${ref.anchor}` : "");
      }
      const tree = ref.worktree === GROUND ? "" : `/${encodeURIComponent(ref.worktree)}`;
      const lines = ref.line ? `#L${ref.line}${ref.endLine && ref.endLine > ref.line ? `-L${ref.endLine}` : ""}` : ref.anchor ? `#${ref.anchor}` : "";
      return `/files/${encodeURIComponent(ref.repo)}${tree}?${new URLSearchParams({ path: ref.path })}${lines}`;
    }
    case "dir": {
      const tree = ref.worktree === GROUND ? "" : `/${encodeURIComponent(ref.worktree)}`;
      return `/files/${encodeURIComponent(ref.repo)}${tree}?${new URLSearchParams({ path: ref.path })}`;
    }
    case "repo":
      return `/files/${encodeURIComponent(ref.repo)}`;
    case "pr":
      return `/reviews/${encodeURIComponent(ref.repo)}/${ref.number}`;
    case "mission":
      return `/missions/${encodeURIComponent(ref.slug)}`;
    case "page":
      return `/pages/${ref.id}`;
    case "talk":
      return `/conversations/${ref.id}`;
    case "artifact":
      return ref.version ? `/artifacts/${ref.id}/v${ref.version}` : `/artifacts/${ref.id}`;
    case "thing": {
      const path = namedOf(ref, known)?.path;
      if (path) return `/x/${encodeURIComponent(ref.plugin)}${path.replace("{ref}", encodeURIComponent(ref.id))}`;
      return ref.url ?? `/x/${encodeURIComponent(ref.plugin)}`;
    }
  }
}

// namedOf is the kind a plugin's thing is of, as its plugin registered it;
// undefined once the plugin's gone, or no longer has the kind.
export function namedOf(ref: Extract<PageRef, { kind: "thing" }>, known?: Known): Named | undefined {
  return known?.things?.find((t) => t.plugin === ref.plugin && t.what === ref.what);
}

// goneCapsule is whether a ref's capsule has gone since it was named
// (landed, or burned): what there is to name no longer has it.
export const goneCapsule = (r: { repo: string; worktree: string }, known?: Known) =>
  r.worktree !== GROUND && !!known?.capsules && !known.capsules.get(r.repo)?.includes(r.worktree);

// isOutside is whether somewhere a ref goes (refHref) is outside ws: an
// address, not one of the web UI's own paths.
export const isOutside = (href: string) => /^[a-z][a-z0-9+.-]*:/i.test(href);

// refKey names a ref, to find it again from where it's drawn.
export const refKey = (ref: PageRef) => JSON.stringify(ref);
export const refOf = (key: string): PageRef | undefined => {
  try {
    return JSON.parse(key) as PageRef;
  } catch {
    return undefined;
  }
};
