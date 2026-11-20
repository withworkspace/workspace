import type { Pinned, ThingAction, ThingField, ThingViews } from "./refs/refs";
import { useEffect, useRef, useState } from "react";
import type { Place } from "./markdown/links";
import { bursts } from "./load";

// Mirrors internal/web/api.go.

export interface Workspace {
  name: string;
  root: string;
  org?: string; // where its repos are on GitHub, for a ref's address
  defaultBranch?: string;
  missions: Mission[];
  // focus: what you've pinned that stands, in your order: the sidebar's
  // Focus (shell/focus.tsx). Absent from a daemon from before it.
  focus?: FocusPin[];
  spaces?: SpaceRef[]; // its spaces of documents, as config says them
  conversations: Talk[]; // the workspace's conversations, newest first
  pages: PageInfo[]; // the workspace's pages, the latest changed first
  artifacts?: ArtifactInfo[]; // its artifacts, the latest version's newest first, discarded ones too
  artifactsRepo?: string; // the repo a promoted artifact lives in, offered the first time one's promoted
  kinds?: DocKind[]; // its kinds of document: a page can be started as one, and filed as one
  docs?: string; // when the threads on documents outside a mission last changed
  // reviews: how many pull requests need you, as GitHub last said; absent
  // until the Reviews page has been opened, or where there's no GitHub.
  reviews?: { count: number; at: string };
  // The ws that serves it (its daemon's), as ws --version says it: what
  // the menu says, and what a plugin's app is told when it connects.
  version?: string;
  // places: where the sidebar's places go, as [ui] sidebar and menu say
  // (shell/places.ts); absent for as they come.
  places?: { sidebar: string[]; menu: string[] };
}

export interface Mission {
  slug: string;
  title: string;
  state: MissionState;
  created: string;
  updated: string;
  flag?: string;
  reReview?: string[]; // capsules waiting for you to re-review
  headline: Headline; // where it stands: ws mission show says the same
  capsules: Capsule[];
  crew: Crew[];
  at?: string; // where it is on its route, in words: "Step 2 of 7, yours: …"
  // about is what it's about, by name: a plugin's thing (RAM-123), a pull
  // request, anything ws names.
  about?: string[];
}

// Headline is where a mission stands: whose court it's in (an agent's,
// yours, or outside: GitHub), in a sentence, and what moves it on.
export interface Headline {
  court: "agent" | "you" | "outside" | "";
  kind: string;
  group: HeadlineGroup;
  label: string; // a few words, with a glyph: "? asks"
  text: string;
  action: HeadlineAction;
  by?: string;
  since?: string;
  checking?: Checking; // kind checking: what's running, so the page can tick
}

// Checking is what ws is running while it checks a step: a check that's a
// command (n of of), or the judge (n absent), reading the checks judged.
export interface Checking {
  step: number;
  n?: number;
  of: number;
  command?: string;
  line?: string; // the last line it's printed
  judged?: number[];
  judge?: string; // who's reading, on the judge's turn
  since: string;
  until: string; // when it's stopped, if it hasn't finished
}

export type HeadlineGroup = "needs-you" | "in-progress" | "waiting" | "up-next" | "past";

// HeadlineAction is what moves a headline on; verb "" (absent) is nothing
// to do.
export interface HeadlineAction {
  verb?: string; // reply, permission, watch, answer, review, review-doc, review-step, launch, launch-queued, resume, accept, wait, open, merge, archive, brief, confirm, unpark, log
  label: string;
  target?: string; // crew id, capsule, a document's ref (review-doc: its threads), PR, or a task (answer, review-step)
  url?: string;
  options?: string[];
  look?: Look[]; // review-step: what the Review step looks at; answer: the bundles that show a step's checks, when they're yours to say
}

// Look is one thing a step of yours looks at: a bundle to approve, a
// capsule's work to review (repo/name), or a document to read (an
// artifact's ref). A look that's read is beside the review (crew's notes,
// their handoffs): the plan doesn't name it, so approving needn't cover it.
export interface Look {
  bundle?: string; // a verification bundle (b3), on the mission's Verify tab
  capsule?: string;
  doc?: string;
  name: string;
  read?: boolean;
  approved?: boolean; // a bundle you've approved already: there to look at again, not what the step waits on
}

export type MissionState = "briefing" | "draft" | "active" | "review" | "landed" | "answered" | "handed-on" | "dropped" | "parked" | "archived";

// Ending is how a mission ends: landed (at Z, its code merged or landed),
// answered (a finding, no code), handed on (its work raised as other
// missions) or dropped.
export type Ending = "landed" | "answered" | "handed-on" | "dropped";

// Conclusion is how a mission ends, suggested or made: its ending (none
// for one from before endings, which only says it looks finished), what
// it came to, each part of Z met or not and how, and where it was handed
// on to. Mirrors internal/web/api.go.
export interface Conclusion {
  ending?: Ending;
  text: string;
  z?: { z: string; met: boolean; how?: string }[];
  to?: string[];
  by?: string;
  at?: string;
}

// EndedCapsule is a capsule that ended alone, before its mission: handed
// on to another mission, which took it, or dropped.
export interface EndedCapsule {
  repo: string;
  name: string;
  ending: "handed-on" | "dropped";
  to?: string;
  why?: string;
  by?: string;
  at: string;
}

export interface Capsule {
  repo: string;
  name: string;
  branch?: string;
  path: string;
  exists: boolean;
  head?: string;
}

export interface Crew {
  id: string;
  harness: string;
  model?: string; // the one it was launched on, when it was picked
  role?: string;
  capsule?: string;
  started: string;
  ended: boolean;
  starting?: boolean; // ws is starting its session, which has reported nothing yet: the mark the mission's headline reads
  activity?: "working" | "waiting" | "permission" | "question";
  status?: string;
  lastReply?: string;
  notice?: string; // what its harness last told you, or the question it asked
  options?: string[]; // that question's answers
  structured?: boolean; // crew over ACP: no terminal, just its conversation
  waitingOn?: string; // the work it ended its turn to wait on, while ws holds that turn
  again?: boolean; // it reported done, and it's been at work since (sent back to it): its done isn't its latest word
}

export class NotSignedIn extends Error {}

// ApiError is a request the server refused or couldn't do: its status, the
// code it gave a refusal (capsule-busy, changed-since, nothing-asked…:
// what a page branches on), and its sentence, to show.
export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string | undefined,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

// request is how the API's asked, every time: JSON in and out, NotSignedIn
// for a 401, and an ApiError for anything else that isn't OK.
async function request<T>(method: string, path: string, opts: { body?: unknown; keepalive?: boolean } = {}): Promise<T> {
  const write = method !== "GET";
  const res = await fetch(path, {
    method,
    headers: write ? { "content-type": "application/json", accept: "application/json" } : { accept: "application/json" },
    body: write ? JSON.stringify(opts.body ?? {}) : undefined,
    keepalive: opts.keepalive,
  });
  if (res.status === 401) throw new NotSignedIn();
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new ApiError(res.status, data.code, data.error ?? `${res.status} ${res.statusText}`);
  return data as T;
}

export const getJSON = <T>(path: string) => request<T>("GET", path);

export type Connection = "connecting" | "live" | "lost";

export interface WorkspaceState {
  workspace?: Workspace;
  error?: Error;
  connection: Connection;
}

// useWorkspace keeps the workspace current: it fetches when the event stream
// opens (so nothing that changed while it was closed is missed) and again
// whenever the server says it changed. It never polls.
// useVisible is whether the page is showing. Streams are held only while
// it is: a browser allows six connections to one host, and streams held
// open by every tab of the web UI used them up, leaving new requests (a
// page's data) waiting for one that never came free.
export function useVisible(): boolean {
  const [visible, setVisible] = useState(() => document.visibilityState !== "hidden");
  useEffect(() => {
    const changed = () => setVisible(document.visibilityState !== "hidden");
    document.addEventListener("visibilitychange", changed);
    return () => document.removeEventListener("visibilitychange", changed);
  }, []);
  return visible;
}

// A topic of the server's events: what a page follows to know when what it
// shows has changed (internal/web/events.go).
export type Topic = "workspace" | "reviews" | "team" | "plugins" | `mission:${string}`;

// EventHandlers are what a page does as it follows a topic: when the
// stream's open (hello: fetch what it shows, so nothing's missed), on each
// change, and when the stream's lost (closed: it won't come back by
// itself, as when it was refused).
export interface EventHandlers {
  hello?: () => void;
  changed: () => void;
  lost?: (closed: boolean) => void;
}

// useEvents follows a topic while the page is showing (one stream each:
// browsers allow only a few connections to a server, and every tab's
// count). Showing again opens it again, and its hello catches up. A change
// is told at once; those within a moment of it, once, after it (bursts).
// says is what the stream's address says of the page besides its topic
// ("&app=jira"): the stream is opened again when it changes.
export function useEvents(topic: Topic, on: EventHandlers, says = "") {
  const visible = useVisible();
  const handlers = useRef(on);
  handlers.current = on;
  useEffect(() => {
    if (!visible) return;
    const events = new EventSource(`/api/events?topic=${topic}${says}`);
    const changes = bursts(() => handlers.current.changed());
    events.addEventListener("hello", () => handlers.current.hello?.());
    events.addEventListener(topic, changes.changed);
    events.onerror = () => handlers.current.lost?.(events.readyState === EventSource.CLOSED);
    return () => {
      changes.stop();
      events.close();
    };
  }, [topic, says, visible]);
  return visible;
}

// useWorkspace is the workspace, kept up to date by its event stream while
// the page is showing; showing again catches up.
export function useWorkspace(): WorkspaceState {
  const [state, setState] = useState<WorkspaceState>({ connection: "connecting" });
  const mounted = useRef(true);
  // (Set on each mount: in development React mounts twice, and the first
  // unmount would otherwise leave it false for good.)
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);
  const load = () =>
    getJSON<Workspace>("/api/workspace").then(
      (workspace) => mounted.current && setState((s) => ({ ...s, workspace, error: undefined })),
      (error: Error) => mounted.current && setState((s) => ({ ...s, error })),
    );
  const visible = useEvents("workspace", {
    hello: () => {
      setState((s) => ({ ...s, connection: "live" }));
      load();
    },
    changed: load,
    lost: (closed) => {
      setState((s) => ({ ...s, connection: "lost" }));
      // A stream refused for being signed out never opens: find out why.
      if (closed) load();
    },
  });
  useEffect(() => {
    if (visible) setState((s) => ({ ...s, connection: "connecting" }));
  }, [visible]);

  return state;
}

// Plugin is one of the workspace's plugins: what it adds to the sidebar,
// and where it stands. Mirrors internal/web/plugins.go.
export interface Plugin {
  name: string;
  description?: string;
  state: "ready" | "setup" | "failed" | "stopped";
  error?: string;
  missing?: string[];
  views: PluginView[];
  // refs are the kinds of thing it has that words name, as ws took them.
  refs?: PluginRef[];
  // actions are what it does of its own when asked: Find's Do has them.
  actions?: PluginAction[];
  // changed counts the times it's said what its view shows is stale.
  changed: number;
  // starts counts the times it's been started.
  starts: number;
}

// PluginRef is a kind of thing a plugin has that words name (a Jira
// issue), and how one is named: one of prefixes, then its tail (a number,
// when it doesn't say). address is where one is outside ws and path where
// one opens in the plugin's app, {ref} its name in each. fields are what
// one holds, and views how the plugin has one drawn from them: under its
// name (card), and beside the list in Find (pane).
export interface PluginRef {
  kind: string; // the plugin's word for it: issue
  title?: string; // what ws calls one: Jira issue
  icon?: string;
  prefixes: string[];
  tail?: string;
  address?: string;
  path?: string;
  fields?: ThingField[];
  views?: ThingViews;
  actions?: PluginAction[]; // what can be done to one of them
}

// PluginAction is something a plugin does when asked: of its own, or to
// one of its things.
export type PluginAction = ThingAction;

// ActionAsk is what a plugin's action wants to know before it goes on:
// one of some options (pick), or a line of words (text).
export interface ActionAsk {
  title: string;
  type: "pick" | "text";
  options?: { value: string; label: string; hint?: string }[];
  placeholder?: string;
}

// ActionAnswer is what running a plugin's action came to: something it
// asks first, or, done, a line to say and somewhere in ws to go.
export interface ActionAnswer {
  ask?: ActionAsk;
  say?: string;
  open?: string;
}

// runPluginAction runs a plugin's action: one of its own, or, with a
// thing's kind and name, one done to that thing. answers are what's been
// answered to what it asked so far, in order: it's run again with each
// answer until it asks nothing more.
export const runPluginAction = (plugin: string, id: string, on: { kind?: string; ref?: string; answers: string[] }) =>
  send<ActionAnswer>("POST", `/api/plugins/${encodeURIComponent(plugin)}/actions/${encodeURIComponent(id)}`, on);

// startMissionAbout starts a mission about something ws names (a plugin's
// thing, RAM-123): it waits to be briefed, on its page.
export const startMissionAbout = (about: string) => send<{ slug: string; path: string }>("POST", "/api/missions", { about });

