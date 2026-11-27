import type { Pinned, ThingAction } from "./refs/refs";
import { useEffect, useRef, useState } from "react";
import type { Place } from "./markdown/links";
import { bursts } from "./load";
import { pageSocket } from "./socket";
// What the API sends and takes, generated from internal/web's Go types
// (scripts/wiretypes, `just wire`). The types below are the ones not yet
// moved there: move one by making the Go say what this file does.
import type {
  PullsUnsent,
  RunLog,
  HarnessTranscriptEntry as TranscriptEntry,
  Transcript,
  Action,
  ArtifactContent,
  ArtifactDetail,
  ArtifactDone,
  ArtifactFoundDetail,
  ArtifactsFound,
  ArtifactFile,
  ArtifactSource,
  BundleNote,
  CellSource,
  Citation,
  ConversationApproval,
  ConversationCapsule,
  ConversationGuideGroup,
  DecideRequest,
  DecideResult,
  DismissWhy,
  DocHit,
  DocIn,
  DocKind,
  DocSaved,
  DocStage,
  Event,
  FocusDone,
  HarnessToolCall,
  LandStopped,
  Landed,
  Landing,
  MissionDetail,
  NeedsAnnounced,
  Notifications,
  NotificationsAnnounced,
  NamesInfo,
  OrchestratorInfo,
  OrchestratorRunDetail,
  PageFiling,
  PageInfo,
  PageTied,
  Pick,
  PluginChanged,
  PluginThing,
  PluginsInstalled,
  ProcessInfo,
  RefPathsFound,
  Repo,
  ReviewGuide,
  PRWritten,
  AppDetail,
  RunDetail,
  RunPast,
  RunsInfo,
  WorktreeRunsInfo,
  SettingChange,
  SettingsInfo,
  Space,
  SpaceActivity,
  SpaceDoc,
  SpaceFresh,
  SpaceRef,
  Talk,
  Task,
  TeamPerson,
  TeamTopic,
  TermKey,
  ThingOffered,
  ThingRequest,
  ThingSaved,
  UpdatesInfo,
  Upgraded,
  WhatsNewInfo,
  WithdrawRequest,
  Worktree,
  GitHubOwners,
  GitHubRepos,
  RepoGroup,
  TalkModel,
  ConversationModel,
  AutomationDryRun,
  AutomationPage,
  AutomationProposed,
  AutomationsInfo,
  AutomationOutputInfo,
  ActionRan,
  ActionsInfo,
  MergeRequest,
  MergeResult,
} from "./wire.gen";
export type {
  CommandLabel,
  Artifact,
  ArtifactContent,
  ArtifactDetail,
  ArtifactDone,
  ArtifactFound,
  ArtifactFoundDetail,
  ArtifactFile,
  ArtifactInfo,
  ArtifactSource,
  ArtifactStatus,
  ArtifactVersion,
  Bundle,
  BundleCheck,
  BundleEvidence,
  BundleNote,
  BundleRan,
  BundleTry,
  Capsule,
  CapsuleDetail,
  CellSource,
  CheckVerdict,
  Checking,
  Citation,
  Conclusion,
  Crew,
  CrewDetail,
  CrewPicks,
  Decision,
  DismissWhy,
  DocHit,
  DocIn,
  DocKind,
  DocSaved,
  DocSaves,
  DocStage,
  EndedCapsule,
  FocusDone,
  FocusPin,
  Headline,
  HeadlineGroup,
  InstalledPlugin,
  LandStopped,
  Landed,
  Landing,
  Look,
  Mission,
  MissionAct,
  Need,
  NeedOf,
  NeedsAnnounced,
  Notification,
  Notifications,
  NotificationsAnnounced,
  MissionCard,
  MissionDetail,
  MissionState,
  OrchestratorCall,
  OrchestratorInfo,
  OrchestratorRun,
  OrchestratorRunDetail,
  PageFiled,
  PageFiling,
  PageInfo,
  PageSource,
  PageTied,
  Plugin,
  PluginChanged,
  PluginFrom,
  PluginRef,
  PluginThing,
  PluginView,
  PluginsInstalled,
  ProcessInfo,
  Proposal,
  Repo,
  ReviewGuide,
  RoleRun,
  AppDetail,
  AppInfo,
  AppPointer,
  RecipeInfo,
  RecipeKind,
  RecipeRender,
  RunDetail,
  RunPast,
  RunReport,
  RunsInfo,
  WorktreeRunsInfo,
  WorkspaceSiloHook as SiloHook,
  WorkspaceSiloState as SiloState,
  WorkspaceSiloSync as SiloSync,
  Setting,
  SettingChange,
  SettingChoice,
  SettingsGroup,
  SettingsInfo,
  Space,
  SpaceActivity,
  SpaceCommit,
  SpaceDoc,
  SpaceDocChange,
  SpaceFlight,
  SpaceFresh,
  SpaceKind,
  SpaceRef,
  Stage,
  StageCheck,
  StageRun,
  StageWatch,
  Talk,
  TeamPerson,
  TeamTopic,
  TermKey,
  ThingOffered,
  ThingPlace,
  ThingRequest,
  ThingSaved,
  UpdatesInfo,
  Upgraded,
  WhatsNewEntry,
  WhatsNewInfo,
  WantedPlugin,
  Workspace,
  Worktree,
  GitHubOwners,
  GitHubRepos,
  AddableRepo,
  RepoGroup,
  TalkModel,
  ConversationModel,
} from "./wire.gen";

