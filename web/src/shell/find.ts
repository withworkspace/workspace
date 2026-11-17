import type { ActVerb, Workspace } from "../api";
import { docAddress, docRef, pageAddress, spaceAt, type SpaceRef } from "../api";
import { GROUND, type PageRef, refHref } from "../refs/refs";
import { fuzzy } from "../ui/fuzzy";
import type { IconName } from "../ui/Icon";
import { crewStatus, headlineStatus, type StatusInfo } from "../ui/status";

// Find, apart from how it's drawn: what there is to find in what the page
// holds of the workspace, what the server's asked for the rest (files,
// pull requests, what's typed read as a ref), and how the two are put
// together into the groups the window shows.

// The groups, in the order they show. best, needs and recent aren't kinds
// of thing: the one that fits most, what's waiting, where you've been.
export const groupOrder = ["best", "needs", "recent", "missions", "pulls", "documents", "said", "files", "code", "pages", "talks", "artifacts", "crew", "capsules", "do", "places"] as const;
export type GroupKey = (typeof groupOrder)[number];

export const groupTitle: Record<GroupKey, string> = {
  best: "Best",
  needs: "Needs you",
  recent: "Recent",
  missions: "Missions",
  pulls: "Pull requests",
  documents: "Documents",
  said: "Said",
  files: "Files",
  code: "Code",
  pages: "Pages",
  talks: "Conversations",
  artifacts: "Artifacts",
  crew: "Crew",
  capsules: "Capsules",
  do: "Do",
  places: "Places",
};

// What an item does that isn't going somewhere.
export type Doing = { act: ActVerb; slug: string; crew?: string } | { theme: "light" | "dark" | "system" } | { help: true } | { page: true };

// PullFound is a pull request as the server found it: enough to say where
// it stands without asking GitHub.
export interface PullFound {
  repo: string;
  number: number;
  title: string;
  author?: string;
  branch?: string;
  state?: string;
  words?: string;
  checks?: string;
  draft?: boolean;
  mine?: boolean;
  stack?: string;
  asks?: boolean;
}

export interface Item {
  id: string; // the same for the same thing, however it was found
  group: GroupKey;
  kind: string; // what it is, in a word
  title: string;
  at?: number[]; // the letters of the title that matched
  hint?: string;
  icon?: IconName;
  status?: StatusInfo;
  score: number;
  to?: string; // where it goes
  doing?: Doing;
  ref?: string; // its name, to copy
  path?: boolean; // the title is a path: its folders are drawn quieter
  notes?: string[]; // said after it: changed in feature, merged
  tone?: string;
  card?: PageRef; // what its card shows, beside the list
  pull?: PullFound;
  said?: Found; // what ws records that says what's typed: its card shows its words, marked
  code?: CodeHit; // a line of a file, found by what it says: its text, the match marked
}

// What a headline asks of you that Find can do outright.
const doable: Partial<Record<string, ActVerb>> = {
  archive: "archive",
  accept: "accept",
  resume: "resume",
  "launch-queued": "launch-queued",
  unpark: "unpark",
  brief: "brief",
};