// PluginThing is one of a plugin's things: its name (key), and its fields
// by name, as its kind declared them (a time as RFC 3339). told is whether
// ws had it from what the plugin's told it, and answered from its own
// records; otherwise the plugin was asked just now.
export interface PluginThing {
  key: string;
  fields: Record<string, string>;
  changed?: string;
  told: boolean;
}

// getPluginThing is one of a plugin's things, by its kind and name: as ws
// has it, or as its plugin says when asked. One the plugin has none of is
// an ApiError with status 404; any other failure is the plugin not being
// there to ask.
export const getPluginThing = (plugin: string, kind: string, ref: string) =>
  getJSON<PluginThing>(`/api/plugins/${encodeURIComponent(plugin)}/refs/${encodeURIComponent(kind)}/${encodeURIComponent(ref)}`);

export interface PluginView {
  title: string;
  icon?: string;
  under?: string;
  path: string; // where in the plugin's pages it opens
  badge?: number;
}

// usePlugins are the workspace's plugins, kept up to date while the page
// is showing. undefined until they're first read. showing is the plugin
// whose app the page shows, if it shows one. The stream that follows them
// is how ws knows a page is showing, and with showing, which plugin's app
// is on screen: it tells plugins who's looking, and they fetch to suit.
export function usePlugins(showing?: string): Plugin[] | undefined {
  const [plugins, setPlugins] = useState<Plugin[]>();
  const load = () =>
    getJSON<Plugin[]>("/api/plugins").then(
      (p) => setPlugins(p),
      () => setPlugins((was) => was ?? []), // none it can show is none
    );
  useEvents("plugins", { hello: load, changed: load }, showing ? `&app=${encodeURIComponent(showing)}` : "");
  return plugins;
}

// Review: mirrors internal/review/diff.go.

export interface Diff {
  baseRef: string;
  base: string;
  since?: string; // only what's changed since this commit (then it's base)
  head: string;
  commits: Commit[];
  uncommitted: string[];
  files: DiffFile[];
}

export interface Commit {
  sha: string;
  subject: string;
  author: string;
  time: string;
}

export interface DiffFile {
  path: string;
  oldPath?: string;
  status: "added" | "deleted" | "modified" | "renamed" | "copied" | "type";
  oldBlob?: string;
  newBlob?: string;
  binary?: boolean;
  additions: number;
  deletions: number;
  large?: boolean;
  hunks: Hunk[];
  // skim: why it needn't be read closely. Such files come last, folded.
  // ("guide": the review's guide says so; the page says that, not git.)
  skim?: "lockfile" | "generated" | "snapshot" | "renamed" | "guide";
}

export interface Hunk {
  oldStart: number;
  oldLines: number;
  newStart: number;
  newLines: number;
  section?: string;
  lines: DiffLine[];
}

export interface DiffLine {
  k: "c" | "a" | "d";
  o?: number;
  n?: number;
  t: string;
  noeol?: boolean;
}

// Repo is one of the workspace's repos, as ws labels it: its display name
// and colour, its group (hidden or not), and its capsules.
export interface Repo {
  name: string;
  label: string;
  color: string;
  group?: string;
  hidden?: boolean;
  cloned: boolean;
  documents?: boolean; // a space of documents, whole: its home is Documents, and Files folds it away
  capsules: string[];
  aliases?: string[]; // what else it's called, which names it in what's written
  branches?: Record<string, string>; // each capsule's branch
  landing?: "pr" | "direct"; // how work lands in it: through a pull request, or straight to its default branch
}

// Worktree is a repo's ground (GROUND: its default branch, checked out) or
// one of its capsules, with its branch and where it stands.
export interface Worktree {
  name: string;
  ground?: boolean;
  branch: string;
  dirty?: boolean;
  ahead?: number;
  behind?: number;
}

export const GROUND = ".ground";

// Space is a space of documents: a repo of them, or a folder of one.
// count is how many documents it has, as read from its default branch.
export interface Space {
  repo: string;
  label: string;
  color: string;
  folder?: string; // "" for the whole repo
  count: number;
  cloned: boolean;
  folders: { name: string; count: number }[]; // the folders at its top, by name
  kinds: SpaceKind[]; // the workspace's kinds whose path is in it: its library's lists
}

// SpaceKind is a kind of document a space has, and where one can stand
// (none, for a kind with no moves).
export interface SpaceKind {
  name: string;
  title: string;
  states?: string[];
}

// SpaceDoc is a document in a space: its path there, its title, its kind
// and where it stands, what its path says (a date, a number), and who
// changed it last and when.
export interface SpaceDoc {
  path: string;
  title: string;
  kind?: string;
  state?: string;
  date?: string;
  number?: string;
  changed?: string;
  changedBy?: string;
}

// SpaceActivity is a space's top: the commits on its default branch lately
// and the documents each changed, the capsules with changes to its
// documents and the pull requests open from them, and how its ground
// stands.
export interface SpaceActivity {
  lately: SpaceCommit[];
  inFlight: SpaceFlight[];
  fresh: SpaceFresh;
}
export interface SpaceCommit {
  sha: string;
  at: string;
  by: string;
  subject: string;
  docs: SpaceDocChange[];
}
export interface SpaceDocChange {
  path: string;
  title?: string;
  how: "added" | "changed" | "removed";
}
export interface SpaceFlight {
  capsule: string;
  branch?: string;
  mission?: string;
  pr?: { number: number; title: string; url?: string };
  docs: SpaceDocChange[];
}
export interface SpaceFresh {
  fetched?: string; // when its repo was last fetched; never, here: absent
  behind: number; // commits its ground is behind the remote's default branch
  ahead?: number;
  branch: string;
  dirty?: boolean; // the ground has changes of its own
}
export const getSpaceActivity = (repo: string) => getJSON<SpaceActivity>(`/api/documents/${encodeURIComponent(repo)}/activity`);
export const fetchSpace = (repo: string) => send<{ fresh: SpaceFresh }>("POST", `/api/documents/${encodeURIComponent(repo)}/fetch`, {});
export const pullSpace = (repo: string) => send<{ fresh: SpaceFresh }>("POST", `/api/documents/${encodeURIComponent(repo)}/pull`, {});

// DocHit is a document a search found, led to the section that says it
// best (anchor: its heading, none for the top), its snippet's words found
// marked, and of which kind and where it stands.
export interface DocHit {
  repo: string;
  label: string; // its space's
  path: string;
  title: string;
  anchor?: string;
  heading?: string;
  snippet: { text: string; mark?: boolean }[];
  score: number;
  every?: boolean; // every word was found in it
  kind?: string;
  state?: string;
}

// searchDocuments finds documents in the workspace's spaces by what they
// say, best first: in the spaces asked, of a kind and where it stands.
export function searchDocuments(q: string, o: { repos?: string[]; kind?: string; state?: string; limit?: number } = {}) {
  const p = new URLSearchParams({ q });
  for (const r of o.repos ?? []) p.append("repo", r);
  if (o.kind) p.set("kind", o.kind);
  if (o.state) p.set("state", o.state);
  if (o.limit) p.set("limit", String(o.limit));
  return getJSON<{ hits: DocHit[]; more: number }>(`/api/documents/search?${p}`);
}

// Citation is a document citing another: where, and how (a link, or a
// ref naming it).
export interface Citation {
  repo: string;
  path: string;
  title?: string;
  line: number;
  how: "link" | "ref";
}
export const getCites = (repo: string, path: string) => getJSON<{ citedBy: Citation[]; cites: Citation[] }>(`/api/documents/${encodeURIComponent(repo)}/cites?${new URLSearchParams({ path })}`);

// getSpaces is the workspace's spaces, read fresh; problem says why one
// couldn't be (it's as it was last read).
export const getSpaces = () => getJSON<{ spaces: Space[]; problem?: string }>("/api/documents");
export const getSpace = (repo: string) => getJSON<{ space: Space; docs: SpaceDoc[]; problem?: string }>(`/api/documents/${encodeURIComponent(repo)}`);

export const getRepos = () => getJSON<{ repos: Repo[] }>("/api/repos").then((r) => r.repos);
export const getWorktrees = (repo: string) => getJSON<{ worktrees: Worktree[] }>(`/api/repos/${encodeURIComponent(repo)}/worktrees`).then((r) => r.worktrees);

// worktreeAPI is where a repo's ground or capsule's files are read, as
// capsuleAPI is a mission's capsule's.
export function worktreeAPI(repo: string, name: string): string {
  return `/api/repos/${encodeURIComponent(repo)}/worktrees/${encodeURIComponent(name)}`;
}

// filesTo is the Files page for a repo's ground or capsule, at a file if
// one's named.
export function filesTo(repo: string, name: string, path?: string): string {
  return `/files/${encodeURIComponent(repo)}${name === GROUND ? "" : `/${encodeURIComponent(name)}`}${path ? `?${new URLSearchParams({ path })}` : ""}`;
}

// placeOf is where a document in a repo is, for the reader: its links to
// other documents open them at their address, and to other files in Files,
// in the same worktree.
export function placeOf(d: DocIn): Place {
  return { api: worktreeAPI(d.repo, d.worktree), path: d.path, to: (p) => (/\.(md|markdown|mdx)$/i.test(p) ? docAddress(docRef(d.repo, d.worktree, p)) : filesTo(d.repo, d.worktree, p)) };
}

export function capsuleAPI(slug: string, repo: string, name: string): string {
  return `/api/missions/${encodeURIComponent(slug)}/capsules/${encodeURIComponent(repo)}/${encodeURIComponent(name)}`;
}

export const getDiff = (api: string, since?: string) => getJSON<Diff>(`${api}/diff${since ? `?since=${since}` : ""}`);

export const getFileDiff = (api: string, base: string, head: string, path: string) =>
  getJSON<DiffFile>(`${api}/file-diff?${new URLSearchParams({ base, head, path })}`);

export const getBlob = (api: string, rev: string, path: string) =>
  getJSON<{ lines: string[] }>(`${api}/blob?${new URLSearchParams({ rev, path })}`).then((b) => b.lines);

// Reviews: mirrors internal/review/review.go.

export const noteKinds = ["blocking", "suggestion", "question", "nit"] as const;
export type NoteKind = (typeof noteKinds)[number];
export type Verdict = "approve" | "changes" | "comment";

export interface ReviewNote {
  id: string;
  path: string;
  side: "new" | "old";
  line: number;
  endLine: number;
  commit: string;
  excerpt: string[];
  // kind is "" for someone's comment on a pull request that doesn't say.
  kind: NoteKind | "";
  body: string;
  // by: who wrote it, when it isn't you: "agent" (a comment an agent
  // proposed and you added), or a login (someone's on a pull request).
  by?: string;
  url?: string; // where it is on GitHub
  suggestion?: string | null;
  status: NoteStatus;
  thread: { by: string; at: string; body: string; commit?: string; status?: string }[];
}

// DocQuote is text a thread highlights, with a little either side to find
// it by again. An empty exact is a thread on the whole block at its lines.
export interface DocQuote {
  exact: string;
  prefix?: string;
  suffix?: string;
}

export type NoteStatus = "open" | "addressed" | "wontfix" | "resolved";

export interface Review {
  id: string;
  capsule: string;
  baseRef: string;
  base: string;
  head: string;
  state: "draft" | "submitted";
  verdict?: Verdict;
  summary?: string;
  created: string;
  submitted?: string;
  comments: ReviewNote[];
}

export interface ReviewState {
  draft: Review | null;
  reviews: Review[];
  viewed: Record<string, string>;
  url?: string; // a pull request's: where the review just submitted is on GitHub
}

const send = <T>(method: string, path: string, body?: unknown) => request<T>(method, path, { body });

export interface NoteDraft {
  kind: NoteKind;
  body: string;
  suggestion?: string | null;
}

export const getReview = (api: string) => getJSON<ReviewState>(`${api}/review`);

export const addNote = (api: string, at: { base: string; head: string }, n: NoteDraft & { path: string; side: "new" | "old"; line: number; endLine: number }) =>
  send<ReviewState>("POST", `${api}/review/comments`, { ...at, ...n });

export const editNote = (api: string, id: string, n: NoteDraft) =>
  send<ReviewState>("PATCH", `${api}/review/comments/${encodeURIComponent(id)}`, n);

export const deleteNote = (api: string, id: string) => send<ReviewState>("DELETE", `${api}/review/comments/${encodeURIComponent(id)}`);

export const setViewed = (api: string, path: string, blob: string, viewed: boolean) =>
  send<ReviewState>("PUT", `${api}/review/viewed`, { path, blob, viewed });

export const submitReview = (api: string, at: { base: string; head: string }, verdict: Verdict, summary: string) =>
  send<ReviewState>("POST", `${api}/review/submit`, { ...at, verdict, summary });

export const discardReview = (api: string) => send<ReviewState>("DELETE", `${api}/review`);