// Mirrors internal/web/api.go.


// HeadlineAction is what moves a headline on; verb "" (absent) is nothing
// to do.
export type HeadlineAction = Action;


// Ending is how a mission ends: landed (at Z, its code merged or landed),
// answered (a finding, no code), handed on (its work raised as other
// missions) or dropped.
export type Ending = "landed" | "answered" | "handed-on" | "dropped";


export class NotSignedIn extends Error {}

// ApiError is a request the server refused or couldn't do: its status, the
// code it gave a refusal (capsule-busy, changed-since, nothing-asked…:
// what a page branches on), and its sentence, to show.
export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string | undefined,
    message: string,
    // output is what was printed on the way to it failing, where the
    // server says: a plugin's build.
    readonly output?: string,
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
  if (!res.ok) throw new ApiError(res.status, data.code, data.error ?? `${res.status} ${res.statusText}`, typeof data.output === "string" ? data.output : undefined);
  return data as T;
}

export const getJSON = <T>(path: string) => request<T>("GET", path);

// getVersioned is a view and the version it's as of (X-Ws-Version), for
// one that has one: 0 where it doesn't say.
export async function getVersioned<T>(path: string): Promise<{ data: T; version: number }> {
  const res = await fetch(path, { headers: { accept: "application/json" } });
  if (res.status === 401) throw new NotSignedIn();
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new ApiError(res.status, data.code, data.error ?? `${res.status} ${res.statusText}`, typeof data.output === "string" ? data.output : undefined);
  return { data: data as T, version: Number(res.headers.get("X-Ws-Version") ?? 0) || 0 };
}

export type Connection = "connecting" | "live" | "lost";

// useVisible is whether the page is showing. What it follows is followed
// only while it is: a hidden tab's socket closes, and doesn't hold the
// daemon up.
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
export type Topic = "workspace" | "reviews" | "team" | "plugins" | "github" | "runs" | "automations" | `mission:${string}` | `conversation:${string}` | `pr:${string}`;

// EventHandlers are what a page does as it follows a topic: when the
// stream's open (hello: fetch what it shows, so nothing's missed), on each
// change, and when the stream's lost (closed: it won't come back by
// itself, as when it was refused).
export interface EventHandlers {
  hello?: () => void;
  // version is the latest version told of what changed, where it has one
  // (the workspace's view): what's held as of it, or later, is current.
  changed: (version?: number) => void;
  lost?: (closed: boolean) => void;
}

// useEvents follows a topic while the page is showing, over the tab's one
// socket (pageSocket). Showing again follows it again, and its hello
// catches up. A change is told at once; those within a moment of it, once,
// after it (bursts). says is what the page says of itself besides its topic
// ("&app=jira", the plugin whose app it shows): it's followed anew when it
// changes.
export function useEvents(topic: Topic, on: EventHandlers, says = "") {
  const visible = useVisible();
  const handlers = useRef(on);
  handlers.current = on;
  useEffect(() => {
    if (!visible) return;
    let version = 0; // the latest told, for a burst's one telling
    const changes = bursts(() => handlers.current.changed(version || undefined));
    const app = new URLSearchParams(says.replace(/^&/, "")).get("app") ?? "";
    const leave = pageSocket.follow(
      topic,
      {
        following: () => handlers.current.hello?.(),
        changed: (said) => {
          version = Math.max(version, said.version ?? 0);
          changes.changed();
        },
        lost: (closed) => handlers.current.lost?.(closed),
      },
      app,
    );
    return () => {
      changes.stop();
      leave();
    };
  }, [topic, says, visible]);
  return visible;
}