// localItems is everything the page already holds that can be found.
export function localItems(w: Workspace): Item[] {
  const place = (to: string, title: string, icon: IconName, hint?: string): Item => ({
    id: `place:${to}`,
    group: "places",
    kind: "Place",
    title,
    hint,
    icon,
    score: 0,
    to,
  });
  const out: Item[] = [
    place("/", "Missions", "flag"),
    place("/conversations", "Conversations", "message"),
    place("/documents", "Documents", "file", "pages spaces records adrs write cells"),
    place("/artifacts", "Artifacts", "artifact", "prototypes sketches html mock-ups frames versions"),
    place("/reviews", "Reviews", "pr", "pull requests prs stacks github"),
    place("/team", "Team", "user", "people timeline who landed merged what's happening"),
    place("/files", "Files", "layers", "repos capsules ground browse"),
    place("/orchestrator", "Orchestrator", "sparkle"),
    {
      id: "do:new-talk",
      group: "do",
      kind: "Do",
      title: "Start a conversation",
      hint: "new mission idea question",
      icon: "plus",
      score: 0,
      to: "/conversations?new=1",
    },
    {
      id: "do:new-page",
      group: "do",
      kind: "Do",
      title: "Start a page",
      hint: "new document write",
      icon: "plus",
      score: 0,
      doing: { page: true },
    },
  ];
  for (const m of w.missions) {
    out.push({
      id: `mission:${m.slug}`,
      group: "missions",
      kind: "Mission",
      title: m.title,
      hint: m.slug,
      status: headlineStatus(m.headline),
      score: 0,
      to: `/missions/${m.slug}`,
      ref: m.slug,
      card: { kind: "mission", slug: m.slug },
    });
    const verb = doable[m.headline.action.verb ?? ""];
    if (verb && m.headline.action.label) {
      const label = m.headline.action.label[0].toUpperCase() + m.headline.action.label.slice(1);
      out.push({
        id: `do:${verb}:${m.slug}`,
        group: "do",
        kind: "Do",
        title: `${label}: ${m.title}`,
        hint: m.headline.text,
        icon: "play",
        score: 0,
        doing: { act: verb, slug: m.slug, crew: m.headline.action.target },
      });
    }
    for (const c of m.capsules) {
      out.push({
        id: `capsule:${m.slug}/${c.repo}/${c.name}`,
        group: "capsules",
        kind: "Capsule",
        title: `${c.repo}/${c.name}`,
        hint: m.title,
        icon: "branch",
        score: 0,
        to: `/missions/${m.slug}/files/${c.repo}/${c.name}`,
        ref: `${c.repo}/${c.name}`,
      });
    }
    for (const c of m.crew) {
      out.push({
        id: `crew:${m.slug}/${c.id}`,
        group: "crew",
        kind: "Crew",
        title: `${c.id} on ${m.title}`,
        hint: [c.harness, c.capsule].filter(Boolean).join(" · "),
        status: crewStatus(c),
        score: 0,
        to: `/missions/${m.slug}/crew/${c.id}`,
      });
    }
  }
  for (const pg of w.pages ?? []) {
    out.push({
      id: `page:${pg.id}`,
      group: "pages",
      kind: "Page",
      title: pg.title,
      hint: pg.id,
      icon: "file",
      score: 0,
      to: pageAddress(pg),
      ref: pg.id,
      card: { kind: "page", id: pg.id },
    });
  }
  for (const t of w.conversations) {
    out.push({
      id: `talk:${t.id}`,
      group: "talks",
      kind: "Conversation",
      title: t.title || t.opening,
      hint: t.id,
      icon: "message",
      score: 0,
      to: `/conversations/${t.id}`,
      ref: t.id,
      card: { kind: "talk", id: t.id },
    });
  }
  // An artifact, by its title (a discarded one is off the list, and isn't
  // found here: ws find finds its notes).
  for (const a of w.artifacts ?? []) {
    if (a.status === "discarded") continue;
    out.push({
      id: `artifact:${a.id}`,
      group: "artifacts",
      kind: "Artifact",
      title: a.title,
      hint: [a.id, `v${a.latest?.n ?? a.versions}`, a.latest?.note].filter(Boolean).join(" · "),
      icon: "artifact",
      score: 0,
      to: `/artifacts/${a.id}`,
      ref: a.id,
      card: { kind: "artifact", id: a.id },
    });
  }
  out.push(
    {
      id: "do:theme-light",
      group: "do",
      kind: "Do",
      title: "Light theme",
      hint: "appearance",
      icon: "sun",
      score: 0,
      doing: { theme: "light" },
    },
    {
      id: "do:theme-dark",
      group: "do",
      kind: "Do",
      title: "Dark theme",
      hint: "appearance",
      icon: "moon",
      score: 0,
      doing: { theme: "dark" },
    },
    {
      id: "do:theme-system",
      group: "do",
      kind: "Do",
      title: "Theme from the system",
      hint: "appearance",
      icon: "monitor",
      score: 0,
      doing: { theme: "system" },
    },
    {
      id: "do:keys",
      group: "do",
      kind: "Do",
      title: "Show the keys",
      hint: "shortcuts help",
      icon: "keyboard",
      score: 0,
      doing: { help: true },
    },
  );
  return out;
}

// --- what the server finds ---

// Found is one thing the server found (find.Result).
export interface Found {
  kind: "file" | "dir" | "pr" | "capsule" | "document" | "said";
  ref: string;
  title: string;
  at?: number[];
  hint?: string;
  score: number;
  exact?: boolean;
  repo?: string;
  worktree?: string;
  path?: string;
  line?: number;
  endLine?: number;
  status?: string;
  in?: { capsule: string; status: string }[];
  number?: number;
  author?: string;
  branch?: string;
  state?: string;
  words?: string;
  checks?: string;
  draft?: boolean;
  mine?: boolean;
  stack?: string;
  asks?: boolean;
  anchor?: string; // a document's: the section that says it best; said, where in the thing (a turn, a section)
  // What ws records, found by what it says (web.SaidFound): where it's
  // read (a mission's slug, a pull request, none for the workspace's),
  // its kind and the kind of place it was found at, the two in words, its
  // words with those found marked, a comment's file, what it's of (a
  // review's capsule or pull request, a cell's or a pass's page), and
  // whether its mission's archived.
  scope?: string;
  thing?: string;
  place?: string;
  what?: string;
  where?: string;
  snippet?: { text: string; mark?: boolean }[];
  on?: string;
  of?: string;
  ended?: boolean;
}