// respond is you on a submitted comment: a reply, or resolving or reopening it.
export const respond = (api: string, id: string, what: "reply" | "resolve" | "reopen", body = "") =>
  send<ReviewState>("POST", `${api}/review/comments/${encodeURIComponent(id)}/respond`, { what, body });

// Missions in full: mirrors MissionDetail in internal/web/api.go.

export interface Proposal {
  text: string;
  by?: string;
  capsule?: string;
  options?: string[];
}

export interface Stage {
  n: number;
  title: string; // a few words
  text: string; // the whole stage
  state: "accepted" | "current" | "todo";
  crew?: string[];
  checkIn?: boolean;
  why?: string;
  // A step of the mission's plan: its kind, who's on it (you: a
  // checkpoint, yours alone; ws: an Open Pull Request, ws's own), that as
  // the plan says it, and what it makes. None for a brief's stage.
  type?: "Work" | "Verify" | "Review" | "Decide" | "Open Pull Request" | "Land" | "Wait" | "Plan";
  who?: "crew" | "you" | "ws" | "CI" | "outside" | "the planner";
  whoSays?: string;
  in?: string; // where it works, as the plan says: a repo, or one of the mission's capsules (repo/name)
  makes?: string;
  opensPR?: boolean; // it makes a pull request, opened through ws
  // A Work step's expansion: the step whose crew it carries on from, its
  // limit as written and when its time runs out (or that it has), and its
  // checks with what the latest round (round) came to.
  carryOn?: number;
  limit?: string;
  deadline?: string;
  outOfTime?: boolean;
  checks?: StageCheck[];
  round?: number;
  // The orchestrator's decisions on it: when it applies and where it runs,
  // as the plan says; whether it waits for a call, was skipped, or was
  // started again; and its runs, one per repo.
  onlyIf?: string;
  inEach?: string;
  held?: boolean;
  skipped?: boolean;
  attempts?: number;
  runs?: StageRun[];
  // A Wait step's expansion: who it waits on (you, or who or what, in the
  // plan's words; absent when it doesn't say), and how it's passed:
  // watched (ws runs its checks, and passes it), done (you mark it done)
  // or said (anyone says it's happened). watch is since when it's waited,
  // and ws's last look at its checks, whose results are on checks.
  on?: string;
  passes?: "watched" | "done" | "said";
  watch?: StageWatch;
}

// StageWatch is a Wait step being waited for.
export interface StageWatch {
  since: string;
  looked?: string; // when ws last ran its checks
  looks?: number;
  came?: string; // what that look came to: met, not yet, ws couldn't tell
}

// StageRun is one of a step's runs, in one repo.
export interface StageRun {
  repo: string;
  capsule?: string;
  done?: boolean;
}

// StageCheck is one of a Work step's checks: a command ws runs (run), or
// words the judge judges; and what it last came to, if it's been checked.
export interface StageCheck {
  text: string;
  run?: boolean;
  result?: "met" | "unmet" | "can't tell" | "yours";
  evidence?: string;
  by?: string;
}

export interface CapsuleDetail extends Capsule {
  dirty?: boolean;
  ahead?: number;
  behind?: number;
  pr?: { number: number; title: string; url: string; merged?: boolean; checks?: string };
  // Its repo has no pull requests: its commits land straight on the
  // default branch when you say. toLand is how many it has to land, and
  // landed when they did.
  direct?: boolean;
  toLand?: number;
  landed?: string;
  // runs are what's running in it (ws run), and recipes the names of what
  // its ws.repo.toml says it can run.
  runs?: RunDetail[];
  recipes?: string[];
}

// RunDetail is a run (ws run) in a capsule. Mirrors internal/web/api.go.
export interface RunDetail {
  ref: string;
  name: string;
  recipe?: string;
  link: string; // by its name, when the workspace has names; else local
  local: string; // at 127.0.0.1 and its port
  port: number;
  state: "running" | "exited" | "gone";
  status?: number; // its exit status, once exited
  up?: boolean; // something listens on its port
  ends?: string; // when it's ended without being asked, in words, while it runs
  ended?: string;
  owner: string;
  by?: string;
  command: string;
  started: string;
  // Where it runs: its repo, its worktree as a name says it (a capsule's
  // name, ground or silo), and the folder.
  repo: string;
  worktree: string;
  dir?: string;
  pid?: number; // leads its process group, while it runs
  // Who it's for, beyond owner's kind.
  mission?: string;
  step?: number;
  crew?: string;
  conversation?: string;
  ownerPid?: number;
  ownerCommand?: string;
}

// Names is where the workspace's names under .localhost stand (ws
// router): whether ws.local.toml's [router] turns them on, the name it
// asks for and the one it has ("" until its daemon's written its routes;
// another, given a clash), the machine's router's port (absent: none
// serving; fallback: 80 couldn't be had) and the daemon holding it, and
// the web UI by its name. Mirrors internal/web/runs.go.
export interface Names {
  on: boolean;
  want: string;
  name?: string;
  port?: number;
  fallback?: boolean;
  heldBy?: number;
  web?: string;
}

// RunsInfo is the Runs page's: every run in the workspace, and its names.
export interface RunsInfo {
  names: Names;
  runs: RunDetail[];
}

export const getRuns = () => getJSON<RunsInfo>("/api/runs");

// endRun ends any run now, or clears away one that's ended, its log with it.
export const endRun = (ref: string) => send<{ stopped: string }>("POST", "/api/runs/stop", { ref });

// getRunLog is the end of a run's output.
export const getRunLog = (ref: string) => getJSON<{ ref: string; log: string }>(`/api/runs/log?ref=${encodeURIComponent(ref)}`);

// runRecipe starts a capsule's recipe, for as long as someone has the
// mission open (and 15 minutes after).
export const runRecipe = (slug: string, repo: string, capsule: string, recipe: string) =>
  send<RunDetail>("POST", `/api/missions/${encodeURIComponent(slug)}/runs`, { repo, capsule, recipe });

// pointSilo points a repo's silo at a capsule (ws silo point), and says
// what it did.
export const pointSilo = (repo: string, to: string) => send<{ said: string }>("POST", `/api/silos/${encodeURIComponent(repo)}/point`, { to });

// stopRun ends a run now.
export const stopRun = (slug: string, ref: string) => send<{ stopped: string }>("POST", `/api/missions/${encodeURIComponent(slug)}/runs/stop`, { ref });

// Landing is where a capsule stands for landing, in a repo that takes
// commits straight to its default branch (direct): what would land, and
// whether your latest review approves it as it is. Mirrors
// internal/web/landing.go.
export interface Landing {
  direct: boolean;
  branch: string;
  commits: string[];
  dirty: boolean;
  verify?: string;
  mission?: string;
  approved: boolean;
  pending?: boolean;
  landed?: Landed;
}

export interface Landed {
  branch: string;
  head: string;
  commits: string[];
  mission?: string;
  warnings?: string[];
}

// LandStopped is a landing that stopped, with nothing pushed: at which
// step, why, and what git or the repo's check said.
export interface LandStopped {
  step: string;
  message: string;
  output?: string;
}

export const getLanding = (repo: string, name: string) => getJSON<Landing>(`${worktreeAPI(repo, name)}/landing`);
export const landCapsule = (repo: string, name: string) => send<{ landed?: Landed; stopped?: LandStopped }>("POST", `${worktreeAPI(repo, name)}/land`);

export interface CrewDetail extends Crew {
  endedAt?: string;
  exit?: string;
  how?: "interrupted" | "handed-back" | "stopped"; // how it ended, beyond its exit
  note?: string;
  dispatch?: string;
  handoff?: string;
}

export interface Artifact {
  ref: string;
  kind: string;
  label?: string;
  state: string;
  url?: string;
  path?: string;
  by?: string;
  added: string;
  readable: boolean;
  // Where the threads on it stand, for a document with open ones.
  threads?: { open: number; waiting: number };
  // How many versions it has, for an artifact (a3): opened at /artifacts/a3.
  versions?: number;
}

// --- Artifacts: versioned HTML prototypes

// ArtifactVersion is one version: what it's based on, who added it (you,
// or crew: c1), where it was made (a conversation, w9, or a mission's
// slug, with its title then), and its note.
export interface ArtifactVersion {
  n: number;
  basedOn?: number;
  at: string;
  by: string;
  from?: string;
  proposal?: number;
  fromTitle?: string;
  note?: string;
  // How many files it has and what they come to, on an artifact's page;
  // absent while there are none to read.
  files?: number;
  size?: number;
}

export type ArtifactStatus = "live" | "promoted" | "discarded";

// ArtifactInfo is an artifact as the list, its refs and Find show it.
export interface ArtifactInfo {
  id: string; // a3
  title: string;
  status: ArtifactStatus;
  gone?: boolean; // its folder isn't where it was, and it wasn't discarded
  folder: string; // its name in .ws/artifacts, on this machine
  where: string; // .ws/artifacts/<folder>/
  home?: ArtifactHome; // where it lives in a repo, promoted there: none for one on this machine only
  here?: boolean; // promoted to stay on this machine, whatever the workspace says of where promoted ones live
  made: string;
  versions: number;
  latest?: ArtifactVersion;
  from?: { from: string; title?: string }[]; // where its versions came from, the latest first
  promoted?: { at: string; by: string; version: number; into: string };
  discarded?: { at: string; by: string; reason?: string };
  // expires is when a live one expires, if it does; expired, by now,
  // until the daemon discards it (or Clear expired does).
  expires?: string;
  expired?: boolean;
  erased?: boolean; // discarded, and its files deleted for good
  restorable?: boolean; // discarded, and it can be put back
}

// ArtifactHome is where a promoted artifact lives in a repo, for everyone
// with it: its folder there as it's said to people
// (records:artifacts/2026-11-12-horizon-glow/), the version that's there,
// and the commit that put it there.
export interface ArtifactHome {
  where: string;
  repo: string;
  version: number;
  commit?: string;
  at: string;
}

// ArtifactDone is what promoting, discarding or restoring one did.
export interface ArtifactDone {
  artifact: ArtifactInfo;
  home?: string; // the folder in a repo it was put in, taken out of or put back in
  landed?: string; // what landing that commit did
  first?: boolean; // it's the first time it was put there
  into?: string; // what it went into: a mission's slug, or a document's ref
  started?: string; // a mission started from it
  recorded?: string; // the mission it was recorded on
}

// ArtifactPromotion is promoting one: the version chosen (0: its latest),
// and what it went into (a mission's slug, or a document's ref), or a
// mission to start from it. For one that lives in no repo yet, home is
// the folder it's to live in (repo:folder/), and here keeps it on this
// machine only; neither is where the workspace says promoted ones live.
// One that lives in a repo goes there, with nothing said.
export interface ArtifactPromotion {
  version?: number;
  into?: string;
  start?: string;
  home?: string;
  here?: boolean;
}

const artifactPath = (id: string, what: string) => `/api/artifacts/${encodeURIComponent(id)}/${what}`;
export const promoteArtifact = (id: string, p: ArtifactPromotion) => send<ArtifactDone>("POST", artifactPath(id, "promote"), p);
export const discardArtifact = (id: string, reason: string) => send<ArtifactDone>("POST", artifactPath(id, "discard"), { reason });
export const restoreArtifact = (id: string) => send<ArtifactDone>("POST", artifactPath(id, "restore"), {});
export const clearExpiredArtifacts = () => send<{ local: string[] }>("POST", "/api/artifacts/clear", {});

export interface ArtifactDetail extends ArtifactInfo {
  all: ArtifactVersion[]; // oldest first
  // homeOffered is the folder in a repo it has, or is offered, to live in:
  // where it lives, or the folder the workspace names for it; none where
  // there's none to offer.
  homeOffered?: string;
}

export const getArtifactDetail = (id: string) => getJSON<ArtifactDetail>(`/api/artifacts/${encodeURIComponent(id)}`);
// ArtifactFile is one of a version's files: its path in the version
// (css/app.css), and its size.
export interface ArtifactFile {
  path: string;
  size: number;
}

// ArtifactSource is one of a version's files, as text to read (its page,
// index.html, unless another's asked for), and the version's files, to
// choose another. binary: it isn't text, and there's none.
export interface ArtifactSource {
  version: number;
  file: string;
  text: string;
  binary?: boolean;
  size: number;
  files: ArtifactFile[];
}

export const getArtifactSource = (id: string, version: number, file = "") =>
  getJSON<ArtifactSource>(`/api/artifacts/${encodeURIComponent(id)}/source?${new URLSearchParams(file ? { v: String(version), file } : { v: String(version) })}`);

// ArtifactCompare is two versions' files, a's against b's: each file that
// differs, diffed as a review's file is (internal/web/artifacts.go), with
// its lines in b, to show the unchanged ones between the changes. Each
// version is named as it was read (a3.v2). No files: the two are the
// same. A file that isn't text is binary, with no hunks.
export interface ArtifactCompare {
  a: string;
  b: string;
  files: { file: DiffFile; lines: string[] }[];
}

export const getArtifactCompare = (a: string, b: string) => getJSON<ArtifactCompare>(`/api/artifacts/compare?${new URLSearchParams({ a, b })}`);