// useEventsOf follows each of topics while the page is showing, as
// useEvents follows one: a change to any is told once, those within a
// moment of it after it (bursts). An empty list follows nothing.
export function useEventsOf(topics: string[], on: EventHandlers) {
  const visible = useVisible();
  const handlers = useRef(on);
  handlers.current = on;
  const key = topics.join("\n");
  useEffect(() => {
    if (!visible || !key) return;
    const changes = bursts(() => handlers.current.changed());
    const leave = key.split("\n").map((topic) =>
      pageSocket.follow(topic, { following: () => handlers.current.hello?.(), changed: () => changes.changed(), lost: (closed) => handlers.current.lost?.(closed) }),
    );
    return () => {
      changes.stop();
      for (const l of leave) l();
    };
  }, [key, visible]);
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


// One of a plugin's own things put in a repo, and saved there, as you:
// what ws's own UI over the plugin's page asks (plugins/Uis/ThingPromoteUi.tsx).
// thingOffered is where one of that kind, called that, could go: the
// places its kind's things go, the first where it goes unless you say.
export const promoteThing = (plugin: string, thing: ThingRequest) => send<ThingSaved>("POST", `/api/plugins/${encodeURIComponent(plugin)}/things/promote`, thing);
export const saveThing = (plugin: string, thing: ThingRequest) => send<ThingSaved>("POST", `/api/plugins/${encodeURIComponent(plugin)}/things/save`, thing);
export const thingOffered = (plugin: string, kind: string, title: string) =>
  getJSON<ThingOffered>(`/api/plugins/${encodeURIComponent(plugin)}/things/offer?kind=${encodeURIComponent(kind)}&title=${encodeURIComponent(title)}`);

// getPluginThing is one of a plugin's things, by its kind and name: as ws
// has it, or as its plugin says when asked. One the plugin has none of is
// an ApiError with status 404; any other failure is the plugin not being
// there to ask.
export const getPluginThing = (plugin: string, kind: string, ref: string) =>
  getJSON<PluginThing>(`/api/plugins/${encodeURIComponent(plugin)}/refs/${encodeURIComponent(kind)}/${encodeURIComponent(ref)}`);


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


export const GROUND = ".ground";




export const getSpaceActivity = (repo: string) => getJSON<SpaceActivity>(`/api/documents/${encodeURIComponent(repo)}/activity`);
export const fetchSpace = (repo: string) => send<{ fresh: SpaceFresh }>("POST", `/api/documents/${encodeURIComponent(repo)}/fetch`, {});
export const pullSpace = (repo: string) => send<{ fresh: SpaceFresh }>("POST", `/api/documents/${encodeURIComponent(repo)}/pull`, {});


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

export const getCites = (repo: string, path: string) => getJSON<{ citedBy: Citation[]; cites: Citation[] }>(`/api/documents/${encodeURIComponent(repo)}/cites?${new URLSearchParams({ path })}`);

// getSpaces is the workspace's spaces, read fresh; problem says why one
// couldn't be (it's as it was last read).
export const getSpaces = () => getJSON<{ spaces: Space[]; problem?: string }>("/api/documents");
export const getSpace = (repo: string) => getJSON<{ space: Space; docs: SpaceDoc[]; problem?: string }>(`/api/documents/${encodeURIComponent(repo)}`);

export const getRepos = () => getJSON<{ repos: Repo[] }>("/api/repos").then((r) => r.repos);
// shareRepo gives a repo of yours to the team (ws repo share): into
// ws.toml, committed on the workspace's config repo and pushed.
export const shareRepo = (repo: string) => send<{ message: string; commit: string }>("POST", `/api/repos/${encodeURIComponent(repo)}/share`);
// Adding a repo from GitHub: whose repos you can add, an owner's repos
// (each as GitHub last said, asked again behind: the github topic says
// when), and adding one, for you or the team.
const again = (yes?: boolean) => (yes ? "again=1" : "");
export const getGitHubOwners = (look?: boolean) => getJSON<GitHubOwners>(`/api/github/owners?${again(look)}`);
export const getGitHubRepos = (owner: string, look?: boolean) => getJSON<GitHubRepos>(`/api/github/repos?owner=${encodeURIComponent(owner)}&${again(look)}`);
// The workspace's groups, showing or hiding one for you (null: the team's
// again), and putting a repo in a group for you ("" for none).
export const getGroups = () => getJSON<{ groups: RepoGroup[] }>("/api/groups").then((r) => r.groups);
export const showGroup = (group: string, hidden: boolean | null) => send<{ message: string }>("POST", `/api/groups/${encodeURIComponent(group)}/shown`, { hidden });
export const groupRepo = (repo: string, group: string) => send<{ message: string }>("POST", `/api/repos/${encodeURIComponent(repo)}/group`, { group });
export const addRepo = (add: { owner: string; name: string; as?: string; group?: string; team?: boolean }) => send<{ repo: string; message: string }>("POST", "/api/repos", add);
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
  at?: string; // when it was said: in a draft here, or first on GitHub
  suggestion?: string | null;
  status: NoteStatus;
  // sending (a reply, or the comment's status): made here, in a pull
  // request's thread, and on its way to GitHub.
  thread: { by: string; at: string; body: string; commit?: string; status?: string; sending?: boolean }[];
  sending?: boolean;
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
  sending?: boolean; // or that it's on its way there, posted behind
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




// Names is where the workspace's names under .localhost stand (ws
// router): whether ws.local.toml's [router] turns them on, the name it
// asks for and the one it has ("" until its daemon's written its routes;
// another, given a clash), the machine's router's port (absent: none
// serving; fallback: 80 couldn't be had) and the daemon holding it, and
// the web UI by its name. Mirrors internal/web/runs.go.
export type Names = NamesInfo;

export const getRuns = () => getJSON<RunsInfo>("/api/runs");

// endRun ends any run now, or clears away one that's ended, its log with it.
export const endRun = (ref: string) => send<{ stopped: string }>("POST", "/api/runs/stop", { ref });

// Apps: the workspace's own recipes with a page. Opening one starts its
// run, or joins the one that's running, and the page opens it again every
// minute while it's open, which keeps it; once nobody has it open, ws
// ends it. Mirrors internal/web/apps.go.
//
// A repo's recipe with a page is an app of each of its worktrees (at),
// a tab of its repo's page; with no at, it's the workspace's own.
export interface AppAt {
  repo: string;
  worktree: string;
}
const placePath = (at: AppAt) => `/api/repos/${encodeURIComponent(at.repo)}/worktrees/${encodeURIComponent(at.worktree)}`;
const appPath = (name: string, rest = "", at?: AppAt) => `${at ? `${placePath(at)}/apps` : "/api/apps"}/${encodeURIComponent(name)}${rest}`;
export const getApp = (name: string, at?: AppAt) => getJSON<AppDetail>(appPath(name, "", at));
export const openApp = (name: string, at?: AppAt) => send<AppDetail>("POST", appPath(name, "/open", at), {});
export const restartApp = (name: string, at?: AppAt) => send<AppDetail>("POST", appPath(name, "/restart", at), {});
export const stopApp = (name: string, at?: AppAt) => send<AppDetail>("POST", appPath(name, "/stop", at), {});

// pointApp points one of the app's needs at a worktree of its repo (a
// capsule's name, ground or silo; "" for the nearest), starting the app
// again on it where it's running.
export const pointApp = (name: string, need: string, at: string) => send<AppDetail>("POST", appPath(name, "/point"), { need, at });

// getRunPast is a task's run, and the earlier runs of it kept behind it,
// the latest first.
export const getRunPast = (ref: string) => getJSON<RunPast>(`/api/runs/past?ref=${encodeURIComponent(ref)}`);

// getRunLog is the end of a run's output.
export const getRunLog = (ref: string) => getJSON<RunLog>(`/api/runs/log?ref=${encodeURIComponent(ref)}`);

// runRecipe starts a capsule's recipe, for as long as someone has the
// mission open (and 15 minutes after).
export const runRecipe = (slug: string, repo: string, capsule: string, recipe: string) =>
  send<RunDetail>("POST", `/api/missions/${encodeURIComponent(slug)}/runs`, { repo, capsule, recipe });

// getWorktreeRuns is what's running in a repo's ground or capsule, and
// its recipes, for the Files page: each ask says the page has it open, so
// what's started from there lasts.
export const getWorktreeRuns = (repo: string, name: string) => getJSON<WorktreeRunsInfo>(`${worktreeAPI(repo, name)}/runs`);

// runInWorktree starts a ground's or capsule's recipe, for as long as
// someone has it open on the Files page (and 15 minutes after).
export const runInWorktree = (repo: string, name: string, recipe: string) => send<RunDetail>("POST", `${worktreeAPI(repo, name)}/runs`, { recipe });

// writePR has the PR writer write a capsule's pull request, for you to
// read and change before you open it. It answers once it's written: a
// minute or more.
export const writePR = (slug: string, capsule: string) =>
  send<PRWritten>("POST", `/api/missions/${encodeURIComponent(slug)}/pr-write`, { capsule });

// pointSilo points a repo's silo at a capsule (ws silo point), and says
// what it did.
export const pointSilo = (repo: string, to: string) => send<{ said: string }>("POST", `/api/silos/${encodeURIComponent(repo)}/point`, { to });

// resyncSilo has the daemon give a repo's silo its target's files in full.
export const resyncSilo = (repo: string) => send<{ resynced: string }>("POST", `/api/silos/${encodeURIComponent(repo)}/resync`, {});

// stopRun ends a run now.
export const stopRun = (slug: string, ref: string) => send<{ stopped: string }>("POST", `/api/missions/${encodeURIComponent(slug)}/runs/stop`, { ref });




export const getLanding = (repo: string, name: string) => getJSON<Landing>(`${worktreeAPI(repo, name)}/landing`);
export const landCapsule = (repo: string, name: string) => send<{ landed?: Landed; stopped?: LandStopped }>("POST", `${worktreeAPI(repo, name)}/land`);


// --- Artifacts: versioned HTML prototypes


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

export const getArtifactDetail = (id: string) => getJSON<ArtifactDetail>(`/api/artifacts/${encodeURIComponent(id)}`);
// The artifacts in the workspace's repos that aren't this machine's, as
// ws found them there; and one of them, by its folder's name
// (records/artifacts/2026-11-20-horizon-glow/), with its files.
export const getArtifactsFound = () => getJSON<ArtifactsFound>("/api/artifacts/found");
export const getArtifactFound = (name: string) => getJSON<ArtifactFoundDetail>(`/api/artifacts/found/detail?${new URLSearchParams({ name })}`);
// A version's source is one of its files, as text to read (its page,
// index.html, unless another's asked for), with the version's files, to
// choose another.
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

export type MissionEvent = Event;



// CrewPick is a harness, and the model to run on it (none: the agent's own).
export type CrewPick = Pick;



// kindHint says what a document's kind is, for its label's title.
export const kindHint = (k: DocKind) =>
  `${k.title}: one of the workspace's kinds of document (${k.name}, at ${k.where})${k.guide ? `. Writing guide: ${k.guide}` : ""}`;


// DocDraft is an edit to a document not saved yet, kept with the mission:
// what's written, and the version it's from.
export interface DocDraft {
  ref: string;
  base: string;
  text: string;
  at: string;
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

// Approval is a document approved, at the version it was then.
export type Approval = ConversationApproval;

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
  | "bundle-looked"
  | "merge"
  | "merge-ready"
  | "merge-off";

// MissionTask is something a mission waits on you for: a question, or a
// step of its route that's yours (to review, answer, land, or go on from).
export type MissionTask = Task;

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
  pr?: string; // for merge, merge-ready and merge-off: the mission's pull request (repo#N)
}

// act does it, and says what happened.
export const act = (slug: string, verb: ActVerb, opts: ActOpts = {}) =>
  send<{ message: string; path?: string }>("POST", `/api/missions/${encodeURIComponent(slug)}/act`, { verb, ...opts });

// decide records a decision of yours on a mission (in place of another,
// with supersedes), and says its name; withdrawDecision takes one that
// stands back, saying why.
export const decide = (slug: string, d: DecideRequest) => send<DecideResult>("POST", `/api/missions/${encodeURIComponent(slug)}/decisions`, d);
export const withdrawDecision = (slug: string, id: string, why: string) =>
  send<{ message: string }>("POST", `/api/missions/${encodeURIComponent(slug)}/decisions/${encodeURIComponent(id)}/withdraw`, { why } satisfies WithdrawRequest);

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


const crewPath = (slug: string, id: string) => `/api/missions/${encodeURIComponent(slug)}/crew/${encodeURIComponent(id)}`;

export const typeIntoCrew = (slug: string, id: string, input: { key: TermKey } | { paste: string }) =>
  send<object>("POST", `${crewPath(slug, id)}/input`, input);




export const getOrchestrator = () => getJSON<OrchestratorInfo>("/api/orchestrator");




export interface ModelInfo {
  id: string;
  name?: string;
  about?: string;
  default?: boolean;
  // Where it sits among the others, where its name says (harness.Model):
  // its maker, the model (family, version, shown as label), and what this
  // one chooses of it. effort "" is the model's own.
  vendor?: string;
  family?: string;
  version?: string;
  label?: string;
  effort?: string;
  thinking?: boolean;
  fast?: boolean;
}

export const getSettings = () => getJSON<SettingsInfo>("/api/settings");
export const setSettings = (changes: SettingChange[]) => send<SettingsInfo>("POST", "/api/settings", { changes });

// Pin, unpin or put your pins in order: one at a time.
export const changeFocus = (change: { pin: string } | { unpin: string } | { order: string[] }) => send<FocusDone>("POST", "/api/focus", change);

export const getProcess = () => getJSON<ProcessInfo>("/api/process");
export const getModels = (harness: string) => getJSON<ModelInfo[]>(`/api/settings/models?harness=${encodeURIComponent(harness)}`);
export const getOrchestratorRun = (slug: string, id: string) =>
  getJSON<OrchestratorRunDetail>(`/api/missions/${encodeURIComponent(slug)}/orchestrator/${encodeURIComponent(id)}`);

// getMission is a mission's page. Its pull requests are as GitHub last
// said: the daemon asks again behind the read, and the mission's event
// tells of a different answer.
export const getMission = (slug: string) => getJSON<MissionDetail>(`/api/missions/${encodeURIComponent(slug)}`);

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

// TalkProposal is something the agent proposed: a decision, a mission
// (with a brief, ready; without, an idea to brief later), an edit to the
// brief, crew, a replan by the mission's planner, a review comment, a
// document of one of the workspace's kinds, or an artifact (or a version
// of one).
export interface TalkProposal {
  n: number;
  kind: "decision" | "mission" | "brief" | "dispatch" | "replan" | "comment" | "plan" | "document" | "artifact" | "plugin" | "automation";
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
  // an automation: what it is and would have done lately (its script is
  // text), as internal/automation.Proposed says it.
  automation?: ProposedAutomation;
  revises?: number; // the agent's revision of an open proposal of its, by number: that one's superseded by this
  state: "open" | "accepted" | "declined" | "superseded" | "withdrawn"; // superseded: a later one took its place; withdrawn: the agent took it back
  because?: string; // withdrawn: why
  result?: string; // a mission's slug, slug:d3, a document's ref (an edit's: where it was saved), or an artifact's version (a3.v1); "" for a decision kept with the conversation; a superseded one's, the later one's number
}

// What a conversation proposes (ProposedAutomation), and what the
// Automations page shows: generated from internal/automation, ops and web.
export type ProposedAutomation = AutomationProposed;
export type {
  AutomationDryLine,
  AutomationDryRun,
  AutomationOutline,
  AutomationOutlineBranch,
  AutomationOutput,
  AutomationOutputInfo,
  AutomationOutlineStep,
  AutomationStepCount,
  AutomationPage,
  AutomationsInfo,
  AutomationTally,
  ActionRan,
  ActionsInfo,
  AutomationInputField,
  OpsAutomationAction as AutomationAction,
  OpsAutomationActionRun as AutomationActionRun,
  OpsAutomationKind as AutomationKind,
  OpsAutomationDid as AutomationDid,
  OpsAutomationItem as AutomationItem,
  OpsAutomationRun as AutomationRun,
} from "./wire.gen";

// getAutomations is each of yours; getAutomation one, with its script and
// latest runs. Mirrors internal/web/automations.go.
export const getAutomations = () => getJSON<AutomationsInfo>("/api/automations");
export const getAutomation = (name: string) => getJSON<AutomationPage>(`/api/automations/${encodeURIComponent(name)}`);
// setAutomationOn turns one on (from the next event) or off.
export const setAutomationOn = (name: string, on: boolean) =>
  send<{ on: boolean }>("POST", `/api/automations/${encodeURIComponent(name)}/on`, { on });
// removeAutomation takes one away: its file and its runs.
export const removeAutomation = (name: string) =>
  send<{ removed: string }>("DELETE", `/api/automations/${encodeURIComponent(name)}`);
// getAutomationOutput is a version of what one shows (the latest without
// one), and every version kept; an HTML one's body isn't sent, it's shown
// from automationOutputFrameURL.
export const getAutomationOutput = (name: string, version?: number) =>
  getJSON<AutomationOutputInfo>(`/api/automations/${encodeURIComponent(name)}/output${version ? `?version=${version}` : ""}`);
export const automationOutputFrameURL = (name: string, version: number) => `/frame/automations/${encodeURIComponent(name)}/output/${version}`;
// runAutomationNow runs one that's on a schedule now, as if its time had
// come.
export const runAutomationNow = (name: string) => send<{ event: string }>("POST", `/api/automations/${encodeURIComponent(name)}/run`);
// getActions is your automations' actions on what on names (a ref), as
// it stands, or on nothing in particular without one; runAction runs one,
// by you, and says how it went by the time it's ended, parked, or a few
// seconds have gone. Mirrors internal/web/automations.go.
export const setAutomationGrant = (name: string, kind: string, on: boolean) =>
  send<{ kind: string; on: boolean }>("POST", `/api/automations/${encodeURIComponent(name)}/grants`, { kind, on });
export const getActions = (on?: string) => getJSON<ActionsInfo>(`/api/actions${on ? `?on=${encodeURIComponent(on)}` : ""}`);
export const runAction = (body: { automation: string; title: string; on?: string; input?: Record<string, unknown> }) => send<ActionRan>("POST", "/api/actions/run", body);
// dryRunAutomation is what one would have done over the last 30 days,
// done again now.
export const dryRunAutomation = (name: string) =>
  send<AutomationDryRun>("POST", `/api/automations/${encodeURIComponent(name)}/dry-run`);

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
export type TalkCall = HarnessToolCall;

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
  model?: ConversationModel; // the model you chose for its turns, over its role's
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
  // version: the records' change number it's as of, or later: a change
  // told as of an earlier one (ConversationOp) is in it already.
  version?: number;
}