// FoundGroup is a kind's results (find.Group): "ref" (what's typed, read
// as a name or an address), "pulls", "capsules" (by their names), "files",
// and by what they say, "documents" (the spaces') and "said" (what ws
// records).
export interface FoundGroup {
  kind: "ref" | "pulls" | "capsules" | "files" | "documents" | "said";
  results: Found[];
  more?: number;
  reading?: number; // capsules not read yet
  at?: string; // when the pull requests were read from GitHub
  problem?: string;
}

// findStream asks the server, and gives each kind's group to on as it
// comes. Aborting the signal drops the request, and the server stops.
export async function findStream(q: string, kinds: string[] | undefined, limit: number, signal: AbortSignal, on: (g: FoundGroup) => void): Promise<void> {
  const params = new URLSearchParams({ q, limit: String(limit) });
  if (kinds) params.set("kinds", kinds.join(","));
  const res = await fetch(`/api/find?${params}`, {
    signal,
    headers: { accept: "application/x-ndjson" },
  });
  if (!res.ok || !res.body) throw new Error(`${res.status} ${res.statusText}`);
  const reader = res.body.getReader();
  const text = new TextDecoder();
  let rest = "";
  for (;;) {
    const { value, done } = await reader.read();
    if (done) break;
    rest += text.decode(value, { stream: true });
    let nl: number;
    while ((nl = rest.indexOf("\n")) >= 0) {
      const line = rest.slice(0, nl).trim();
      rest = rest.slice(nl + 1);
      if (!line) continue;
      const data = JSON.parse(line) as FoundGroup | { done?: boolean };
      if ("kind" in data) on(data);
    }
  }
}

// warmFind has the server read whatever of its index is due, so the first
// letter typed is answered from memory.
export function warmFind(): void {
  fetch("/api/find", { headers: { accept: "application/x-ndjson" } }).then(
    (r) => r.text(),
    () => undefined,
  );
}

const pullTone = (state?: string) => (state && ["needs-review", "back-with-you", "returned", "ready"].includes(state) ? "attention" : "quiet");

// foundItem is a server's result as an item of the list.
export function foundItem(f: Found, spaces?: SpaceRef[]): Item {
  if (f.kind === "document") {
    // A space's document, found by what it says: led to the section.
    return {
      id: `document:${f.ref}`,
      group: "documents",
      kind: "Document",
      title: f.title,
      hint: f.hint,
      icon: "book",
      score: f.score,
      to: docAddress(f.ref) + (f.anchor ? `#${f.anchor}` : ""),
      ref: f.ref,
    };
  }
  if (f.kind === "said") return saidItem(f);
  if (f.kind === "pr") {
    const number = f.number ?? 0;
    const repo = f.repo ?? "";
    const notes = [...new Set([f.draft ? "draft" : "", f.words ?? "", f.checks === "failure" ? "checks fail" : f.checks === "pending" ? "checks running" : ""].filter(Boolean))];
    return {
      id: `pr:${f.ref}`,
      group: "pulls",
      kind: "Pull request",
      title: f.title,
      at: f.at,
      hint: f.asks ? f.hint : [f.ref, f.author, f.stack].filter(Boolean).join(" · "),
      icon: "pr",
      score: f.score,
      to: `/reviews/${encodeURIComponent(repo)}/${number}`,
      ref: f.ref,
      notes,
      tone: pullTone(f.state),
      pull: {
        repo,
        number,
        title: f.title,
        author: f.author,
        branch: f.branch,
        state: f.state,
        words: f.words,
        checks: f.checks,
        draft: f.draft,
        mine: f.mine,
        stack: f.stack,
        asks: f.asks,
      },
    };
  }
  const repo = f.repo ?? "";
  const worktree = f.worktree || GROUND;
  if (f.kind === "capsule") {
    return { id: `capsule:${f.ref}`, group: "capsules", kind: "Capsule", title: f.title, at: f.at, hint: f.hint, icon: "branch", score: f.score, to: `/files/${encodeURIComponent(repo)}/${encodeURIComponent(worktree)}`, ref: f.ref };
  }
  const path = f.path ?? "";
  const ref: PageRef = {
    kind: "file",
    repo,
    worktree,
    path,
    ...(f.line ? { line: f.line, ...(f.endLine ? { endLine: f.endLine } : {}) } : {}),
  };
  const notes = [
    ...(f.line ? [`line${f.endLine ? `s ${f.line} to ${f.endLine}` : ` ${f.line}`}`] : []),
    ...(worktree !== GROUND && f.status ? [`${f.status} in ${worktree}`] : []),
    ...(f.in ?? []).map((c) => `${c.status === "removed" ? "removed" : "changed"} in ${c.capsule}`),
  ];
  if (f.kind === "dir") {
    const tree = worktree === GROUND ? "" : `/${encodeURIComponent(worktree)}`;
    return {
      id: `dir:${f.ref}`,
      group: "files",
      kind: "Folder",
      title: f.title,
      at: f.at,
      icon: "layers",
      score: f.score,
      to: `/files/${encodeURIComponent(repo)}${tree}`,
      ref: f.ref,
      path: true,
      notes,
    };
  }
  return {
    id: `file:${f.title}`,
    group: "files",
    kind: "File",
    title: f.title,
    at: f.at,
    icon: "file",
    score: f.score,
    to: !f.line && spaceAt(spaces, repo, path) ? docAddress(docRef(repo, worktree, path)) : refHref(ref),
    ref: f.ref,
    path: true,
    notes,
    card: ref,
  };
}