// frameURL is where a version runs: a route of its own, outside the app
// and its API, sandboxed by its own policy (internal/web/artifacts.go).
// tweaks is the state it opens in, as its address says it ("?t.density=compact",
// artifacts/tweaks.ts): none, for the state its page opens as.
export const frameURL = (id: string, version: number, tweaks = "") => `/frame/${encodeURIComponent(id)}/v${version}${tweaks}`;

// proposalFrameURL is where an artifact a conversation proposed runs, as a
// version does: the workspace's conversation (w9), or a mission's (slug).
export const proposalFrameURL = (slug: string, id: string, n: number) =>
  `/frame/${slug ? `missions/${encodeURIComponent(slug)}/` : ""}${encodeURIComponent(id)}/proposals/${n}`;

export interface MissionEvent {
  time: string;
  kind: string;
  text: string;
  by?: string; // a crew id, "orchestrator", or absent (you, or ws)
  to?: string; // whom a message to crew is for (resume_with, told): by is then who said it ("you", "orchestrator", "ws"), none for one logged before ws said
}

// Bundle is a verification bundle: what was handed over to show a step's
// work is as its checks say. It's never edited: a later one of its step
// takes its place.
export interface Bundle {
  id: string; // b3
  step?: number; // its step's number as the plan has it now
  stepName?: string;
  crew: string; // who made it
  by?: string; // what they ran on: claude on sonnet
  own?: boolean; // they did the work it shows; else they're a verifier, who didn't
  capsule?: string; // repo/name
  commit?: string; // the commit it's a bundle of
  stale?: boolean; // its capsule has moved on since, to head
  head?: string;
  at: string;
  replaces?: string;
  replacedBy?: string;
  verdict?: "approved" | "sent back"; // yours of it, as a review has one
  verdictAt?: string;
  notes?: BundleNote[]; // what you said wasn't right, when you sent it back
  looked?: string[]; // its checks you've marked as looked at, by their keys in its list
  asked?: boolean; // the mission is at a step of yours (askedAt) that looks at it: a Review step, or its own with its checks yours to say
  askedAt?: number;
  summary: string;
  try?: BundleTry[];
  checks: BundleCheck[];
  notVerified?: string[];
  noticed?: string[]; // what its crew came across that isn't one of its checks
  ran?: BundleRan[]; // its step's checks that are commands, as ws ran them at its commit
}

// BundleNote is what you said isn't right of a bundle: of a check, by its
// number, or (0) of it as a whole.
export interface BundleNote {
  check: number;
  text: string;
}

// BundleTry is something to do to see it yourself: command where it starts
// with one, look for what to look for.
export interface BundleTry {
  text: string;
  command?: string;
  look?: boolean;
}

// CheckVerdict is what a check came to: yours is one a verifier left for
// you, as the plan keeps it yours to settle.
export type CheckVerdict = "met" | "unmet" | "can't tell" | "yours";

export interface BundleCheck {
  n: number;
  check?: string; // the step's check; none for one of the bundle's own
  verdict: CheckVerdict;
  said: string;
  points?: string[]; // facts of the same kind, listed under what it said
  shows?: BundleEvidence[];
  lines?: string[];
  try?: BundleTry[];
  asks?: string[];
  judged?: BundleRan; // what the judge last said of it
}

// BundleEvidence is a file that shows a check, served from url.
export interface BundleEvidence {
  name: string;
  url: string;
  kind: "image" | "video" | "text";
  size?: number;
  caption?: string;
  before?: BundleEvidence; // the same on ground
  madeBy?: string; // the command that made it
}

// BundleRan is what a check of the bundle's step came to when ws ran it,
// or had it judged.
export interface BundleRan {
  n?: number;
  check: string;
  result: CheckVerdict;
  evidence?: string;
  by: string;
}

export interface MissionDetail {
  slug: string;
  title: string;
  state: MissionState;
  created: string;
  request?: string;
  from?: string;
  conversation?: string; // the conversation it came out of: w3, or another mission's (cache/t4)
  conversationTitle?: string;
  talks: Talk[]; // the conversations about it (its Talk tab), newest first
  planner?: string; // the conversation its planner is in (t1), which its brief's page talks in
  brief: string;
  briefUnwritten?: boolean; // the brief's empty, or the template it was made with
  plan?: string; // plan.md: its route, in place of the brief's stages
  planProblem?: string; // why plan.md doesn't read, when it was changed by hand
  flag?: Proposal; // the oldest of tasks
  tasks?: MissionTask[]; // what it waits on you for, oldest first
  dispatch?: Proposal;
  conclude?: Conclusion; // how it ends, suggested
  ending?: Conclusion; // how it ended, once you said
  endedCapsules?: EndedCapsule[];
  at?: string; // where it is on its route, in words: "Step 2 of 7, yours: …"
  z?: string[]; // what has to be true for it to end at Z: each part checked when it ends
  reReview?: { capsule: string; review: string; answered: number; total: number }[];
  headlines: Headline[]; // most pressing first: the first is the mission's
  runs: OrchestratorRun[]; // the orchestrator's latest runs on it, newest first
  stages: Stage[];
  capsules: CapsuleDetail[];
  crew: CrewDetail[];
  artifacts: Artifact[];
  bundles: Bundle[]; // its verification bundles, oldest first (its Verify tab)
  decisions: Decision[]; // those that stand first, newest first
  events: MissionEvent[];
  github: boolean;
  crewPicks: CrewPicks; // what Launch crew may start crew on
}

// CrewPick is a harness, and the model to run on it (none: the agent's own).
export interface CrewPick {
  harness: string;
  model?: string;
}

// CrewPicks are what new crew may be launched on from a mission's page: the
// default (what Launch crew starts them on), and the others [crew] allowed
// lists (none: just the default).
export interface CrewPicks {
  default: CrewPick;
  allowed: CrewPick[];
}

// Decision is what was decided on a mission, and why.
export interface Decision {
  id: string;
  what: string;
  why?: string;
  by: string; // "you", a crew id, or "orchestrator"
  from?: string; // where it was decided: a ref
  about?: string[];
  at: string;
  state: "current" | "superseded" | "withdrawn";
  supersededBy?: string;
  withdrawnWhy?: string;
}

export interface ArtifactContent extends Artifact {
  text: string;
  markdown: boolean;
  version: string; // names text: comments say the version they were made on
  annotatable: boolean; // a document to comment on
  stage?: DocStage; // the stage of the brief's plan it was made for
  editable: boolean; // it can be edited here (in a capsule, or the mission's own)
  uneditable?: string; // why it can't, if it can't
  media?: string; // its content type, for an image or a video: shown as it is, from raw, with no text
  raw?: string; // where an image or a video is served from, as it is
  in?: DocIn; // the worktree it was read from, for a file in a repo
  // The mission that records it, for one opened as a file: its threads are there.
  mission?: { slug: string; title: string; ref: string };
  saves?: DocSaves; // how an edit to it is saved, for a document outside a mission
  docKind?: DocKind; // which of the workspace's kinds of document it is, if one
  page?: string; // the page it was written in, for a document filed from one
}

// DocKind is a kind of document the workspace has ([documents.<name>]), as
// one of its documents is of it: what one's called, where they go
// (repo:path, with {date}, {slug} and {number}), the file that says how
// one's written, and the first line of a commit that changes this one
// (from the kind's message; none if it has none).
export interface DocKind {
  name: string;
  title: string;
  where: string;
  guide?: string;
  subject?: string;
}

// kindHint says what a document's kind is, for its label's title.
export const kindHint = (k: DocKind) =>
  `${k.title}: one of the workspace's kinds of document (${k.name}, at ${k.where})${k.guide ? `. How one's written: ${k.guide}` : ""}`;

// DocSaves is how saving a document outside a mission goes: in its
// capsule (committed as you); landed, a commit straight to the default
// branch of a repo with no pull requests (once verify passes); or lifted,
// into a capsule made for it in a repo that takes them, and left to you.
export interface DocSaves {
  how: "capsule" | "land" | "lift";
  branch: string;
  verify?: string;
  subject: string; // the commit's first line, unless you give one
}

// DocIn is where a document is: a repo, one of its worktrees (GROUND, or
// a capsule's name) and its path there.
export interface DocIn {
  repo: string;
  worktree: string;
  path: string;
}

// DocDraft is an edit to a document not saved yet, kept with the mission:
// what's written, and the version it's from.
export interface DocDraft {
  ref: string;
  base: string;
  text: string;
  at: string;
}

// DocSaved is what saving an edited document did; conflict, instead, is
// an edit that clashes with changes made meanwhile (nothing's written):
// the two merged, each clash marked, to settle and save from version.
export interface DocSaved {
  message?: string;
  version: string;
  commit?: string;
  merged?: boolean;
  conflict?: { text: string; clashes: number };
  ref?: string; // where the document is now, when saving moved it (into a capsule lifted for it)
}

// A document's draft and save are its mission's, or (no slug) the
// workspace's, for a file in a repo that's in no mission.
const docsBase = (slug: string) => (slug ? `/api/missions/${encodeURIComponent(slug)}` : "/api/docs");
const draftPath = (slug: string, ref?: string) => `${docsBase(slug)}/draft${ref ? `?${new URLSearchParams({ ref })}` : ""}`;

// readDoc reads a document as it is now: a mission's, or one outside any.
export const readDoc = (slug: string, ref: string) => (slug ? getArtifact(slug, ref) : getDoc(ref));

export const getDraft = (slug: string, ref: string) => getJSON<{ draft: DocDraft | null }>(draftPath(slug, ref)).then((r) => r.draft);
export const putDraft = (slug: string, d: { ref: string; base: string; text: string }) => send<{ draft: DocDraft }>("PUT", draftPath(slug), d);
export const dropDraft = (slug: string, ref: string) => send<{ draft: null }>("DELETE", draftPath(slug, ref));

// saveDoc saves a document edited from version base (message: the
// commit's subject, if any).
export const saveDoc = (slug: string, d: { ref: string; base: string; text: string; message?: string }) =>
  send<DocSaved>("POST", slug ? `/api/missions/${encodeURIComponent(slug)}/artifact/save` : "/api/docs/save", d);

// DocStage is the stage a document was made for: approving it offers to
// accept the stage, while it's the current one.
export interface DocStage {
  n: number;
  title: string;
  state: "todo" | "current" | "accepted";
}

// Approval is a document approved, at the version it was then.
export interface Approval {
  doc: string;
  version: string;
  by: string;
  at: string;
}

// ActVerb is what the web UI can do about a mission: answer or dismiss its
// question, accept a stage, archive, unpark, and start crew (new, the
// queued crew, a briefing agent, ended crew resumed) or stop them.
export type ActVerb =
  | "answer"
  | "dismiss"
  | "ask-agent"
  | "accept"
  | "look"
  | "archive"
  | "unarchive"
  | "park"
  | "unpark"
  | "launch"
  | "launch-queued"
  | "brief"
  | "confirm"
  | "edit-brief"
  | "edit-plan"
  | "replan"
  | "title"
  | "resume"
  | "stop"
  | "end"
  | "unend"
  | "hand-on"
  | "drop"
  | "open-pr"
  | "bundle-approve"
  | "bundle-send-back"
  | "bundle-looked";

// MissionTask is something a mission waits on you for: a question, or a
// step of its route that's yours (to review, answer, land, or go on from).
export interface MissionTask {
  n: number;
  verb: "answer" | "review" | "approve" | "land" | "do" | "checks" | "time" | "reply";
  text: string;
  step?: number;
  options?: string[];
  by?: string;
  at: string;
}

// ActOpts is what an act says beyond its verb: text; the crew it's to;
// for end, how it ends (none: as suggested), each part of Z met or not
// ("<n>: <how>") and the missions it's handed on to; for hand-on and
// drop, the capsule (repo/name) that ends alone; for open-pr, the capsule
// it's opened from, its title, whether it's a draft, and (text) its
// description.
export interface ActOpts {
  text?: string;
  task?: number; // for answer: the task it answers (its headline's target); none, the oldest
  crew?: string;
  harness?: string; // for launch: what to start crew on, one of the mission's crewPicks (none: the default)
  model?: string;
  ending?: Ending;
  met?: string[];
  unmet?: string[];
  to?: string[];
  capsule?: string;
  title?: string;
  draft?: boolean;
  burnAll?: boolean; // for archive: burn every capsule, even one whose work hasn't merged
  bundle?: string; // for bundle-approve, bundle-send-back and bundle-looked: the bundle (b3)
  notes?: BundleNote[]; // for bundle-send-back: what's not right
  check?: string; // for bundle-looked: the check, by its key in the bundle's list
  looked?: boolean;
}

// act does it, and says what happened.
export const act = (slug: string, verb: ActVerb, opts: ActOpts = {}) =>
  send<{ message: string; path?: string }>("POST", `/api/missions/${encodeURIComponent(slug)}/act`, { verb, ...opts });