// A conversation is the workspace's (slug "") or a mission's (its Talk tab).
const talkPath = (slug: string, id = "", rest = "") =>
  `/api${slug ? `/missions/${encodeURIComponent(slug)}` : ""}/conversations${id ? `/${encodeURIComponent(id)}` : ""}${rest}`;

export const getTalk = (slug: string, id: string) => getJSON<TalkDetail>(talkPath(slug, id));
export const startTalk = (slug: string, body: string) => send<TalkDetail>("POST", talkPath(slug), { body });
export const sayInTalk = (slug: string, id: string, body: string, given?: TalkGiven) =>
  send<TalkDetail>("POST", talkPath(slug, id, "/say"), given ? { body, given } : { body });
export const setTalkStatus = (slug: string, id: string, status: "open" | "resolved") => send<TalkDetail>("POST", talkPath(slug, id, "/status"), { status });
// What a conversation's next turns run on, and choosing its model ("" is
// its role's again).
export const getTalkModel = (slug: string, id: string) => getJSON<TalkModel>(talkPath(slug, id, "/model"));
export const setTalkModel = (slug: string, id: string, model: string) => send<TalkModel>("POST", talkPath(slug, id, "/model"), { model });

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

export const dismissWhys: [DismissWhy, string, string][] = [
  ["wrong", "Wrong", "Not true"],
  ["already", "Already said", "Already on the page"],
  ["minor", "Not worth it", "True, but not worth acting on"],
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
export type { TranscriptEntry };

export type { Transcript };

// last: only its latest entries, for what needs no more than what it's
// doing now.
export const getTranscript = (slug: string, id: string, last?: number) =>
  getJSON<Transcript>(`/api/missions/${encodeURIComponent(slug)}/crew/${encodeURIComponent(id)}/transcript${last ? `?last=${last}` : ""}`);

// sayToCrew sends crew a message: typed in if it's running, or resuming it
// with it if it's ended.
// switchCrewModel puts running crew on another model, its session going
// on.
export const switchCrewModel = (slug: string, id: string, model: string) => send<{ message: string }>("POST", `${crewPath(slug, id)}/model`, { model });
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

// unread: it's never been read, and is being (or, asked as kept, nobody's
// shown it yet): the workspace's readsSeen moves when it has been.
export const getSummary = (api: string, kept?: boolean) => getJSON<CapsuleSummary & { unread?: boolean }>(`${api}/summary${kept ? "?kept=1" : ""}`);

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
  // unread: it's never been read, and is being: the workspace's readsSeen
  // moves when it has been.
  unread?: boolean;
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

// checkRefPaths is which of the paths a page's words name are files or
// folders on each repo's default branch, as the daemon keeps them: a repo
// that's not been read yet isn't in the answer (the workspace's
// groundsSeen moves when it has been).
export const checkRefPaths = (repos: Record<string, string[]>) => send<RefPathsFound>("POST", "/api/refs/paths", { repos });
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
  // Whether it merges cleanly into its base: unknown ("") until GitHub says.
  merges?: "clean" | "conflicts";
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
  // What ws offers to merge it, of yours on its own: now (ready), or once
  // it can (ready: only waiting holds it). A stack's layer merges with
  // its stack.
  merge?: "now" | "ready";
  autoMerge?: boolean; // it's set to merge by itself once it can
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
  merges?: boolean; // yours, GitHub's, every layer ready: it merges whole, from its top
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
  reading?: boolean; // nothing's been read yet (or not as far back): the page's told when it has
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


// summariseTeam asks for a period's account to be written.
export const summariseTeam = (since: string) => send<{ started: boolean }>("POST", "/api/team/summarise", { since });

// seenTeam says you've looked at the page.
export const seenTeam = () => send<{ ok: boolean }>("POST", "/api/team/seen", {});

// Updates: what the daemon last found of later releases (it looks once a
// day), looking now, and installing ws's latest in place of the one
// that's running, as ws upgrade does.
export const getUpdates = () => getJSON<UpdatesInfo>("/api/updates");
export const checkUpdates = () => send<UpdatesInfo>("POST", "/api/updates/check", {});
export const upgradeWS = () => send<Upgraded>("POST", "/api/upgrade", {});

// Which of these needs, new to this page, the desktop hasn't been told of
// by any page: the daemon answers once for each, so they're told once
// however many pages are open. holding is every need the page has now.
export const announceNeeds = (keys: string[], holding: string[]) => send<NeedsAnnounced>("POST", "/api/needs/announce", { keys, holding }).then((a) => a.fresh);

// Notifications: the feed of what happened, kept (what needs you isn't
// here: it's the workspace's needs, worked out). Saying some are read, or
// putting them away, answers with the feed as it is then.
export const getNotifications = () => getJSON<Notifications>("/api/notifications");
export const readNotifications = (ns: number[]) => send<Notifications>("POST", "/api/notifications/read", { ns });
export const readAllNotifications = () => send<Notifications>("POST", "/api/notifications/read", { all: true });
export const dismissNotifications = (ns: number[]) => send<Notifications>("POST", "/api/notifications/dismiss", { ns });
// Which of these notifications, new to this page, the desktop hasn't been
// told of by any page.
export const announceNotifications = (ns: number[]) => send<NotificationsAnnounced>("POST", "/api/notifications/announce", { ns }).then((a) => a.fresh);

// What's new: what this ws says is new, and what of it is unseen in this
// workspace; and saying it's been seen.
export const getWhatsNew = () => getJSON<WhatsNewInfo>("/api/whats-new");
export const seenWhatsNew = () => send<{ ok: boolean }>("POST", "/api/whats-new/seen", {});

// The Plugins page: the workspace's plugins as it lists them, and adding
// one (from the repo it's published at, or a folder on this machine),
// updating one to its repo's latest release, and removing one.
export const getInstalledPlugins = () => getJSON<PluginsInstalled>("/api/plugins/installed");
export const addPlugin = (from: string) => send<PluginChanged>("POST", "/api/plugins", { from });
export const updatePlugin = (name: string) => send<PluginChanged>("POST", `/api/plugins/${encodeURIComponent(name)}/update`, {});
export const removePlugin = (name: string, data: boolean) => send<{ removed: string }>("POST", `/api/plugins/${encodeURIComponent(name)}/remove`, { data });

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
  // What was said from here that GitHub didn't take, to send again or let
  // go of.
  unsent?: PullsUnsent[];
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
export type GuideGroup = ConversationGuideGroup;

export interface GuideText {
  does: string;
  differs?: string[];
  groups: GuideGroup[];
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
  sending?: boolean; // a comment made from here, on its way to GitHub
}

// Discussion is what's been said on a pull request, oldest first, and
// each other reviewer's latest verdict.
export interface Discussion {
  items: Said[];
  reviewers: { login: string; kind: "approved" | "changes" | "dismissed"; at: string; stale?: boolean }[];
}

export const getPRDiscussion = (api: string) => getJSON<Discussion>(`${api}/discussion`);

// commentPR says something on the pull request itself. It answers with it
// shown, as on its way: GitHub's told behind.
export const commentPR = (api: string, body: string) => send<Discussion>("POST", `${api}/discussion`, { body });

// Something said from a pull request's page that GitHub didn't take: sent
// again, or let go of.
export type Unsent = PullsUnsent;
export const resendPR = (api: string, id: number) => send<Record<string, never>>("POST", `${api}/unsent/${id}/resend`);
export const discardPR = (api: string, id: number) => send<Record<string, never>>("DELETE", `${api}/unsent/${id}`);

// askPR asks about a pull request (about lines of its diff, when given),
// docking it first: an ask, as startAsk's is about a document, followed
// with getTalk and kept only if you keep it.
export const askPR = (api: string, q: { body: string; path?: string; side?: "new" | "old"; line?: number; endLine?: number; commit?: string }) =>
  send<TalkDetail>("POST", `${api}/ask`, q);

// dockPR checks a pull request out here, as ws dock does.
export const dockPR = (api: string) => send<{ message: string }>("POST", `${api}/dock`);

// declinePR takes your name off a pull request's reviewers on GitHub: it
// isn't yours to review, and stops needing you.
export const declinePR = (api: string) => send<Record<string, never>>("POST", `${api}/decline`);

// mergePR merges a pull request (a stack whole, asked of its top), now or
// once it can ("ready"), or stops it merging by itself ("off"), at the
// commit the page shows (head): ws never goes round its base's rules, and
// says why when it won't.
export const mergePR = (api: string, req: MergeRequest) => send<MergeResult>("POST", `${api}/merge`, req);

// A capsule a conversation's agent lifted.
export type TalkCapsule = ConversationCapsule;

// Pictures pasted for an agent (internal/images): kept in the workspace's
// .ws/images, and named in what's written by their path. Mirrors
// internal/web/images.go.
import type { Pasted } from "./wire.gen";
export type { Pasted };
export const pastePicture = async (picture: Blob) => send<Pasted>("POST", "/api/images", { data: await base64Of(picture) });

// base64Of is a picture's bytes as base64, which is how a write sends them
// (every write is JSON).
async function base64Of(blob: Blob): Promise<string> {
  const url = await new Promise<string>((resolve, reject) => {
    const r = new FileReader();
    r.onload = () => resolve(r.result as string);
    r.onerror = () => reject(r.error);
    r.readAsDataURL(blob);
  });
  return url.slice(url.indexOf(",") + 1);
}