// saidKind is what a thing found by what it says is, in a word.
function saidKind(f: Found): string {
  switch (f.place) {
    case "brief":
      return f.anchor ? "Brief" : "Mission";
    case "handoff":
      return "Handoff";
    case "file":
      return "File";
    case "finding":
      return "Finding";
  }
  switch (f.thing) {
    case "decision":
      return "Decision";
    case "comment":
      return "Comment";
    case "review":
      return "Review";
    case "talk":
    case "pr":
      return "Talk";
    case "pass":
    case "review-pass":
      return "Pass";
    case "cell":
      return "Cell";
    case "mission":
      return "Mission";
    case "pull-request":
      return "Pull request";
    case "page":
      return "Page";
    case "artifact":
      return "Artifact";
  }
  return f.thing?.startsWith("thread") ? "Thread" : "Conversation";
}

const saidIcon: Record<string, IconName> = {
  Mission: "flag",
  Brief: "flag",
  Decision: "gavel",
  Comment: "pencil",
  Review: "eye",
  Handoff: "file",
  File: "file",
  Finding: "sparkle",
  Pass: "sparkle",
  Cell: "file",
  "Pull request": "pr",
  Page: "file",
  Artifact: "artifact",
};

// saidTo is the page of a thing found by what it says: its mission (the
// brief, for a section of it), a pull request's (one kept, or the review
// of one), the review of its capsule, a handoff's or a file's page, the
// document a thread is on, the conversation, or the page a cell or a pass
// is of.
function saidTo(f: Found): string {
  const scope = f.scope ?? "";
  const slug = encodeURIComponent(scope);
  const pr = /^([\w.-]+)#(\d+)$/.exec(scope) ?? (f.of ? /^([\w.-]+)#(\d+)$/.exec(f.of) : null);
  if (pr) return `/reviews/${encodeURIComponent(pr[1])}/${pr[2]}`;
  switch (f.place) {
    case "brief":
      return f.anchor ? `/missions/${slug}?tab=brief` : `/missions/${slug}`;
    case "handoff":
    case "file":
      return `/missions/${slug}/artifact?${new URLSearchParams({ ref: f.ref })}`;
  }
  switch (f.thing) {
    case "decision":
      return `/missions/${slug}`;
    case "comment":
    case "review": {
      const [repo, name] = (f.of ?? "").split("/");
      return repo && name ? `/missions/${slug}/review/${encodeURIComponent(repo)}/${encodeURIComponent(name)}` : `/missions/${slug}?tab=work`;
    }
  }
  // An artifact, at the version whose note says it.
  if (f.thing === "artifact") return `/artifacts/${encodeURIComponent(f.ref)}${f.anchor && /^v\d+$/.test(f.anchor) ? `/${f.anchor}` : ""}`;
  if (f.of && /^p\d+$/.test(f.of)) return `/pages/${encodeURIComponent(f.of)}`;
  // A thread on a document is read beside it.
  if (scope && f.of && f.thing?.startsWith("thread")) return `/missions/${slug}/artifact?${new URLSearchParams({ ref: f.of })}`;
  return scope ? `/missions/${slug}/conversations/${encodeURIComponent(f.ref)}` : `/conversations/${encodeURIComponent(f.ref)}`;
}