// TermScreen is a running crew's screen, as the server emulates it: a line per
// row with its colours as ANSI escapes, the cursor, and the terminal's size.
export interface TermScreen {
  text: string;
  w: number;
  h: number;
  cx: number;
  cy: number;
  cv: boolean;
  title?: string;
}

// TermKey is a key as the browser names it (KeyboardEvent.key), which the
// server types into the crew for the modes it has on.
export interface TermKey {
  key: string;
  ctrl?: boolean;
  alt?: boolean;
  shift?: boolean;
}

const crewPath = (slug: string, id: string) => `/api/missions/${encodeURIComponent(slug)}/crew/${encodeURIComponent(id)}`;

// crewScreenURL is the event stream of a running crew's screen: "screen"
// events (TermScreen), then "exit".
export const crewScreenURL = (slug: string, id: string) => `${crewPath(slug, id)}/screen`;

export const typeIntoCrew = (slug: string, id: string, input: { key: TermKey } | { paste: string }) =>
  send<object>("POST", `${crewPath(slug, id)}/input`, input);

// The orchestrator: how it's set up, and its runs.
export interface OrchestratorInfo {
  enabled: boolean;
  harness: string;
  model?: string;
  problem?: string; // why it can't run as set up
  dispatch: "auto" | "ask";
  maxDispatches: number;
  runs: OrchestratorRun[]; // newest first, across missions
}

export interface OrchestratorRun {
  mission: string;
  missionTitle: string;
  id: string;
  started: string;
  ended: string;
  for: string; // what it looked at: "c2 (done)"
  trigger: string;
  harness: string;
  model?: string;
  retry?: boolean; // run again after a run that did nothing
  acted: boolean; // it ran a ws mission command
  error?: string;
  said?: string;
  calls: number;
  refused: number;
}

export interface OrchestratorCall {
  tool: string;
  input: string;
  result?: string;
  error?: boolean;
  refused?: boolean;
}

export interface OrchestratorRunDetail extends OrchestratorRun {
  prompt: string;
  output: string;
  log: OrchestratorCall[];
}

export const getOrchestrator = () => getJSON<OrchestratorInfo>("/api/orchestrator");

// Settings: yours, in ws.local.toml, each with where its value comes from
// (ws's default, the team's ws.toml, or yours), and what each role runs.
export interface SettingChoice {
  value: string;
  label?: string;
  note?: string; // why it may not work here: a harness that isn't installed
}

export interface Setting {
  section: string; // "" for a key at the top
  key: string;
  label: string;
  about?: string;
  kind: "harness" | "model" | "choice" | "bool" | "number" | "text" | "picks" | "color" | "places"; // picks: a list of harness:model; color: a hex colour, #rrggbb; places: the sidebar's, by id (shell/places.ts)
  choices?: SettingChoice[];
  value: unknown;
  source: "default" | "team" | "yours";
  team?: unknown;
  yours?: unknown;
  default?: unknown;
  harness?: string; // a model's: the harness whose models to offer
}

export interface RoleRun {
  harness: string;
  model?: string;
  problem?: string;
  note?: string;
}

export interface SettingsGroup {
  id: string;
  title: string;
  about?: string;
  settings: Setting[];
  runs?: RoleRun;
}

export interface SettingsInfo {
  file: string;
  team: string;
  groups: SettingsGroup[];
  unknown?: string[];
}

export interface SettingChange {
  section: string;
  key: string;
  value?: unknown;
  clear?: boolean;
}

export interface ModelInfo {
  id: string;
  name?: string;
  about?: string;
  default?: boolean;
}

export const getSettings = () => getJSON<SettingsInfo>("/api/settings");
export const setSettings = (changes: SettingChange[]) => send<SettingsInfo>("POST", "/api/settings", { changes });

// FocusPin is a pin: its name, as ws writes it (a ref is read back from
// it), and what the graph says of it, where it says: what it is (mission,
// talk, pull-request, thing…), what it's called, how it stands. Mirrors
// internal/web/api.go.
export interface FocusPin {
  name: string;
  kind?: string;
  title?: string;
  state?: string;
}
export interface FocusDone {
  said: string;
  focus: FocusPin[];
}
// Pin, unpin or put your pins in order: one at a time.
export const changeFocus = (change: { pin: string } | { unpin: string } | { order: string[] }) => send<FocusDone>("POST", "/api/focus", change);
// The workspace's process (PROCESS.md: how work goes here, in prose), as
// it's written. The planner and the orchestrator read it as it is.
export interface ProcessInfo {
  file: string;
  name: string; // as it reads from the workspace's root
  exists: boolean;
  text: string;
  commit?: string; // the config repo's last commit of it
  uncommitted?: boolean; // the file isn't as committed
}

export const getProcess = () => getJSON<ProcessInfo>("/api/process");
export const getModels = (harness: string) => getJSON<ModelInfo[]>(`/api/settings/models?harness=${encodeURIComponent(harness)}`);
export const getOrchestratorRun = (slug: string, id: string) =>
  getJSON<OrchestratorRunDetail>(`/api/missions/${encodeURIComponent(slug)}/orchestrator/${encodeURIComponent(id)}`);

export const getMission = (slug: string, github: boolean) =>
  getJSON<MissionDetail>(`/api/missions/${encodeURIComponent(slug)}${github ? "?github=1" : ""}`);

export const getArtifact = (slug: string, ref: string) =>
  getJSON<ArtifactContent>(`/api/missions/${encodeURIComponent(slug)}/artifact?${new URLSearchParams({ ref })}`);

// getDoc is a document outside a mission: a file in a repo, by its ref
// (repo:path on the default branch, repo/capsule:path in a capsule).
export const getDoc = (ref: string) => getJSON<ArtifactContent>(`/api/docs?${new URLSearchParams({ ref })}`);

// docRef names a file in a repo's ground or capsule, as a document.
export const docRef = (repo: string, worktree: string, path: string) => `${repo}${worktree === GROUND ? "" : `/${worktree}`}:${path}`;

// docTo is the page that opens a file as a document, to comment on.
export const docTo = (ref: string) => docAddress(ref);

// docAddress is a document's one address: a file in a repo at
// /documents/<repo>/<path>, as it is in a capsule with ?in=<capsule>,
// edited there with ?edit (as the page tied to it: its id, when it's
// known, ?edit=p8). Any repo's markdown has one; a space's is also listed
// in Documents.
export function docAddress(ref: string, edit: boolean | string = false): string {
  const at = ref.indexOf(":");
  const [repo, capsule] = ref.slice(0, at).split("/");
  const path = ref.slice(at + 1);
  const q: string[] = [];
  if (capsule) q.push(`in=${encodeURIComponent(capsule)}`);
  if (edit) q.push(typeof edit === "string" ? `edit=${encodeURIComponent(edit)}` : "edit");
  return `/documents/${encodeURIComponent(repo)}/${path.split("/").map(encodeURIComponent).join("/")}${q.length ? `?${q.join("&")}` : ""}`;
}

// pageAddress is where a page is: the document it's tied to, edited there;
// else Local's.
export const pageAddress = (p: { id: string; source?: { ref: string } }) => (p.source ? docAddress(p.source.ref, p.id) : `/documents/local/${encodeURIComponent(p.id)}`);

// SpaceRef is a space of documents as the workspace says it: what a link
// to one of its documents needs to know.
export interface SpaceRef {
  repo: string;
  folder?: string;
  label: string;
  color: string;
}

// spaceAt is the space a file is in, if it's one of a space's documents.
export const spaceAt = (spaces: SpaceRef[] | undefined, repo: string, path: string) =>
  spaces?.find((s) => s.repo === repo && path.startsWith(s.folder ?? "") && /\.(md|markdown|mdx)$/i.test(path));

// spaceTo is a space's page, at one of its lists if one's named: a kind's
// (kind=adr), or a folder's, by its path in the repo with its slash
// (folder=guides/), as the reader's crumbs open it there.
export const spaceTo = (repo: string, at?: { kind: string } | { folder: string }) =>
  `/documents/${encodeURIComponent(repo)}${at ? `#${new URLSearchParams(at)}` : ""}`;

// Threads on a document: mirrors internal/web/threads.go and
// internal/conversation.

export const threadKinds = ["ask", "change", "note"] as const;
export type ThreadKind = (typeof threadKinds)[number];

export interface ThreadTurn {
  n: number;
  by: string; // "you", a crew id, "agent", or "ws" (saying the agent couldn't answer)
  at: string;
  body: string;
  version?: string; // the document's version when it was said
  commit?: string;
  changed?: boolean; // crew say they've changed the document for it
  refs?: Pinned[]; // what its names were when it was said
}

export interface Thread {
  id: string;
  kind: ThreadKind;
  anchor: { doc: string; path: string; quote?: DocQuote; line: number; endLine: number; version: string; excerpt?: string[] };
  by: string;
  at: string;
  status: "open" | "resolved";
  statusBy?: string;
  statusAt?: string;
  turns: ThreadTurn[];
  answering?: boolean; // an agent is answering it now
  suggestions?: Suggestion[]; // oldest first: at most one open, the latest
  // sent: handed to crew to change the document for, until they say
  // they have; finished, they've ended or reported done since without.
  sent?: { crew: string; by: string; at: string; finished?: boolean };
  // draft: what was decided, drafted by its agent once you resolved it,
  // for you to record (as decision) or skip.
  draft?: { what: string; why?: string; by: string; at: string; state: "open" | "recorded" | "skipped"; decision?: string };
}

// draftWaiting: a resolved thread's drafted decision, for you to record or skip.
export const draftWaiting = (t: Thread) => t.draft?.state === "open";

// Suggestion is an edit suggested in a thread: from, the document's
// markdown as it is, to what it should read.
export interface Suggestion {
  n: number;
  from: string;
  to: string;
  by: string;
  at: string;
  state: "open" | "accepted" | "declined" | "superseded";
  commit?: string;
}

export const pendingSuggestion = (t: Thread) => [...(t.suggestions ?? [])].reverse().find((s) => s.state === "open");

// withCrew: crew have the thread, to change the document for it.
export const withCrew = (t: Thread) => !!t.sent && !t.sent.finished;

// waitingOnYou: someone's answered an open thread, and it's your turn.
export const waitingOnYou = (t: Thread) =>
  t.status === "open" &&
  !t.answering &&
  !withCrew(t) &&
  ((t.turns.length > 1 && t.turns[t.turns.length - 1].by !== "you") || !!t.sent?.finished);

export interface ThreadState {
  version: string; // the document's now
  threads: Thread[]; // oldest first
  message?: string; // what a write did, when it's worth saying
  approved?: Approval; // your latest approval, at the version it was then
  stage?: DocStage;
}

// A document's threads are its mission's, or (no slug) the workspace's
// own, for a file in a repo that's in no mission.
const threadsPath = (slug: string, ref: string, rest = "") =>
  `${slug ? `/api/missions/${encodeURIComponent(slug)}` : "/api/docs"}/threads${rest}?${new URLSearchParams({ ref })}`;

export const getThreads = (slug: string, ref: string) => getJSON<ThreadState>(threadsPath(slug, ref));

export const openThread = (
  slug: string,
  ref: string,
  // from, with suggestion: the markdown it replaces, when that isn't the
  // quote as it reads (an edit made in the editor, whole lines of it).
  t: { version: string; line: number; endLine: number; quote: DocQuote; kind: ThreadKind; body: string; suggestion?: string; from?: string },
) => send<ThreadState>("POST", threadsPath(slug, ref), t);

// settleSuggestion accepts (writing it in) or declines a thread's
// suggested edit.
export const settleSuggestion = (slug: string, ref: string, id: string, action: "accept" | "decline") =>
  send<ThreadState>("POST", threadsPath(slug, ref, `/${encodeURIComponent(id)}/suggestion`), { action });

export const sayOnThread = (slug: string, ref: string, id: string, body: string) =>
  send<ThreadState>("POST", threadsPath(slug, ref, `/${encodeURIComponent(id)}/say`), { body });

export const setThreadStatus = (slug: string, ref: string, id: string, status: "open" | "resolved") =>
  send<ThreadState>("POST", threadsPath(slug, ref, `/${encodeURIComponent(id)}/status`), { status });

// approveDoc approves the document as you read it (its version). It's
// refused while a change asked for on it is open.
export const approveDoc = (slug: string, ref: string, version: string) => send<ThreadState>("POST", threadsPath(slug, ref, "/approve"), { version });

// sendThreads hands threads to crew to change the document for: those
// named, or none, every change asked for that's open and not with crew.
export const sendThreads = (slug: string, ref: string, ids: string[]) => send<ThreadState>("POST", threadsPath(slug, ref, "/send"), { ids });

// recordDecision records what was decided in a thread (from the document,
// and its stage), or skips its draft.
export const recordDecision = (slug: string, ref: string, id: string, d: { action: "record"; what: string; why: string } | { action: "skip" }) =>
  send<ThreadState>("POST", threadsPath(slug, ref, `/${encodeURIComponent(id)}/decision`), d);

// getThreadVersion is the document's text at a version a thread was on.
export const getThreadVersion = (slug: string, ref: string, v: string) =>
  getJSON<{ text: string }>(`${threadsPath(slug, ref, "/version")}&${new URLSearchParams({ v })}`).then((x) => x.text);

// Conversations with an agent about the workspace (w1, w2, …): mirrors
// internal/web/talk.go.

export interface Talk {
  id: string;
  title?: string;
  opening: string; // its first message, clipped
  summary?: string;
  status: "open" | "resolved";
  answering?: boolean;
  waiting?: boolean; // the agent replied: your turn
  became?: string; // the mission it became
  proposed?: number; // proposals waiting for you
  pr?: string; // the pull request it's about (api#89)
  doc?: string; // the document it's about, an ask kept (records:notes/x.md)
  section?: string; // the section it was asked of, if a passage was
  quote?: string; // the passage, clipped
  at: string;
  updated: string;
}

// TalkProposal is something the agent proposed: a decision, a mission
// (with a brief, ready; without, an idea to brief later), an edit to the
// brief, crew, a replan by the mission's planner, a review comment, a
// document of one of the workspace's kinds, or an artifact (or a version
// of one).
export interface TalkProposal {
  n: number;
  kind: "decision" | "mission" | "brief" | "dispatch" | "replan" | "comment" | "plan" | "document" | "artifact" | "plugin";
  by: string;
  at: string;
  what?: string;
  why?: string;
  title?: string;
  brief?: string;
  capsules?: string[];
  about?: string[]; // a mission: what it's about, by names ws reads (RAM-123, api#12)
  docks?: string[]; // briefing: existing work to build on (repo/branch, repo#PR)
  slug?: string; // briefing: a better slug
  was?: string; // an edit to the brief: the brief it replaces (brief is the new one); to a document, the document as it was read
  text?: string; // a dispatch: what the crew's to do; a replan: what to change in the plan
  capsule?: string;
  reads?: string[];
  // a review comment: the pull request, its lines, its kind and (text)
  // what it says, with a suggested replacement for the lines.
  pr?: string;
  path?: string;
  line?: number;
  endLine?: number;
  side?: "new" | "old";
  noteKind?: NoteKind;
  suggestion?: string | null;
  excerpt?: string[];
  // an edit to the mission's plan: the plan as it should read (was is as
  // it read); briefing, the brief comes with its plan (planWas as it read).
  plan?: string;
  planWas?: string;
  // a revision of the plan the planner proposed because it's yours to
  // confirm (why says what makes it yours), and the planner's step it was
  // written at: accepting it passes that step.
  step?: number;
  // a document: its kind (by name), (text) the whole of it, and the guide
  // it was written to (the kind's, which the agent was shown first); as it
  // was proposed, its title, where it would go (path, repo:path) and the
  // commit's first line (subject). An edit to one names it (to, its ref),
  // with the whole of it as the agent read it (was): text is as it should
  // read, and subject the commit's that changes it.
  docKind?: string;
  guide?: string;
  subject?: string;
  // an artifact: its note, and (title) a new one's title, or the artifact
  // it's a version of (to, a3), the version it's based on (on) and the
  // artifact's latest version as it was proposed (latest); as proposed,
  // where it would go (path), its size, and what in it won't load in the
  // frame. Its HTML isn't sent: it runs from proposalFrameURL. files:
  // what it's made of, for one that's more than its page.
  note?: string;
  files?: ArtifactFile[];
  to?: string;
  on?: number;
  latest?: number;
  size?: number;
  warnings?: string[];
  // a plugin's: the plugin, which of what it proposes (its id), what the
  // card calls that (label) and its icon; title is the plugin's line for
  // the whole, and items what it'll do, each accepted on its own or with
  // the rest. Result, settled, is what the items made, by name.
  plugin?: string;
  proposes?: string;
  label?: string;
  icon?: string;
  items?: PluginProposalItem[];
  revises?: number; // the agent's revision of an open proposal of its, by number: that one's superseded by this
  state: "open" | "accepted" | "declined" | "superseded" | "withdrawn"; // superseded: a later one took its place; withdrawn: the agent took it back
  because?: string; // withdrawn: why
  result?: string; // a mission's slug, slug:d3, a document's ref (an edit's: where it was saved), or an artifact's version (a3.v1); "" for a decision kept with the conversation; a superseded one's, the later one's number
}

// PluginProposalItem is one thing a plugin's proposal does, as the plugin
// drafted it: what it's called, the thing it changes (on), what it'll be
// row by row (was, for a change) and at length (markdown); and what came
// of it: made (with what, by name), left (not accepted with the rest), or
// open, with why it wasn't done the last time it was accepted (problem).
export interface PluginProposalItem {
  title: string;
  on?: string;
  rows?: { label: string; value: string; was?: string }[];
  markdown?: string;
  state?: "made" | "left";
  made?: string;
  say?: string;
  problem?: string;
}

// TalkCall is a tool call the agent made on the way to a reply.
export interface TalkCall {
  tool: string;
  input: string;
  result?: string;
  error?: boolean;
  refused?: boolean;
}

export interface TalkTurn {
  n: number;
  by: string; // "you", "agent", or "ws" (saying the agent couldn't answer)
  at: string;
  body: string;
  calls?: TalkCall[];
  edited?: string; // a cell as you'd edited it by hand, when you asked for a change
  notes?: string; // what a cell's agent had to tell you about a version: beside the cell, never in it
  ask?: boolean; // yours, in a cell: a question, to be answered and not written into the page
  answer?: boolean; // the agent's: its answer to one, not a version of the cell
  refs?: Pinned[]; // what its names were when it was said
  // origin: what ws wrote a turn of yours for, where you didn't type it
  // ("proposal:3": your settling that card). The agent read it as yours;
  // it's drawn as what happened, never as your words. quiet: one the
  // agent wasn't woken for, which doesn't change whose turn it is.
  origin?: string;
  quiet?: boolean;
  // given: on a turn of yours said on a plugin's page, what the page gave
  // the agent with it. Not your words: folded under them.
  given?: TalkGiven;
}

// TalkGiven is what a plugin's page gave a conversation's agent with
// something said there (conversation.Given): its state as words (page,
// only when it's changed since the agent last had it), what was held,
// and context for the message alone.
export interface TalkGiven {
  from?: string;
  page?: string;
  holding?: string;
  holdingText?: string;
  context?: string;
}

export interface TalkDetail {
  id: string;
  kind: string;
  goal?: string; // what it's for, when it's for something: "brief" is its mission's planner's (briefing it, then carrying on)
  what?: string; // what it is, as it's answered: "talk", "pr", "doc" (an ask about a document), …
  // anchor: what it's about, when it's about a pull request (and lines of
  // its diff), or a document (and a passage of it: its quote, the section
  // it's in, and the lines it was found on).
  anchor?: {
    pr?: string;
    doc?: string;
    path?: string;
    side?: "new" | "old";
    line?: number;
    endLine?: number;
    commit?: string;
    excerpt?: string[];
    quote?: DocQuote;
    section?: string;
    thing?: string; // a plugin's thing, asked about from its page
    plugin?: string;
  };
  via?: string; // the plugin a talk was started from, which worded its first message
  ephemeral?: boolean; // an ask nobody's kept: not listed, and swept an hour after its last turn
  kept?: string; // when an ask was kept: a conversation like any since
  title?: string;
  summary?: string;
  by: string;
  at: string;
  status: "open" | "resolved";
  turns: TalkTurn[];
  answering?: boolean;
  // run: the agent answering now, as far as it's got.
  run?: { calls: TalkCall[]; said?: string };
  proposals?: TalkProposal[];
  // The capsules its agent lifted, to try things in: the conversation's
  // own, put away with it (away: when, their branches kept).
  capsules?: TalkCapsule[];
  message?: string; // what a write did (accepting a proposal)
  path?: string; // where the conversation is now, when it's moved (its mission renamed)
}

// A conversation is the workspace's (slug "") or a mission's (its Talk tab).
const talkPath = (slug: string, id = "", rest = "") =>
  `/api${slug ? `/missions/${encodeURIComponent(slug)}` : ""}/conversations${id ? `/${encodeURIComponent(id)}` : ""}${rest}`;

export const getTalk = (slug: string, id: string) => getJSON<TalkDetail>(talkPath(slug, id));
export const startTalk = (slug: string, body: string) => send<TalkDetail>("POST", talkPath(slug), { body });
export const sayInTalk = (slug: string, id: string, body: string, given?: TalkGiven) =>
  send<TalkDetail>("POST", talkPath(slug, id, "/say"), given ? { body, given } : { body });
export const setTalkStatus = (slug: string, id: string, status: "open" | "resolved") => send<TalkDetail>("POST", talkPath(slug, id, "/status"), { status });

// Asking from an ask bar (ask/AskBar.tsx): mirrors internal/web/asks.go. An
// ask is the workspace's conversation from the start, about a document
// (its ref) and a passage of it as it reads (its quote, and the heading of
// the section it's in); followed with getTalk and said to again with
// sayInTalk, and kept only if you keep it.
export const startAsk = (q: { doc: string; quote?: DocQuote; section?: string; body: string }) => send<TalkDetail>("POST", "/api/asks", q);
// askThing asks about a plugin's thing (sketch-13) from its page, with
// what the page gives the agent.
export const askThing = (q: { thing: string; body: string; given?: TalkGiven }) => send<TalkDetail>("POST", "/api/asks", q);
// keepAsk keeps one (Keep as a conversation): it's listed in Conversations
// from then on, about its document, and isn't swept.
export const keepAsk = (slug: string, id: string) => send<TalkDetail>("POST", talkPath(slug, id, "/keep"));

// Pages: documents you write here, with cells an agent fills in. Mirrors
// internal/web/pages.go.

export interface PageInfo {
  id: string;
  title: string;
  updated: string; // when it, or one of its cells, last changed
  cells?: number;
  filling?: number; // cells an agent's filling now
  stale?: number; // cells written from a file that's changed since
  findings?: number; // its passes' findings still open
  passing?: number; // passes running now
  kind?: string; // the kind of document it's written as, if it was started as one
  filed?: PageFiled; // where it was filed as a document, once it was
  source?: PageSource; // the file in a repo it's tied to, if it is: saving writes there
  to?: string; // where it's to be filed, of no kind: repo:folder/
  unsaved?: boolean; // tied to a file, with something to save there
}

// PageSource is the file a page is tied to: its ref, and the version of
// it the page last agreed with.
export interface PageSource {
  ref: string;
  version: string;
  at: string;
}

// PageTied is where a page tied to a file stands with it: the file, whether
// it's changed since the page last agreed with it (bring that in before
// saving), or isn't there any more (gone says why), and how saving writes.
export interface PageTied {
  ref: string;
  in?: DocIn;
  changed?: boolean;
  gone?: string;
  saves?: DocSaves;
}

// PageFiled is where a page was filed: its document's ref (repo:path, or
// repo/capsule:path where it was left in a capsule), of what kind, the
// commit that added it, whether that landed, when, and the version of the
// page it was filed from.
export interface PageFiled {
  ref: string;
  kind: string;
  commit?: string;
  landed?: boolean;
  at: string;
  version: string;
}

// PageFiling is what filing a page would do, or did: where it goes, the
// commit's first line, how it's saved (land: straight to branch, once
// verify passes; lift: in a capsule lifted for it, left to you), whether
// there's a file there already, and, done, the commit and what happened.
export interface PageFiling {
  kind: string;
  title: string;
  ref: string;
  subject: string;
  how: "land" | "lift";
  branch: string;
  verify?: string;
  taken?: boolean;
  commit?: string;
  message?: string;
}

// CellSource is a file a cell was written from: in one of the workspace's
// repos, its repo, worktree (".ground", or a capsule) and file there.
// state says whether it's changed since ("" for as it was).
export interface CellSource {
  path: string;
  repo?: string;
  worktree?: string;
  file?: string;
  state?: "changed" | "gone";
}

// PageCell is a cell of a page: its conversation (what you asked for, each
// version the agent wrote, what you asked to change), and while the agent's
// filling it, what it's done so far.
export interface PageCell {
  id: string;
  kind: "cell";
  anchor: { page: string; finding?: string }; // finding: the pass's finding it's a question about
  at: string;
  status: "open" | "resolved";
  turns: TalkTurn[];
  answering?: boolean;
  run?: { calls: TalkCall[] };
  sources?: CellSource[]; // what it was written from, as it now reads
  goal?: string; // what it's for: ask (a question), plan (asked of the page as a whole)
  // An outline: asked for more than a block, a cell's agent plans the
  // blocks first. blocks are what it planned, in order; placed is when the
  // page put them in. Each block is then a cell of its own (id w12-b1),
  // with of naming the cell that planned it, and queued while it waits its
  // turn to be written.
  blocks?: PlannedBlock[];
  placed?: string;
  of?: string;
  queued?: boolean;
  // moved: what it was written beside has changed since: a cell above it,
  // or (for one with a goal) the page around it.
  moved?: "above" | "page";
}