// saidItem is what ws records, found by what it says, as a row: a line
// like a document's, its words dim after its title, where it's read after
// them.
function saidItem(f: Found): Item {
  const kind = saidKind(f);
  const scope = f.scope ?? "";
  const notes = [...(scope ? [scope] : []), ...(f.ended ? ["archived"] : [])];
  return {
    id: `said:${scope}/${f.ref}`,
    group: "said",
    kind,
    title: f.title,
    hint: f.hint,
    icon: saidIcon[kind] ?? "message",
    score: f.score,
    to: saidTo(f),
    ref: f.ref,
    notes,
    said: f,
  };
}

// --- what the files say (code search) ---

// CodeHit is a line a code search matched (ops.CodeHit): where, and the
// line's text (a window of it, when cut), with where the patterns matched
// in it, as string indexes.
export interface CodeHit {
  repo: string;
  worktree: string;
  path: string;
  line: number;
  column: number;
  text: string;
  cut?: boolean;
  ranges: [number, number][];
}

// CodeEnd is how a code search ended: how many hits, and what capped it
// (a cap on matches, or on bytes), or what went wrong part way.
export interface CodeEnd {
  matches: number;
  bytes: number;
  capped?: "matches" | "bytes";
  problem?: string;
}

// codeStream asks the server to search the files for each of patterns,
// across the workspace or in repo (and in the whole of one of its
// worktrees, given it), and gives each hit to on as it comes. Aborting the
// signal drops the request, which stops ripgrep.
export async function codeStream(patterns: string[], where: { repo?: string; worktree?: string }, signal: AbortSignal, on: (h: CodeHit) => void): Promise<CodeEnd> {
  const params = new URLSearchParams();
  for (const p of patterns) params.append("p", p);
  if (where.repo) params.set("repo", where.repo);
  if (where.worktree) params.set("worktree", where.worktree);
  const res = await fetch(`/api/search?${params}`, { signal, headers: { accept: "application/x-ndjson" } });
  if (!res.ok || !res.body) {
    const said = await res.json().catch(() => undefined);
    throw new Error((said as { error?: string } | undefined)?.error ?? `${res.status} ${res.statusText}`);
  }
  const reader = res.body.getReader();
  const text = new TextDecoder();
  let rest = "";
  let end: CodeEnd = { matches: 0, bytes: 0 };
  for (;;) {
    const { value, done } = await reader.read();
    if (done) break;
    rest += text.decode(value, { stream: true });
    let nl: number;
    while ((nl = rest.indexOf("\n")) >= 0) {
      const line = rest.slice(0, nl).trim();
      rest = rest.slice(nl + 1);
      if (!line) continue;
      const data = JSON.parse(line) as CodeHit | (CodeEnd & { done: true });
      if ("done" in data) end = data;
      else on(data);
    }
  }
  return end;
}

// codeItem is a hit as a row: it goes to the file at its line.
export function codeItem(h: CodeHit, i: number): Item {
  const ref: PageRef = { kind: "file", repo: h.repo, worktree: h.worktree || GROUND, path: h.path, line: h.line };
  const where = `${h.repo}/${h.path}:${h.line}`;
  return {
    id: `code:${h.repo}/${h.worktree}:${h.path}:${h.line}:${i}`,
    group: "code",
    kind: "Code",
    title: h.text,
    hint: where,
    icon: "file",
    score: 0,
    to: refHref(ref),
    ref: h.worktree === GROUND ? `${h.repo}/${h.path}:${h.line}` : `${h.repo}/${h.worktree}:${h.path}:${h.line}`,
    notes: h.worktree === GROUND ? [] : [`in ${h.worktree}`],
    code: h,
  };
}

// codeNote says what capped a code search, or what went wrong part way.
export function codeNote(end: CodeEnd | undefined): string | undefined {
  if (!end) return undefined;
  if (end.problem) return end.problem;
  if (end.capped === "matches") return `Stopped at the first ${end.matches} matches. There are more: say more of what you're after.`;
  if (end.capped === "bytes") return `Stopped at ${end.matches} matches, as many as fit. There are more: say more of what you're after.`;
  return undefined;
}

// codeLead is how much of a line shows before its first match, at most,
// so the match is in view: a row holds about 80 characters of code on a
// wide window, 34 on a phone's (codeLeadNarrow). What's before is left
// off, and a "…" says so.
export const codeLead = 40;
export const codeLeadNarrow = 12;