// PlannedBlock is a block of an outline: where it goes (the words of a
// heading already in the page, or here, start or end), the heading it goes
// under, and a sentence on what it'll say.
export interface PlannedBlock {
  key: string;
  after?: string;
  heading?: string;
  level?: number;
  summary: string;
}

export interface PageDetail extends PageInfo {
  text: string;
  version: string; // names text: a save says which version it was edited from
  cell: PageCell[];
  tied?: PageTied; // for a page tied to a file, where it stands with it
}

const pagePath = (id = "", rest = "") => `/api/pages${id ? `/${encodeURIComponent(id)}` : ""}${rest}`;
const cellPath = (id: string, cell: string, rest: string) => pagePath(id, `/cells/${encodeURIComponent(cell)}${rest}`);

// newPage starts a page: titled, of a kind, or (of no kind) to be filed in
// a folder of a repo (to: repo:folder/).
export const newPage = (title = "", kind = "", to = "") => send<PageDetail>("POST", pagePath(), { title, kind, ...(to ? { to } : {}) });
// pageOpenTo is where a file in a repo is edited: at its address, as the
// page tied to it (made the first time), or the mission that records it.
export const pageOpenTo = (ref: string) => docAddress(ref, true);
export const openFilePage = (ref: string) => send<{ id: string; mission?: { slug: string; title: string; ref: string } }>("POST", "/api/pages/open", { ref });
// savePageToFile writes a page tied to a file back to it (message: the
// commit's first line, if you gave one). FileChanged: the file's moved on
// since, and what changed is brought in first (bringIn).
export class FileChanged extends ApiError {}
export async function savePageToFile(id: string, message?: string): Promise<DocSaved> {
  try {
    return await send<DocSaved>("POST", pagePath(id, "/save"), { message });
  } catch (e) {
    if (e instanceof ApiError && e.status === 409) throw new FileChanged(e.status, e.code, e.message);
    throw e;
  }
}
export const bringIn = (id: string) => send<{ changed: boolean; clashes: number }>("POST", pagePath(id, "/bring-in"), {});
// filePage files a page as a document of one of the workspace's kinds
// (title: what it's named from; message: the commit's first line), or,
// dry, says what that would do.
// filePage files a page: as a kind of document, or (kind "") where to says,
// a folder of a repo (or a file in one), or where the page says.
export const filePage = (id: string, r: { kind: string; to?: string; title?: string; message?: string; dry?: boolean }) => send<PageFiling>("POST", pagePath(id, "/file"), r);
export const getPage = (id: string) => getJSON<PageDetail>(pagePath(id));
export const removePage = (id: string) => send<{ removed: boolean }>("DELETE", pagePath(id));
export const getPageCells = (id: string) => getJSON<{ cell: PageCell[] }>(pagePath(id, "/cells")).then((r) => r.cell);

// PageChanged is a save refused: the page has changed since the version
// the edit started from.
export class PageChanged extends ApiError {}

// savePage saves a page's text, edited from the version base.
export async function savePage(id: string, text: string, base: string, opts: { keepalive?: boolean } = {}): Promise<{ version: string; title: string }> {
  try {
    return await request("PUT", pagePath(id), { body: { text, base }, keepalive: opts.keepalive });
  } catch (e) {
    if (e instanceof ApiError && e.status === 409) throw new PageChanged(e.status, e.code, e.message);
    throw e;
  }
}

// A cell is asked for (it's numbered at once), then filled once the page
// has saved the text it's in; saying something in it has it written again.
// holds is what the cell's made around, when that's words of yours to be
// changed.
// ask: it's a question (>?), answered beside the page. finding: it's a
// question about a pass's finding, with no place in the page.
export const openCell = (id: string, prompt: string, opts: { goal?: string; holds?: string; ask?: boolean; finding?: string } = {}) =>
  send<PageCell>("POST", pagePath(id, "/cells"), { prompt, goal: opts.goal ?? "", holds: opts.holds ?? "", ask: !!opts.ask, finding: opts.finding ?? "" });
export const fillCell = (id: string, cell: string) => send<PageCell>("POST", cellPath(id, cell, "/fill"));
// The page has put an outline's blocks in: they're placed once.
export const markPlaced = (id: string, cell: string) => send<{ placed: boolean }>("POST", cellPath(id, cell, "/placed"));
// edited is the cell as it reads in the page, when that's not as its agent
// last wrote it.
// ask: it's a question about the cell, to be answered, not a change to it.
export const tweakCell = (id: string, cell: string, body: string, edited?: string, ask = false) =>
  send<PageCell>("POST", cellPath(id, cell, "/say"), { body, edited: edited ?? "", ask });

// A pass over a page: it reads the whole page and says things about it,
// kept beside the page and never written into it. Mirrors PagePass in
// internal/web/pages.go.
export interface PassFinding {
  key: string; // the same from one run to the next, for the same kind on the same words
  kind: string;
  quote?: string; // the words of the page it's about; none for something missing
  text: string;
  suggest?: string; // what those words should read
  state: "open" | "done" | "dismissed";
  why?: DismissWhy; // why it was dismissed, where you said
  // A review's finding is on lines of the diff, not words of a page: with
  // no path, it's about the change as a whole.
  path?: string;
  side?: "new" | "old";
  line?: number;
  endLine?: number;
  commit?: string; // the commit those lines are at
  excerpt?: string[]; // how they read
  blocks?: boolean; // of a kind that makes its pass no go while it's open
  // thread: a follow-up's finding is about a comment of yours (its thread
  // on GitHub): whether what's been pushed since deals with it.
  thread?: string;
}

// Why a finding was dismissed: how often each is how well a pass does
// (ws show passes).
export type DismissWhy = "wrong" | "already" | "minor";
export const dismissWhys: [DismissWhy, string, string][] = [
  ["wrong", "Wrong", "It isn't so"],
  ["already", "Already said", "The page says it already"],
  ["minor", "Not worth it", "It's so, but not worth acting on"],
];

export interface PagePass {
  name: string;
  title: string;
  about: string;
  kinds?: string[]; // what its findings can be; none for a skill's, which names its own
  cold?: boolean; // its agent gets the page and nothing else
  skill?: string; // one of the workspace's skills, which its agent follows…
  args?: string; // …for this
  unavailable?: string; // why it can't be run, if it can't
  id?: string; // its conversation, once run
  running?: boolean;
  run?: { calls: TalkCall[] };
  runs?: number;
  at?: string; // when it last said something
  summary?: string;
  findings: PassFinding[];
  failed?: string; // why its latest run couldn't
  moved?: boolean; // the page has changed since it last ran
}

const passPath = (id: string, pass: string, rest: string) => pagePath(id, `/passes/${encodeURIComponent(pass)}${rest}`);

export const getPagePasses = (id: string) => getJSON<{ passes: PagePass[] }>(pagePath(id, "/passes")).then((r) => r.passes);
export const runPass = (id: string, pass: string) => send<PagePass>("POST", passPath(id, pass, "/run"));
// settleFinding marks a pass's finding done or dismissed (why: it was
// wrong, the page already said it, or it's so but not worth acting on), or
// opens it again.
export const settleFinding = (id: string, pass: string, key: string, state: PassFinding["state"], why?: DismissWhy) =>
  send<PagePass>("POST", passPath(id, pass, `/findings/${encodeURIComponent(key)}`), { state, why });

// settleTalkProposal accepts (with your edits) or declines a proposal.
export const settleTalkProposal = (
  slug: string,
  id: string,
  n: number,
  action: "accept" | "decline",
  edits: { what?: string; why?: string; title?: string; brief?: string; text?: string; plan?: string; note?: string; items?: number[] } = {},
) => send<TalkDetail>("POST", talkPath(slug, id, `/proposals/${n}`), { action, ...edits });

// A crew's transcript: its session as its harness keeps it (Claude Code's
// and pi's), for reading beside its terminal or once it's ended.
export interface TranscriptEntry {
  by: "user" | "agent" | "call";
  at?: string;
  text?: string;
  id?: string;
  tool?: string;
  input?: string;
  result?: string;
  error?: boolean;
  refused?: boolean;
  pending?: boolean; // nothing back yet: the step in hand, or waiting for permission
  edit?: { path: string; old?: string; new: string; all?: boolean };
  todos?: { content: string; status: "pending" | "in_progress" | "completed"; active?: string }[];
  questions?: { question: string; header?: string; options?: string[]; multi?: boolean }[];
}

export interface Transcript {
  harness: string;
  available: boolean;
  why?: string;
  entries: TranscriptEntry[];
  earlier?: number;
}

export const getTranscript = (slug: string, id: string) =>
  getJSON<Transcript>(`/api/missions/${encodeURIComponent(slug)}/crew/${encodeURIComponent(id)}/transcript`);

// transcriptStreamURL is the event stream of a crew's transcript: it, then
// again each time its session changes ("transcript" events).
export const transcriptStreamURL = (slug: string, id: string) =>
  `/api/missions/${encodeURIComponent(slug)}/crew/${encodeURIComponent(id)}/transcript/stream`;

// sayToCrew sends crew a message: typed in if it's running, or resuming it
// with it if it's ended.
export const sayToCrew = (slug: string, id: string, text: string) =>
  send<{ message: string }>("POST", `/api/missions/${encodeURIComponent(slug)}/crew/${encodeURIComponent(id)}/say`, { text });

// CrewAnswer answers what crew's screen is asking, or interrupts its turn.
// It's refused (409) when the screen isn't showing that prompt.
export type CrewAnswer = { permission: "allow" | "always" | "deny" } | { option: string } | { text: string } | { interrupt: true };

export const answerCrew = (slug: string, id: string, a: CrewAnswer) =>
  send<{ message: string }>("POST", `/api/missions/${encodeURIComponent(slug)}/crew/${encodeURIComponent(id)}/answer`, a);

// A file that differs from a capsule's branch point, committed or not,
// without its lines. Mirrors review.Changed.
export interface ChangedFile {
  path: string;
  oldPath?: string;
  status: string; // added, deleted, modified, renamed, copied or type
  additions: number;
  deletions: number;
  skim?: string; // why it needn't be read closely: a lockfile, generated, a snapshot, renamed
  uncommitted?: boolean; // it has changes that aren't committed, which a review doesn't show
}

// A capsule in brief (the Work tab's row and pane): what's changed since
// its branch point, committed, its latest commits, how many files aren't
// committed, and the files that differ as it is now. Mirrors
// review.Summary.
export interface CapsuleSummary {
  baseRef: string;
  base?: string;
  commits: Commit[]; // the latest, newest first
  commitCount: number;
  files: number;
  additions: number;
  deletions: number;
  uncommitted: number;
  changed: ChangedFile[]; // committed or not
  changedMore?: number; // how many more there are than it lists
}

export const getSummary = (api: string) => getJSON<CapsuleSummary>(`${api}/summary`);

// A capsule's files, for the file explorer: mirrors review.Tree and
// review.FileView.
export interface TreeFile {
  path: string;
  oldPath?: string;
  status?: "added" | "modified" | "deleted" | "renamed" | "copied" | "type"; // since the branch point; absent: unchanged
  uncommitted?: boolean;
}

export interface FileTree {
  baseRef: string;
  base?: string;
  head?: string;
  branch?: string;
  files: TreeFile[];
  truncated?: boolean;
}

export interface LineChange {
  kind: "added" | "modified" | "deleted";
  start: number;
  end?: number;
}

export interface FileView {
  path: string;
  size: number;
  binary?: boolean;
  tooLarge?: boolean;
  lines: string[];
  deleted?: boolean;
  changes: LineChange[];
}

export const getTree = (api: string) => getJSON<FileTree>(`${api}/tree`);
export const getFile = (api: string, path: string) => getJSON<FileView>(`${api}/file?${new URLSearchParams({ path })}`);
export const rawURL = (api: string, path: string) => `${api}/raw?${new URLSearchParams({ path })}`;

// CrewPrompt is the menu on a running crew's screen, as the server reads
// it: what it asks, what about (the command, the file), and its options.
export interface CrewPrompt {
  title: string;
  context: string[];
  options: string[];
}

export const getCrewPrompt = (slug: string, id: string) =>
  getJSON<CrewPrompt>(`/api/missions/${encodeURIComponent(slug)}/crew/${encodeURIComponent(id)}/prompt`);

// The Reviews page: open pull requests across the workspace's repos.
// Mirrors internal/pulls and internal/web/reviews.go.

export type PRState =
  | "needs-review"
  | "back-with-you"
  | "waiting-on-them"
  | "approved"
  | "others"
  | "drafts"
  | "automated"
  | "returned"
  | "ready"
  | "waiting-review";

export interface PR {
  repo: string;
  number: number;
  title: string;
  author: string;
  url: string;
  branch: string;
  base: string; // the branch it's for: a stack's layer is for the layer below
  head?: string;
  draft?: boolean;
  bot?: boolean;
  fork?: boolean;
  mine?: boolean;
  additions: number;
  deletions: number;
  files: number;
  checks?: "success" | "failure" | "pending";
  decision?: string;
  created?: string;
  updated?: string;
  state: PRState; // the section it's in
  words: string; // why, in a few words
  asked?: "you" | "team";
  reviewed?: { state: string; at?: string; commit?: string; behind?: boolean };
  mission?: { slug: string; title: string };
  capsule?: string; // the capsule its branch is checked out in here
  onTopOf?: number;
}