// codeRuns is a line's text as a row shows it: in runs, those the patterns
// matched marked, its indent dropped, and starting no more than lead
// characters before the first match (clipped: it starts later than the
// line does).
export function codeRuns(text: string, ranges: [number, number][], lead = codeLead): { runs: { text: string; hit: boolean }[]; clipped: boolean } {
  const sorted = [...ranges].sort((a, b) => a[0] - b[0]);
  const indent = text.length - text.trimStart().length;
  let start = indent;
  const first = sorted[0]?.[0];
  if (first !== undefined && first - lead > start) {
    start = first - lead;
    // Not half a character, nor a word's tail if a space is near.
    if (/[\uDC00-\uDFFF]/.test(text[start] ?? "")) start--;
    const space = text.indexOf(" ", start);
    if (space >= 0 && space < first && space - start < lead / 2) start = space + 1;
    while (start < first && /\s/.test(text[start])) start++;
  }
  const out: { text: string; hit: boolean }[] = [];
  let at = start;
  for (const [from, to] of sorted) {
    const s = Math.max(from, at);
    if (to <= s) continue;
    if (s > at) out.push({ text: text.slice(at, s), hit: false });
    out.push({ text: text.slice(s, to), hit: true });
    at = to;
  }
  if (at < text.length) out.push({ text: text.slice(at), hit: false });
  return { runs: out, clipped: start > indent };
}

// codeRematch is the hits found a moment ago that still say q, its
// matches found again: shown until the answer for q comes. Case matters
// when q has a capital, as the server's smart case has it.
export function codeRematch(hits: CodeHit[], q: string): CodeHit[] {
  if (!q) return [];
  const sensitive = q !== q.toLowerCase();
  const want = sensitive ? q : q.toLowerCase();
  const out: CodeHit[] = [];
  for (const h of hits) {
    const text = sensitive ? h.text : h.text.toLowerCase();
    const ranges: [number, number][] = [];
    for (let i = text.indexOf(want); i >= 0; i = text.indexOf(want, i + want.length)) ranges.push([i, i + want.length]);
    if (ranges.length) out.push({ ...h, ranges });
  }
  return out;
}

// repoOfPage is the repo the page at path is of, for code search's tag: a
// repo's files or a file in them, a capsule's files or its review, or a
// document in a space of a repo. Elsewhere, none.
export function repoOfPage(path: string, spaces?: SpaceRef[]): string | undefined {
  const seg = path.split("/").map((p) => {
    try {
      return decodeURIComponent(p);
    } catch {
      return p;
    }
  });
  if (seg[1] === "files" && seg[2]) return seg[2];
  if (seg[1] === "missions" && (seg[3] === "files" || seg[3] === "review") && seg[4]) return seg[4];
  if (seg[1] === "documents" && seg[2] && seg[3] && seg[2] !== "local" && spaces?.some((s) => s.repo === seg[2])) return seg[2];
  return undefined;
}

// --- what's typed ---

// A first character says what you're after: > things to do, # pull
// requests, / files, % what the files say.
export type Scope = "all" | "do" | "pulls" | "files" | "code";
export const scopePrefix: Record<Exclude<Scope, "all">, string> = {
  do: ">",
  pulls: "#",
  files: "/",
  code: "%",
};

// parseQuery reads what's typed: its scope, and the rest. A path on disk
// under the workspace's root starts with / and isn't asking for files
// only: it's a ref, to be read whole.
export function parseQuery(raw: string, root?: string): { scope: Scope; q: string } {
  const s = raw.trimStart();
  if (s.startsWith(">")) return { scope: "do", q: s.slice(1).trim() };
  if (s.startsWith("#")) return { scope: "pulls", q: s.slice(1).trim() };
  if (s.startsWith("%")) return { scope: "code", q: s.slice(1).trim() };
  if (s.startsWith("/") && !(root && s.startsWith(`${root}/`))) return { scope: "files", q: s.slice(1).trim() };
  return { scope: "all", q: s.trim() };
}

// withScope is what's typed, asking for scope instead.
export function withScope(raw: string, scope: Scope, root?: string): string {
  const { q } = parseQuery(raw, root);
  return scope === "all" ? q : scopePrefix[scope] + q;
}