// Stack is a stack of pull requests: one row in the list and one review,
// with its layers (bottom first) inside.
export interface Stack {
  repo: string;
  number?: number; // GitHub's; absent for one ws worked out
  workedOut?: boolean;
  title: string;
  author: string;
  mine?: boolean;
  base: string;
  layers: PR[];
  merged?: number;
  additions: number;
  deletions: number;
  state: PRState;
  words: string;
  open: number; // the layer to open: the lowest that needs you
  onTopOf?: number;
}

export interface ReviewRow {
  key: string;
  pr?: PR;
  stack?: Stack;
}

export interface ReviewSection {
  key: PRState;
  title: string;
  yours?: boolean;
  folded?: boolean;
  rows: ReviewRow[];
}

export interface ReviewsList {
  at: string; // when GitHub was last asked
  me: string;
  sections: ReviewSection[]; // those with something in them, theirs first
  count: number;
  problems?: string[];
  stale?: boolean;
  error?: string;
  // passes: how each pass that's been run over a pull request stands
  // there, by pull request (api#89).
  passes?: Record<string, PassMark[]>;
}

// PassMark is how a pass stands over a pull request, in brief.
export interface PassMark {
  name: string;
  title: string;
  standing: { status: "none" | "running" | "failed" | "nogo" | "notes" | "go"; open: number; blocking: number; overruled?: number };
  moved?: boolean; // it's been pushed to since the pass ran
}

export const getReviews = (refresh = false) => getJSON<ReviewsList>(`/api/reviews${refresh ? "?refresh=1" : ""}`);

// The Team page: what the people working in the workspace's repos have
// been doing, by day.

export type TeamEventKind = "opened" | "ready" | "reviewed" | "merged" | "closed" | "landed";
export type StrandState = "in-progress" | "in-review" | "landed" | "closed";

// TeamEvent is one thing that happened: a plain fact, from git or GitHub.
export interface TeamEvent {
  id: string;
  at: string;
  kind: TeamEventKind;
  repo: string;
  pr?: number;
  commit?: string;
  title: string;
  url?: string;
  branch?: string;
  login?: string;
  name?: string;
  email?: string;
  bot?: boolean;
  who?: string; // the key of the person it's by
  verdict?: "approved" | "changes" | "commented";
  additions?: number;
  deletions?: number;
  files?: number;
  paths?: string[];
}

export interface TeamPerson {
  key: string;
  name: string;
  short: string; // what they're called in a sentence
  login?: string;
  bot?: boolean;
}

// TeamItem is a pull request, or one person's commits to one repo, on one
// day: what happened to it then.
export interface TeamItem {
  key: string;
  strand: string;
  kind: "pr" | "commits";
  repo: string;
  pr?: number;
  // For one change made in several repos, a pull request in each: every
  // repo it's in, and its pull requests. repo is then the first.
  repos?: string[];
  prs?: { repo: string; number: number; name: string; url?: string; state: StrandState }[];
  name: string;
  url?: string;
  state: StrandState; // where it stands now
  people: string[]; // keys: whose it is first
  words: string;
  at: string;
  bot?: boolean;
  additions?: number;
  deletions?: number;
  events: TeamEvent[];
  // What an agent wrote of it: what the change does, and what's worth
  // knowing of it. narrative is where that stands, when it isn't written:
  // being written, failed (gistError says why), or not written yet.
  gist?: string;
  know?: Know[];
  narrative?: "writing" | "failed" | "unwritten";
  gistError?: string;
  // What it touches of how things are done: a guide, a skill, CI, lint
  // settings, or a path the workspace watches.
  touches?: { name: string; title: string; files: string[] }[];
  // Capsules of yours (repo/name) that have changed files it changed too,
  // once it's landed: what you'll meet when you next rebase.
  near?: { capsule: string; files: string[] }[];
}

export type KnowKind = "contract" | "action" | "convention" | "removed" | "behaviour";

// Know is one thing worth knowing of a change, for someone who didn't
// write it: what kind of thing, the changed file it rests on, and what.
export interface Know {
  kind: KnowKind;
  in?: string; // the repo the file's in, for a change made in several
  file: string;
  text: string;
}

// Known is something worth knowing, with the row it's of.
export interface Known extends Know {
  strand: string;
  item: string; // the key of its row
  name: string;
  repo: string;
  pr?: number;
  people: string[];
  at: string;
}

export interface TeamDay {
  date: string; // 2026-11-10
  items: TeamItem[];
}

export interface TeamView {
  at: string; // when git and GitHub were last looked at
  since: string;
  days: TeamDay[];
  people: TeamPerson[];
  github: boolean; // pull requests were asked for
  repos: Record<string, { label: string; color?: string }>;
  seen?: string; // when you last looked at the page: what's since is new to you
  problems?: string[];
  stale?: boolean;
  error?: string;
  // The period's account: what the team's been working on, as a few
  // topics, written by an agent when asked. stale: the period has changed
  // since; can: how many changes there are to write it from.
  summary?: { at?: string; topics?: TeamTopic[]; stale?: boolean; writing?: boolean; failed?: string; can: number };
  know?: Known[]; // everything worth knowing in the period, the latest first
  // How what agents write stands: whether one does here (off says why
  // not), how many gists are being written, and how many rows have none.
  narrative?: { on: boolean; off?: string; writing: number; unwritten: number };
}

// narrateTeam asks for gists: of one strand (again, if it failed), or of
// everything in a period that has none.
export const narrateTeam = (what: { strand?: string; since?: string }) => send<{ started: number }>("POST", "/api/team/narrate", what);

// TeamTopic is one thing the team's been working on: what it's called,
// what it comes to, the changes that are part of it and who's on them.
export interface TeamTopic {
  name: string;
  text: string;
  strands: string[];
  people: string[];
}

// summariseTeam asks for a period's account to be written.
export const summariseTeam = (since: string) => send<{ started: boolean }>("POST", "/api/team/summarise", { since });

// seenTeam says you've looked at the page.
export const seenTeam = () => send<{ ok: boolean }>("POST", "/api/team/seen", {});

// shutDownDaemon closes the workspace's daemon (and so this web UI), as
// ws daemon stop does.
export const shutDownDaemon = () => send<{ ok: boolean }>("POST", "/api/shutdown", {});

export const getTeam = (since: string, refresh = false) => getJSON<TeamView>(`/api/team?since=${encodeURIComponent(since)}${refresh ? "&refresh=1" : ""}`);

export function prAPI(repo: string, number: number | string): string {
  return `/api/prs/${encodeURIComponent(repo)}/${number}`;
}

// PRDetail is a pull request in full: what GitHub says of it, its commits
// as fetched, its stack, your last review of it and the conversations
// about it.
export interface PRDetail {
  pr: PR;
  body?: string;
  bodyID?: string; // names the description by what it says
  stack?: Stack;
  layer?: number; // 1 is the bottom
  baseRef: string;
  base: string;
  head: string;
  needsRebase?: boolean;
  reviewed?: { id: string; verdict?: Verdict; head: string; base: string; at: string; url?: string };
  // since: what looking again shows: what's been pushed ("commits"), or,
  // rebased, each file's change against what you reviewed ("changes").
  since?: "commits" | "changes";
  docked?: { capsule: string; branch: string; by: "ws" | "you"; at: string };
  talks: Talk[];
  noAsk?: string; // why an agent can't be asked about it
}

export const getPR = (repo: string, number: number | string) => getJSON<PRDetail>(prAPI(repo, number));

// PRDiff is a pull request's diff, by scope. Rebased, a since diff is the
// whole layer, and unchanged are the files whose change is as you
// reviewed it.
export interface PRDiff extends Diff {
  scope: "layer" | "since" | "whole";
  reviewed?: string;
  rebased?: boolean;
  unchanged?: string[];
}

export const getPRDiff = (api: string, scope: "layer" | "since" | "whole") => getJSON<PRDiff>(`${api}/diff?scope=${scope}`);

// Posting is what submitting a pull request's review would post, and
// where.
export interface Posting {
  repo: string;
  number: number;
  title: string;
  url: string;
  commit: string;
  event: "APPROVE" | "REQUEST_CHANGES" | "COMMENT";
  body: string;
  comments: { id: string; path: string; side: "RIGHT" | "LEFT"; startLine?: number; line: number; body: string }[];
  loose?: number; // comments in the body, not on their lines
}

export const previewPRReview = (api: string, head: string, verdict: Verdict, summary: string) =>
  send<Posting>("POST", `${api}/review/preview`, { head, verdict, summary });

export const submitPRReview = (api: string, head: string, verdict: Verdict, summary: string) =>
  send<ReviewState>("POST", `${api}/review/submit`, { head, verdict, summary });

// GitHub's review threads on a pull request, as a review's comments.
export const getPRThreads = (api: string) => getJSON<Review>(`${api}/threads`);

export const respondPRThread = (api: string, id: string, what: "reply" | "resolve" | "reopen", body = "") =>
  send<Review>("POST", `${api}/threads/${encodeURIComponent(id)}/respond`, { what, body });

// A pass over a review: a page's pass, with how it stands (go or no go,
// worked out from what it found) and the commit it last read the change
// at. Mirrors ReviewPass in internal/web/reviewpasses.go.
export interface ReviewPass extends PagePass {
  blocks?: string[]; // the kinds that block
  standing: PassMark["standing"];
  ran?: string;
}

export const getPRPasses = (api: string) => getJSON<{ passes: ReviewPass[] }>(`${api}/passes`).then((r) => r.passes);
export const runPRPass = (api: string, pass: string) => send<ReviewPass>("POST", `${api}/passes/${encodeURIComponent(pass)}/run`);
// settlePRFinding takes a finding into your draft review (as the agent's
// comment), dismisses it (why: wrong, said already, not worth it), or
// opens it again.
export const settlePRFinding = (api: string, pass: string, key: string, state: "taken" | "dismissed" | "open", why?: DismissWhy) =>
  send<ReviewPass>("POST", `${api}/passes/${encodeURIComponent(pass)}/findings/${encodeURIComponent(key)}`, { state, why });

// A review's guide: what the change does, where that differs from what
// it says it does, and its files in the order to read them, in groups.
// Mirrors ReviewGuide in internal/web/reviewpasses.go.
export interface GuideGroup {
  name: string;
  why?: string;
  skim?: boolean; // mechanical: they needn't be read closely
  files: { path: string; note?: string }[];
}

export interface GuideText {
  does: string;
  differs?: string[];
  groups: GuideGroup[];
}

export interface ReviewGuide {
  id?: string;
  running?: boolean;
  failed?: string;
  at?: string;
  ran?: string; // the commit it read the change at
  moved?: boolean; // it's been pushed to since
  guide?: GuideText;
  // While it's being written: since when (none while it waits to start),
  // what its agent was given (the files it may place, and those it wasn't
  // shown, and why), whether it's thinking before it writes, and what it's
  // written so far, read as the guide is.
  since?: string;
  given?: { files: string[]; left?: { path: string; why: string }[] };
  thinking?: boolean;
  draft?: GuideText;
}

export const getPRGuide = (api: string) => getJSON<ReviewGuide>(`${api}/guide`);
export const runPRGuide = (api: string) => send<ReviewGuide>("POST", `${api}/guide/run`);

// Said is something said on a pull request itself, on GitHub: a review
// (its verdict and what was written with it) or a comment.
export interface Said {
  id: string;
  by: string;
  mine?: boolean;
  bot?: boolean;
  kind: "comment" | "approved" | "changes" | "commented" | "dismissed";
  body?: string;
  at: string;
  url?: string;
  stale?: boolean; // a review of commits it's moved on from
}

// Discussion is what's been said on a pull request, oldest first, and
// each other reviewer's latest verdict.
export interface Discussion {
  items: Said[];
  reviewers: { login: string; kind: "approved" | "changes" | "dismissed"; at: string; stale?: boolean }[];
}

export const getPRDiscussion = (api: string) => getJSON<Discussion>(`${api}/discussion`);

// commentPR says something on the pull request itself. It posts at once.
export const commentPR = (api: string, body: string) => send<Discussion>("POST", `${api}/discussion`, { body });

// askPR asks about a pull request (about lines of its diff, when given),
// docking it first: an ask, as startAsk's is about a document, followed
// with getTalk and kept only if you keep it.
export const askPR = (api: string, q: { body: string; path?: string; side?: "new" | "old"; line?: number; endLine?: number; commit?: string }) =>
  send<TalkDetail>("POST", `${api}/ask`, q);

// dockPR checks a pull request out here, as ws dock does.
export const dockPR = (api: string) => send<{ message: string }>("POST", `${api}/dock`);

// A capsule a conversation's agent lifted.
export interface TalkCapsule {
  repo: string;
  name: string;
  branch: string;
  at: string;
  away?: string;
  kept?: boolean;
}