// asks is what to ask the server for: the kinds (undefined: all of them),
// and how many of each. Nothing for things to do, which the page holds.
export function asks(scope: Scope, only: GroupKey | undefined, q: string): { kinds?: string[]; limit: number } | undefined {
  const group = scope === "all" ? only : scope;
  if (group === "code") return undefined; // code search asks for itself (codeStream)
  if (group === "pulls") return { kinds: ["pulls"], limit: wide };
  if (group === "files") return { kinds: ["ref", "files"], limit: wide };
  if (group === "documents" || group === "said") return q ? { kinds: [group], limit: wide } : undefined; // found by what they say
  if (group) return undefined; // a kind the page holds
  if (!q) return undefined; // nothing typed: what needs you, where you've been
  return { limit: few };
}

// How many of a group show among the others, and when it's the only one.
export const few = 5;
export const wide = 60;

// --- putting it together ---

// How much a kind counts for, when kinds are weighed against each other
// for the best line: things with names over files.
const weight: Partial<Record<GroupKey, number>> = {
  missions: 4,
  pulls: 2,
  documents: 1,
  said: 1,
  pages: 2,
  talks: 2,
  places: 1,
  files: -6,
};

// hintScore is how well what's typed matches a hint (a slug, an id, a few
// words something might be looked for by): each term has to start a word
// of it. Not letter by letter, as a title is matched: "main" isn't in
// "new mission idea question".
function hintScore(hint: string, q: string): number | undefined {
  const lower = hint.toLowerCase();
  let score = 0;
  for (const term of q.toLowerCase().split(/\s+/).filter(Boolean)) {
    let at = lower.indexOf(term);
    while (at > 0 && /[a-z0-9]/.test(lower[at - 1])) at = lower.indexOf(term, at + 1);
    if (at < 0) return undefined;
    score += 8 + term.length * 3;
  }
  return score;
}

// rank scores items against what's typed, dropping those that don't
// match. A hint matches too, for less.
export function rank(items: Item[], q: string): Item[] {
  if (!q) return items.map((it) => ({ ...it, score: 0, at: undefined }));
  const out: Item[] = [];
  for (const it of items) {
    const title = fuzzy(it.title, q);
    const hint = it.hint ? hintScore(it.hint, q) : undefined;
    if (!title && hint === undefined) continue;
    if (title && (hint === undefined || title.score >= hint)) out.push({ ...it, score: title.score, at: title.at });
    else out.push({ ...it, score: hint!, at: undefined });
  }
  return out;
}

// Memory is what Find remembers of you: where you've been, the latest
// first, and what you chose for what you typed.
export interface Memory {
  visits: { id: string }[];
  chosen: Record<string, string>;
}

// boost is how much more an item counts for being somewhere you go.
function boost(it: Item, q: string, memory?: Memory): number {
  if (!memory) return 0;
  let b = 0;
  const i = memory.visits.findIndex((v) => v.id === it.id);
  if (i >= 0 && i < 20) b += 6 - i * 0.3;
  if (memory.chosen[q.toLowerCase()] === it.id) b += 40;
  return b;
}

export interface Section {
  group: GroupKey;
  title: string;
  items: Item[];
  more: number; // matched, and not shown
  note?: string;
}

export interface Built {
  sections: Section[];
  rows: Item[]; // every item shown, in order: what the cursor moves through
}

export interface Assembly {
  q: string;
  scope: Scope;
  only?: GroupKey; // narrowed to a group with ⇥
  local: Item[]; // what the page holds
  found: FoundGroup[]; // what the server's answered, for this q
  needs?: Item[]; // what's waiting for you
  recent?: Item[]; // where you've been
  memory?: Memory;
  spaces?: SpaceRef[]; // the workspace's spaces: a document in one opens in Documents
  code?: { hits: CodeHit[]; end?: CodeEnd }; // what code search found, in the code scope
}

// build is the window's sections for what's typed.
export function build(a: Assembly): Built {
  const group = a.scope === "all" ? a.only : a.scope;
  const sections: Section[] = [];
  const add = (g: GroupKey, items: Item[], more = 0, note?: string, title = groupTitle[g]) => {
    if (items.length || note) sections.push({ group: g, title, items, more, note });
  };

  // Nothing typed, and not narrowed: what needs you, where you've been,
  // and the places.
  if (!a.q && !group) {
    const needs = a.needs ?? [];
    const seen = new Set(needs.map((n) => n.id));
    add("needs", needs.slice(0, few));
    add("recent", (a.recent ?? []).filter((r) => !seen.has(r.id)).slice(0, 8));
    add(
      "places",
      a.local.filter((it) => it.group === "places"),
    );
    return { sections, rows: sections.flatMap((s) => s.items) };
  }

  // What the files say: only in its own scope, never among the rest,
  // where its lines would drown everything else. In the order found.
  if (group === "code") {
    if (a.q) add("code", (a.code?.hits ?? []).map(codeItem), 0, codeNote(a.code?.end));
    return { sections, rows: sections.flatMap((s) => s.items) };
  }

  const byGroup = new Map<GroupKey, { items: Item[]; more: number; note?: string }>();
  const put = (g: GroupKey) => {
    let got = byGroup.get(g);
    if (!got) byGroup.set(g, (got = { items: [], more: 0 }));
    return got;
  };
  for (const it of rank(a.local, a.q))
    put(it.group).items.push({
      ...it,
      score: it.score + boost(it, a.q, a.memory),
    });
  let exact: Item | undefined;
  for (const g of a.found) {
    if (g.kind === "ref") {
      if (g.results[0]) exact = foundItem(g.results[0], a.spaces);
      continue;
    }
    const into = put(g.kind);
    for (const f of g.results) {
      const it = foundItem(f, a.spaces);
      // A mission's capsule is already here, from what the page holds.
      if (g.kind === "capsules" && into.items.some((x) => x.ref === it.ref)) continue;
      into.items.push({ ...it, score: it.score + boost(it, a.q, a.memory) });
    }
    into.more += g.more ?? 0;
    if (g.problem) into.note = g.problem;
    else if (g.reading) into.note = `Reading ${g.reading} ${g.reading === 1 ? "capsule" : "capsules"}: their files may be missing.`;
  }
  // A pull request, a page or an artifact listed already (an open pull
  // request, or a page or an artifact whose title has what's typed) isn't
  // said again: Said has those found only by what they say.
  const listed = new Set([...(byGroup.get("pulls")?.items ?? []), ...(byGroup.get("pages")?.items ?? []), ...(byGroup.get("artifacts")?.items ?? [])].map((it) => it.ref));
  const said = byGroup.get("said");
  const kept = new Set(["pull-request", "page", "artifact"]);
  if (said) said.items = said.items.filter((it) => !kept.has(it.said?.thing ?? "") || !listed.has(it.ref));
  for (const got of byGroup.values()) got.items.sort((x, y) => y.score - x.score);
  // A pull request that isn't among those in hand is offered after those
  // that are: demo#1 is on its way to demo#12 more often than it's meant.
  if (exact?.pull?.asks && byGroup.get("pulls")?.items.length) {
    put("pulls").items.push({ ...exact, score: -1 });
    exact = undefined;
  }

  // The best line: what's typed naming one thing outright, or the highest
  // of any kind. Only when everything's being looked through.
  let best: Item | undefined;
  if (!group && a.q) {
    const lower = a.q.toLowerCase();
    best = exact ?? a.local.find((it) => it.ref?.toLowerCase() === lower && ["missions", "pages", "talks"].includes(it.group));
    if (!best) {
      let top = -Infinity;
      for (const [g, got] of byGroup) {
        const it = got.items[0];
        if (it && it.score + (weight[g] ?? 0) > top) {
          top = it.score + (weight[g] ?? 0);
          best = it;
        }
      }
    }
    if (best)
      sections.push({
        group: "best",
        title: groupTitle.best,
        items: [{ ...best, at: best === exact ? undefined : best.at }],
        more: 0,
      });
  } else if (exact && group === "files") {
    best = exact;
    sections.push({
      group: "best",
      title: groupTitle.best,
      items: [exact],
      more: 0,
    });
  }

  for (const g of groupOrder) {
    if (group && g !== group) continue;
    const got = byGroup.get(g);
    if (!got) continue;
    const items = got.items.filter((it) => it.id !== best?.id);
    const room = group ? wide : few;
    add(g, items.slice(0, room), got.more + Math.max(0, items.length - room), got.note, a.q || g !== "files" ? groupTitle[g] : "Changed in capsules");
  }
  return { sections, rows: sections.flatMap((s) => s.items) };
}

// rematch is what the server answered for what was typed a moment ago,
// kept to what still matches now: shown until the answer for this comes,
// so the list doesn't empty between letters. The page's matcher is the
// server's, so what's kept is what it would answer with.
export function rematch(found: FoundGroup[], q: string): FoundGroup[] {
  const out: FoundGroup[] = [];
  for (const g of found) {
    if (g.kind === "ref") continue; // what was typed then isn't what's typed now
    const results: Found[] = [];
    for (const f of g.results) {
      const m = fuzzy(f.title, q.replace(/:\d+(-\d+)?$/, ""));
      if (m) results.push({ ...f, at: m.at, score: m.score, exact: false });
    }
    out.push({ ...g, results, more: 0 });
  }
  return out;
}
