import { Fragment, memo, useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { Link, useNavigate, useParams, useSearchParams } from "react-router";
import {
  addNote,
  capsuleAPI,
  deleteNote,
  type Diff,
  type DiffFile,
  type DismissWhy,
  discardReview,
  editNote,
  getBlob,
  getDiff,
  getFileDiff,
  getReview,
  type NoteDraft,
  type NoteKind,
  noteKinds,
  type PassFinding,
  type Review as ReviewData,
  type ReviewNote,
  type ReviewState,
  respond,
  setViewed,
  submitReview,
} from "../api";
import { type KeyMap, useKeys } from "../keys";
import { Banner } from "../ui/Banner";
import { Icon } from "../ui/Icon";
import { Crumbs } from "../ui/Crumbs";
import { Excerpt } from "../ui/Excerpt";
import { NoteBody, NoteCard, NoteHead, NoteHint, NoteId, NoteStatus, NoteThread, NoteTurn } from "../ui/NoteCard";
import { ReplyBox } from "../ui/ReplyBox";
import { Suggestion } from "./Suggestion";
import { fuzzy } from "../ui/fuzzy";
import { Inline } from "../ui/Inline";
import { useMedia } from "../ui/useMedia";
import { useTitle } from "../ui/useTitle";
import { toast } from "../ui/toast";
import { Loading } from "../ui/Loading";
import { Callout } from "../ui/Callout";
import { Keys } from "../ui/Keys";
import { exact, stamp } from "../ui/when";
import { highlightLines, languageOf, useHighlighter } from "./highlight";
import { useCodeNav } from "./CodeNav";
import { LandBar } from "./Land";
import { joinSuggestion, SubmitDialog, splitSuggestion, statusWords, suggestionBlock, verdictWords } from "./parts";
import { FileList, type ViewEntry, ViewList } from "./FileList";
import { type FindingAct, FindingCard, followWords } from "./Findings";
import { GuideMark, GuideSection } from "../reviews/Guide";
import { MissionContext } from "../reviews/Overview";
import { PassesSection, PassMarks } from "../reviews/Passes";
import { useReviewPasses } from "../reviews/usePasses";
import { type FileGroup, groupByOrder, type Order, orderFiles, skimWords } from "./files";
import { type Cell, cellHTML, type Editing, type FileState, type Found, fileRows, type Highlighted, isChange, isHead, navigable, type Placed, type Row, sameLine, type Target } from "./model";
import { useLatest } from "../load";
import { Chip } from "../ui/Chip";
import { Button, buttonClass } from "../ui/Button";
import { capital, humanError } from "../ui/text";
import { Segment, Segmented } from "../ui/Segmented";
import { Checkbox } from "../ui/Checkbox";
import { TextArea } from "../ui/TextArea";
import { Finder, FinderNone, FinderRow } from "../ui/Finder";
import { useWorkspace } from "../workspace/store";
import { usePref } from "../prefs";

export const reviewKeys: [string, string][] = [
  ["1 – 5", "Overview, Guide, Discussion, Passes, Changes (as many as the review has)"],
  ["j / k", "Next / previous line"],
  ["n / p", "Next / previous change"],
  ["J / K", "Next / previous file"],
  ["mod+D / mod+U", "Down / up half a page"],
  ["g / G", "First / last"],
  ["t", "Jump to a file"],
  ["c", "Comment on the line (or the selected lines)"],
  ["⇧↓ / ⇧↑", "Select lines (or V then move, or drag the line numbers)"],
  ["⏎", "On a comment: reply (or edit a draft); on a pass's finding: take it into your review; show hidden lines, or a large or folded file"],
  ["x", "On a comment: resolve or reopen it; else mark the file viewed"],
  ["D", "Delete a draft comment; on a pass's finding: dismiss it (then 1, 2 or 3 for why)"],
  ["s", "Submit the review"],
  ["i", "Re-reviewing: what changed since your last review, or all of it"],
  ["e", "Show 20 more lines around this change"],
  ["u", "Unified / side by side"],
  ["r", "Reload"],
  ["esc", "Clear the selection, or back to the mission"],
];

const fileID = (f: DiffFile) => f.newBlob ?? f.oldBlob ?? f.status;

// A view's seen mark (how much had been said when you last looked) is kept
// beside the files' viewed marks, under ":" and its key: no file is named
// so.
const seenKey = (view: string) => `:${view}`;

// ViewKey names a review's views: what's read beside the changes, and the
// changes. One shows at a time, in this order down the side.
export type ViewKey = "overview" | "guide" | "discussion" | "passes" | "changes";

const viewNames: Record<ViewKey, { label: string; icon: ViewEntry["icon"] }> = {
  overview: { label: "Overview", icon: "info" },
  guide: { label: "Guide", icon: "book" },
  discussion: { label: "Discussion", icon: "message" },
  passes: { label: "Passes", icon: "list-checks" },
  changes: { label: "Changes", icon: "code" },
};

// ReviewPart is one of the views read beside the changes. Its body may
// want to send the cursor to something in the diff (a pass's finding, a
// comment): that shows the Changes view. seen, where there is one, is kept
// each time it's looked at (how much had been said, say), and news says
// what's new since, of the seen that was kept.
export interface ReviewPart {
  key: Exclude<ViewKey, "changes">;
  mark?: React.ReactNode; // how it stands, beside its name in the list
  count?: number; // how much is in it
  seen?: string;
  news?: (seen: string) => string | undefined;
  body: React.ReactNode | ((go: { toFinding: (pass: string, key: string) => boolean; toComment: (id: string) => boolean }) => React.ReactNode);
}

const FILES_MIN = 200;
const FILES_MAX = 640;

// A width that's kept is the list's only while it's one it can have.
const filesWidthOf = (w: number | undefined) => (w !== undefined && w >= FILES_MIN && w <= FILES_MAX ? w : undefined);


// targetOf is the lines a comment on these rows is on: the side picked
// (side by side, the column dragged), else the first line's (new, or
// removed if it was removed), and every line of that side among them.
// Comments on the old side are on removed lines: with none, it's the new.
function targetOf(rows: Row[], picked?: "old" | "new"): Target | undefined {
  const lines = rows.filter((r) => r.type === "line");
  const first = lines[0];
  if (first?.type !== "line") return undefined;
  const nums = (side: "old" | "new") =>
    lines.flatMap((r) => {
      if (r.type !== "line") return [];
      const c = side === "new" ? r.right : r.left?.kind === "d" ? r.left : undefined;
      return c ? [c.num] : [];
    });
  let side: "old" | "new" = picked ?? (first.right ? "new" : "old");
  let ns = nums(side);
  if (!ns.length && side === "old") [side, ns] = ["new", nums("new")];
  return ns.length ? { side, line: Math.min(...ns), endLine: Math.max(...ns) } : undefined;
}

const targetWords = (t: Target) =>
  `${t.side === "old" ? "Removed l" : "L"}${t.endLine > t.line ? `ines ${t.line}–${t.endLine}` : `ine ${t.line}`}`;

// ReviewDiff is a diff to review: a capsule's, or a pull request's, which
// may also say which files' changes are as you last reviewed them (after a
// rebase, when the diff is all of it).
export type ReviewDiff = Diff & { rebased?: boolean; unchanged?: string[] };

// ShownFinding is a pass's finding to show in the diff: the pass's name
// and what it's called, and the finding.
export interface ShownFinding {
  pass: string;
  title: string;
  finding: PassFinding;
}

// Selection is lines picked in the diff: to comment on, or to ask about.
export interface Selection {
  path: string;
  side: "old" | "new";
  line: number;
  endLine: number;
  commit: string; // the commit those lines are at
}

// ReviewSource is what's being reviewed, and where its diff and its
// reviews come from: a mission's capsule, or a pull request.
export interface ReviewSource {
  api: string; // where its diff, files and reviews are read and written
  title: string; // the page's
  name: React.ReactNode; // what it is, in the header
  back: { to: string; label: string };
  // headNow is where it is now: a diff of an earlier commit offers to
  // reload. updated changes when its reviews may have, to read them again.
  headNow?: string;
  updated?: string;
  // sinceOf says what "since your last review" is of (a name for it), or
  // undefined when there's nothing since; getDiff reads the diff, of that
  // or (since undefined) of everything. diffKey changes when the diff does.
  sinceOf: (latest?: ReviewData) => string | undefined;
  getDiff: (since?: string) => Promise<ReviewDiff>;
  diffKey?: string;
  // opened is its reviews and its diff (as getDiff gives it, of what
  // sinceOf says), read already: it opens on them, not on nothing.
  opened?: { rs: ReviewState; diff: ReviewDiff };
  sinceName?: (latest?: ReviewData) => string; // "r2", "your last review"
  movedWords: string; // it has new commits since the diff loaded
  empty: string; // there's nothing in the diff
  browse?: string; // where its files are browsed
  // readOnly is why comments can't be made here, when they can't.
  readOnly?: string;
  side?: React.ReactNode; // above the list of files
  actions?: React.ReactNode; // in the header
  notices?: React.ReactNode; // under it
  // views are what's read beside the changes (a pull request's
  // description, its guide, what's been said on it, its passes), each a
  // view of its own above the changes in the list down the side, in the
  // order given. The page opens on the first.
  views?: ReviewPart[];
  // order is an order to read the files in that isn't their folders' (a
  // guide's); onOrder switches between the two, where there's one.
  order?: Order;
  onOrder?: (guided: boolean) => void;
  // guideNotes are the guide's line on each file, by path: what the
  // change does to it. They're said on the file in either order.
  guideNotes?: Record<string, string>;
  // followups are what a follow-up said of your comments, by the comment
  // (its thread): whether what's been pushed since deals with it. It's
  // said on the comment.
  followups?: Record<string, { kind: string; text: string; blocks?: boolean }>;
  // findings are what passes found that's still open, to show on the
  // diff's lines; onFinding takes one into the draft or dismisses it, and
  // answers once that's done. An ask about one goes to onAsk, with what
  // was found said first.
  findings?: ShownFinding[];
  onFinding?: (pass: string, key: string, what: "taken" | "dismissed", why?: DismissWhy) => Promise<unknown>;
  // threads are comments kept somewhere else (a pull request's, on
  // GitHub), shown as a submitted review's; onThread answers one, and
  // gives them back as they then are. With them, your own submitted
  // comments aren't drawn from the reviews kept here: they're among them.
  threads?: ReviewData;
  onThread?: (id: string, what: "reply" | "resolve" | "reopen", body: string) => Promise<unknown>;
  // submit draws the dialog that ends the review; done is called with the
  // reviews as they then are.
  submit: (ctx: { draft: ReviewData | null; diff: ReviewDiff; onClose: () => void; onDiscard: () => void; done: (next: ReviewState) => void }) => React.ReactNode;
  submitted: (r: ReviewData, state: ReviewState) => React.ReactNode; // said once it's submitted
  // onAsk asks an agent about the selected lines (beside Comment), or with
  // none, about the whole of it; with a pass's finding, what it found.
  onAsk?: (sel?: Selection, finding?: AskedFinding) => void;
  // foot stands over the foot of the changes, outside their scroll, so
  // it stays put however far down they go (a pull request's ask bar);
  // dismiss drops what it has open over them (esc), saying whether it had
  // anything.
  foot?: React.ReactNode;
  dismiss?: () => boolean;
  keys?: KeyMap; // more keys, the page's own
}

// AskedFinding is a pass's finding asked about: the pass, by its title,
// and what it found.
export interface AskedFinding {
  pass: string;
  kind: string;
  text: string;
}

// Review is a mission capsule's review.
export function Review() {
  const { slug = "", repo = "", name = "" } = useParams();
  const api = capsuleAPI(slug, repo, name);
  // Its passes and its guide: as a pull request's review has.
  const rp = useReviewPasses(api);
  const mission = useWorkspace((w) => w.missions.find((m) => m.slug === slug));
  const current = mission?.capsules.find((c) => c.repo === repo && c.name === name);
  if (!mission || !current) {
    return (
      <div className="page">
        <h1>No such capsule</h1>
        <p className="dim">
          {slug} has no capsule {repo}/{name}.
        </p>
      </div>
    );
  }
  const source: ReviewSource = {
    api,
    title: `Review ${repo}/${name}`,
    name: (
      <span className="mono">
        {repo}/{name}
      </span>
    ),
    back: { to: `/missions/${slug}`, label: mission.title },
    headNow: current.head,
    updated: `${mission.updated}:${rp.took}`,
    // A capsule has no description of its own: its overview is its
    // mission, whose brief says what it's for. Then its guide and passes.
    // (Nothing's said on GitHub about a capsule.)
    views: [
      { key: "overview", body: <MissionContext mission={mission} capsule={current} /> },
      { key: "guide", mark: <GuideMark guide={rp.guide} />, body: <GuideSection api={api} guide={rp.guide} guided={rp.guided} onGuide={rp.setGuide} onGuided={rp.setGuidedKept} /> },
      {
        key: "passes",
        mark: <PassMarks passes={rp.passes} />,
        body: (go) => (
          <PassesSection
            api={api}
            passes={rp.passes}
            hidden={rp.hidden}
            canAsk={false}
            toFinding={go.toFinding}
            toComment={go.toComment}
            onResolve={() => Promise.resolve()}
            onPass={rp.putPass}
            onTook={rp.bumpTook}
            onHide={rp.hide}
            onAsk={() => undefined}
          />
        ),
      },
    ],
    order: rp.order,
    onOrder: rp.guide?.guide ? rp.setGuidedKept : undefined,
    guideNotes: rp.notes,
    findings: rp.findings,
    onFinding: rp.onFinding,
    // Since your last review, when there's been work since.
    sinceOf: (latest) => (latest && current.head && latest.head !== current.head ? latest.head : undefined),
    getDiff: (since) => getDiff(api, since),
    movedWords: "The capsule has new commits since this loaded",
    empty: "No committed changes yet.",
    browse: `/missions/${slug}/files/${repo}/${name}?only=changed`,
    // A repo with no pull requests: this is the only review there is, and
    // it lands from here.
    notices: <LandBar repo={repo} name={name} updated={`${mission.updated}:${current.head}`} back={`/missions/${slug}?tab=work`} />,
    submit: ({ draft, diff, onClose, onDiscard, done }) => (
      <SubmitDialog
        draft={draft}
        onClose={onClose}
        onDiscard={onDiscard}
        onSubmit={async (verdict, summary) => done(await submitReview(api, { base: diff.base, head: diff.head }, verdict, summary))}
      />
    ),
    submitted: (r) => (
      <Callout tone="done" title={`Submitted ${r.id}: ${verdictWords[r.verdict!]}`}>
        It's the mission's now: the orchestrator is deciding what happens next.
      </Callout>
    ),
  };
  return <ReviewView key={api} source={source} />;
}

// ReviewView is the review page: a diff file by file, keyboard first,
// with the files you've viewed remembered by content, comments by kind,
// and a draft until you submit.
export function ReviewView({ source }: { source: ReviewSource }) {
  const navigate = useNavigate();
  const api = source.api;
  useTitle(source.title);

  const [diff, setDiff] = useState<ReviewDiff | undefined>(source.opened?.diff);
  const [rs, setRS] = useState<ReviewState>(source.opened?.rs ?? { draft: null, reviews: [], viewed: {} });
  const [error, setError] = useState<string>();
  const [blobs, setBlobs] = useState<Record<string, string[]>>({});
  const [expanded, setExpanded] = useState<Record<string, FileState["expanded"]>>({});
  const [loadedLarge, setLoadedLarge] = useState<Record<string, DiffFile>>({});
  // Files whose change is as you reviewed it stay folded until opened.
  const [opened, setOpened] = useState<Record<string, boolean>>({});
  const [splitChosen, setSplit] = usePref("reviewSplit");
  // On a narrow window there's room for one side only.
  const narrow = useMedia("(max-width: 860px)");
  const split = splitChosen && !narrow;
  const [cursor, setCursor] = useState("");
  const [selectFrom, setSelectFrom] = useState<string>();
  const [selectSide, setSelectSide] = useState<"old" | "new">(); // side by side, the column dragged
  const [editing, setEditing] = useState<Editing>();
  const [finder, setFinder] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [submitted, setSubmitted] = useState<{ review: ReviewData; state: ReviewState }>();
  const highlighting = useHighlighter();
  const [replying, setReplying] = useState<string>(); // the comment being answered
  // The list's width while it's dragged, and as it was left (kept).
  const [keptWidth, keepFilesWidth] = usePref("reviewFilesWidth");
  const [dragged, setFilesWidth] = useState<number>();
  const filesWidth = dragged ?? filesWidthOf(keptWidth);
  // What the diff is of: since your last review when there's been work
  // since ("auto"), or all the changes.
  const [scope, setScope] = useState<"auto" | "all">("auto");
  const [scopeNote, setScopeNote] = useState<string>();
  const [reviewsLoaded, setReviewsLoaded] = useState(!!source.opened);
  // Opened on what's been read, it isn't read again as it opens.
  const primed = useRef({ review: !!source.opened, diff: !!source.opened });

  // The view showing: the first unless the address names another. The
  // rest stay as they were left (scrolled, typed into), hidden.
  const [params, setParams] = useSearchParams();
  const parts = source.views ?? [];
  const viewKeys: ViewKey[] = [...parts.map((p) => p.key), "changes"];
  const asked = params.get("view") as ViewKey | null;
  const view: ViewKey = asked && viewKeys.includes(asked) ? asked : viewKeys[0];
  const show = (v: ViewKey) =>
    setParams(
      (p) => {
        const next = new URLSearchParams(p);
        if (v === viewKeys[0]) next.delete("view");
        else next.set("view", v);
        return next;
      },
      { replace: true },
    );

  const fail = (e: Error) => setError(e.message);
  const latest = rs.reviews[0]; // your last submitted review of it
  const src = useRef(source);
  src.current = source;
  const sinceable = source.sinceOf(latest);
  const since = scope === "auto" ? sinceable : undefined;
  // The reviews come first (they say what "since" is), and again whenever
  // what's reviewed changes: crew answering comments shows as they do.
  const beginReview = useLatest();
  const loadReview = useCallback(() => {
    const fresh = beginReview();
    return getReview(api).then(
      (x) => {
        if (!fresh()) return;
        setRS(x);
        setReviewsLoaded(true);
      },
      (e: Error) => fresh() && fail(e),
    );
  }, [api, beginReview]);
  const beginDiff = useLatest();
  const loadDiff = useCallback(() => {
    const fresh = beginDiff();
    src.current.getDiff(since).then(
      (d) => {
        if (!fresh()) return;
        setDiff(d);
        setError(undefined);
      },
      (e: Error) => {
        if (!fresh()) return;
        if (!since) return fail(e);
        setScope("all"); // a rebase took the last review's commit away
        setScopeNote(e.message);
      },
    );
  }, [api, since, source.diffKey, beginDiff]);
  useEffect(() => {
    if (primed.current.review) primed.current.review = false;
    else loadReview();
  }, [loadReview, source.updated]);
  useEffect(() => {
    if (!reviewsLoaded) return;
    if (primed.current.diff) primed.current.diff = false;
    else loadDiff();
  }, [loadDiff, reviewsLoaded]);
  const load = () => {
    loadReview();
    loadDiff();
  };

  const order = source.readOnly ? undefined : source.order;
  const guideNotes = source.readOnly ? undefined : source.guideNotes;
  const files = useMemo(() => {
    const list = diff?.files.map((f) => loadedLarge[f.path] ?? f) ?? [];
    return order ? orderFiles(list, order) : list;
  }, [diff, loadedLarge, order]);
  // In the guide's order, each of its groups is named in the diff, with
  // why it's read there, above its first file.
  const guideHeads = useMemo(() => (order ? new Map(groupByOrder(files, order).groups.map((g) => [g.files[0], g])) : undefined), [files, order]);
  // Comments to show, and where: on its own side when the diff shows that
  // side's commit, and re-reviewing, a comment on the last review's lines
  // on the old side, which is them. Elsewhere, at the file's end (resolved
  // ones then left out).
  const threads = source.threads;
  const notes = useMemo(() => {
    const byPath: Record<string, Placed[]> = {};
    if (!diff) return byPath;
    const where = (n: ReviewNote): Target | undefined => {
      if (n.side === "new" && n.commit === diff.head) return n;
      if (n.commit === diff.base) return { side: "old", line: n.line, endLine: n.endLine };
      return undefined;
    };
    const add = (r: ReviewData, draft: boolean) => {
      for (const n of r.comments) {
        const at = where(n);
        if (!at && !draft && n.status === "resolved") continue;
        (byPath[n.path] ??= []).push({ note: n, review: r.id, draft, at });
      }
    };
    // Comments kept elsewhere (GitHub's threads) include yours, once
    // submitted: then the reviews kept here aren't drawn as well.
    if (threads) add(threads, false);
    else [...rs.reviews].reverse().forEach((r) => add(r, false));
    // Where comments can't be made (a whole stack), a draft isn't shown:
    // it's a layer's.
    if (rs.draft && !source.readOnly) add(rs.draft, true);
    return byPath;
  }, [rs, diff, threads, source.readOnly]);
  // What passes found, where its lines are in this diff: on the side
  // whose commit it was found at. One found at another commit (the change
  // has moved on, or this is what's changed since) isn't drawn: its pass
  // says it's out of date.
  const found = useMemo(() => {
    const byPath: Record<string, Found[]> = {};
    if (!diff || source.readOnly) return byPath;
    for (const x of source.findings ?? []) {
      const f = x.finding;
      if (!f.path || !f.line || f.state !== "open") continue;
      const side = f.side === "old" ? "old" : "new";
      if (f.commit !== (side === "new" ? diff.head : diff.base)) continue;
      (byPath[f.path] ??= []).push({ ...x, at: { side, line: f.line, endLine: f.endLine ?? f.line } });
    }
    return byPath;
  }, [source.findings, diff, source.readOnly]);
  const [dismissing, setDismissing] = useState<string>(); // the finding being dismissed: why?
  const unchanged = useMemo(() => new Set(diff?.unchanged ?? []), [diff]);
  // A file whose change is as you reviewed it stays folded, unless there's
  // something on it still to settle (a draft, an open comment) or you've
  // opened it.
  const unsettled = useMemo(() => {
    const out = new Set<string>();
    for (const [path, list] of Object.entries(notes)) if (list.some((p) => p.draft || p.note.status !== "resolved")) out.add(path);
    for (const path of Object.keys(found)) out.add(path);
    return out;
  }, [notes, found]);
  const states = useMemo(
    () =>
      files.map((f): FileState => {
        const same = unchanged.has(f.path);
        // One to skim (a lockfile, generated) is folded too.
        return { viewed: rs.viewed[f.path] === fileID(f), blob: blobs[f.path], expanded: expanded[f.path] ?? {}, unchanged: same, folded: (same || !!f.skim) && !(opened[f.path] ?? unsettled.has(f.path)) };
      }),
    [files, rs.viewed, blobs, expanded, unchanged, opened, unsettled],
  );
  const rows = useMemo(
    () => files.map((f, i) => fileRows(i, f, states[i], split, notes[f.path], editing?.file === i ? editing : undefined, found[f.path])),
    [files, states, split, notes, editing, found],
  );
  // Comments on files this diff doesn't have (re-reviewing, files that
  // haven't changed since): listed after the files, to answer and resolve.
  const orphans = useMemo((): Row[] => {
    const shown = new Set(files.map((f) => f.path));
    return Object.entries(notes)
      .filter(([path]) => !shown.has(path))
      .flatMap(([, list]) => list)
      .map((placed) => ({ type: "comment", key: `${files.length}:c:${placed.note.id}`, file: files.length, placed, unplaced: true }));
  }, [notes, files]);
  const sinceShown = !!diff?.since || !!diff?.rebased;
  const nav = useMemo(() => [...rows.flat(), ...orphans].filter(navigable), [rows, orphans]);
  const at = Math.max(0, nav.findIndex((r) => r.key === cursor));
  const row = nav[at];

  // Keep the cursor on the same line when the layout changes under it, or
  // failing that (a gap it was on has opened), in the same place.
  const last = useRef<{ row?: Row; at: number }>({ at: 0 });
  const stay = useRef<string>(undefined);
  useEffect(() => {
    if (!nav.length) return;
    const i = nav.findIndex((r) => r.key === cursor);
    if (i >= 0) {
      last.current = { row: nav[i], at: i };
      return;
    }
    // Looking again starts at the first file there's something to read
    // in.
    const first = !cursor && sinceShown ? (nav.find((r) => r.type === "file" && !states[r.file]?.folded && !states[r.file]?.viewed) ?? nav.find((r) => r.type === "file")) : undefined;
    const same = last.current.row && sameLine(nav, last.current.row);
    if (first) stay.current = first.key;
    setCursor((first ?? same ?? nav[Math.min(last.current.at, nav.length - 1)]).key);
  }, [nav, cursor]);

  // When the files' order changes (the guide's, or their folders'), the
  // cursor stays on the file it was on: rows are named by where their file
  // is in the list.
  const pathAt = useRef<string>(undefined);
  useEffect(() => {
    const i = pathAt.current ? files.findIndex((f) => f.path === pathAt.current) : -1;
    if (i >= 0) setCursor(`${i}`);
  }, [order?.key]);
  useEffect(() => {
    pathAt.current = row && row.file >= 0 ? files[row.file]?.path : undefined;
  });

  // Each view keeps where it was scrolled to: the pane is put back there
  // when it's shown again, before it's drawn.
  const pane = useRef<HTMLDivElement>(null);
  const scrolls = useRef<Partial<Record<ViewKey, number>>>({});
  const viewNow = useRef(view);
  useLayoutEffect(() => {
    if (viewNow.current === view) return;
    viewNow.current = view;
    pane.current?.scrollTo({ top: scrolls.current[view] ?? 0 });
  }, [view]);
  const onPaneScroll = () => {
    if (pane.current) scrolls.current[viewNow.current] = pane.current.scrollTop;
  };

  // Bring the cursor into view when it moves (not when the rows rebuild),
  // or when it's sent to a row it's already on (jump). At the very first
  // row, the page's top: the header and its warnings. The diff hidden,
  // it's left where it is.
  const [jump, setJump] = useState(0);
  const goTo = (key: string) => {
    show("changes");
    setCursor(key);
    setJump((n) => n + 1);
  };
  useEffect(() => {
    if (!row) return;
    // Opened on a file past the first (looking again), the page stays at
    // its top: what it is comes before what's changed. So it does when
    // the diff's first shown, there.
    if (stay.current === row.key || view !== "changes") {
      stay.current = undefined;
      return;
    }
    if (at === 0) {
      pane.current?.scrollTo({ top: 0 });
      return;
    }
    const el = row.type === "file" ? document.getElementById(`file-${row.file}`) : document.getElementById(`row-${row.key}`);
    el?.scrollIntoView({ block: row.type === "file" ? "start" : "nearest" });
  }, [row?.key, jump]);

  // The selection runs from where V was pressed to the cursor, within a
  // file.
  const selection = useMemo(() => {
    if (!selectFrom) return undefined;
    const a = nav.findIndex((r) => r.key === selectFrom);
    if (a < 0 || !row || nav[a].file !== row.file) return undefined;
    const [lo, hi] = a <= at ? [a, at] : [at, a];
    return { file: row.file, keys: new Set(nav.slice(lo, hi + 1).map((r) => r.key)) };
  }, [selectFrom, nav, at, row]);

  const move = (to: number) => {
    const r = nav[Math.min(Math.max(to, 0), nav.length - 1)];
    if (r) setCursor(r.key);
  };
  const findFrom = (from: number, step: number, test: (r: Row, i: number) => boolean) => {
    for (let i = from + step; i >= 0 && i < nav.length; i += step) if (test(nav[i], i)) return move(i);
  };
  // A change starts at a changed line whose line before (comments aside)
  // isn't one of the same file's changes.
  const lineBefore = (i: number) => {
    for (let j = i - 1; j >= 0; j--) if (nav[j].type !== "comment") return nav[j];
    return undefined;
  };
  const startsChange = (i: number) => {
    const before = lineBefore(i);
    return isChange(nav[i]) && !(before && isChange(before) && before.file === nav[i].file);
  };

  const loadBlob = async (f: DiffFile) => {
    if (blobs[f.path] || !diff) return blobs[f.path];
    const lines = await getBlob(api, diff.head, f.path);
    setBlobs((b) => ({ ...b, [f.path]: lines }));
    return lines;
  };
  // expand shows up to n more lines on each side of a gap (all of it if n
  // is 0), loading the file first.
  const expand = async (file: number, gap: number, n: number) => {
    const f = files[file];
    try {
      await loadBlob(f);
    } catch (e) {
      fail(e as Error);
      return;
    }
    setExpanded((x) => {
      const e = x[f.path]?.[gap] ?? { top: 0, bottom: 0 };
      // Before the first hunk, lines open upwards from it; after the last,
      // downwards; between two, from both.
      const first = gap === 0;
      const last = gap === f.hunks.length;
      const next = n === 0 ? { top: 1e9, bottom: 0 } : { top: first ? 0 : e.top + n, bottom: last ? 0 : e.bottom + n };
      return { ...x, [f.path]: { ...x[f.path], [gap]: next } };
    });
  };
  const showLarge = (f: DiffFile) => {
    if (!diff) return;
    getFileDiff(api, diff.base, diff.head, f.path).then((full) => setLoadedLarge((l) => ({ ...l, [f.path]: full })), fail);
  };
  const toggleViewed = (file: number) => {
    const f = files[file];
    const marking = rs.viewed[f.path] !== fileID(f);
    setRS((s) => {
      const viewed = { ...s.viewed };
      if (marking) viewed[f.path] = fileID(f);
      else delete viewed[f.path];
      return { ...s, viewed };
    });
    setViewed(api, f.path, fileID(f), marking).then(setRS, fail);
    if (marking) {
      const after = nav.findIndex((r) => r.type === "file" && r.file > file && rs.viewed[files[r.file].path] !== fileID(files[r.file]));
      setCursor(after >= 0 ? nav[after].key : `${file}`);
    } else setCursor(`${file}`);
  };
  // A file folded because its change is as you reviewed it, or because
  // it's one to skim, opens to read.
  const toggleFolded = (file: number) => setOpened((o) => ({ ...o, [files[file].path]: !!states[file]?.folded }));

  // A view with a seen (what's been said) is seen by being looked at:
  // what's in it then is kept, and what comes while it's showing is seen
  // too. Nothing's kept until it's been read.
  const part = parts.find((p) => p.key === view);
  const seenNow = part?.seen;
  const seenKept = rs.viewed[seenKey(view)];
  useEffect(() => {
    if (!reviewsLoaded || !seenNow || seenNow === seenKept) return;
    const key = seenKey(view);
    setRS((s) => ({ ...s, viewed: { ...s.viewed, [key]: seenNow } }));
    setViewed(api, key, seenNow, true).then(setRS, fail);
  }, [view, seenNow, seenKept, reviewsLoaded, api]);

  const selectedRows = useMemo(() => (selection ? nav.filter((r) => selection.keys.has(r.key)) : []), [selection, nav]);
  const selectedTarget = useMemo(() => targetOf(selectedRows, selectSide), [selectedRows, selectSide]);

  // picked is the lines to act on: those selected, or the cursor's.
  const picked = () => {
    const target = selection ? selectedTarget : row?.type === "line" ? targetOf([row]) : undefined;
    const file = selection ? selection.file : row?.file;
    return target && file !== undefined ? { file, target } : undefined;
  };
  // comment opens the editor on the selected lines, or the cursor's.
  const comment = () => {
    if (source.readOnly) return toast(source.readOnly);
    const on = picked();
    if (!diff || !on) return;
    setEditing(on);
    select(undefined);
  };
  // ask asks an agent about them.
  const ask = () => {
    const on = picked();
    if (!diff || !source.onAsk) return;
    if (!on) return source.onAsk(); // no lines under the cursor: about it all
    source.onAsk({ path: files[on.file].path, ...on.target, commit: on.target.side === "new" ? diff.head : diff.base });
    select(undefined);
  };
  // select starts (or, with no key, ends) a selection.
  const select = (key?: string, side?: "old" | "new") => {
    setSelectFrom(key);
    setSelectSide(side);
  };

  // Selecting lines: shift and the arrows from the keyboard (V and j/k too);
  // with the mouse, dragging the line numbers, or a click on one, and
  // shift-click to extend. A plain click elsewhere ends it.
  const sameFile = (a?: string, b?: string) => a !== undefined && b !== undefined && a.split(":")[0] === b.split(":")[0];
  // extend grows the selection by a line: over comments, but not across a
  // gap (the lines either side of one aren't together) or out of the file.
  const extend = (step: number) => {
    if (row?.type !== "line") return;
    let i = at + step;
    while (nav[i]?.type === "comment") i += step;
    const next = nav[i];
    if (next?.type !== "line" || next.file !== row.file) return;
    if (!selectFrom) select(row.key);
    setCursor(next.key);
  };
  const dragging = useRef(false);
  useEffect(() => {
    const up = () => (dragging.current = false);
    window.addEventListener("mouseup", up);
    return () => window.removeEventListener("mouseup", up);
  }, []);
  const rowClick = (key: string, shift: boolean) => {
    if (shift && sameFile(cursor, key)) {
      if (!selectFrom) select(cursor);
    } else select(undefined);
    setCursor(key);
  };
  const gutterDown = (key: string, shift: boolean, side?: "old" | "new") => {
    dragging.current = true;
    if (!(shift && sameFile(cursor, key))) select(key, side);
    else if (!selectFrom) select(cursor, side);
    setCursor(key);
  };
  const dragOver = (key: string) => {
    if (dragging.current && sameFile(selectFrom, key)) setCursor(key);
  };
  // act is what's done with a pass's finding: taken into the draft (which
  // is then read again), dismissed, or asked about.
  const act: FindingAct = (on, what, why) => {
    const f = on.finding;
    setDismissing(undefined);
    if (what === "ask") {
      const title = source.findings?.find((x) => x.pass === on.pass)?.title ?? on.pass;
      const about = { pass: title, kind: f.kind, text: f.text };
      if (!diff || !f.path || !f.line) return source.onAsk?.(undefined, about);
      return source.onAsk?.({ path: f.path, side: f.side === "old" ? "old" : "new", line: f.line, endLine: f.endLine ?? f.line, commit: f.commit ?? diff.head }, about);
    }
    if (what === "open" || !source.onFinding) return;
    source.onFinding(on.pass, f.key, what, why).then(() => (what === "taken" ? loadReview() : undefined), fail);
  };
  const activate = (r: Row) => {
    if (r.type === "finding") act(r.found, "taken");
    else if (r.type === "gap") expand(r.file, r.gap, r.count !== undefined && r.count <= 40 ? 0 : 20);
    else if (r.type === "note" && r.large) showLarge(files[r.file]);
    else if (r.type === "file" && (states[r.file]?.unchanged || files[r.file]?.skim) && !states[r.file].viewed) toggleFolded(r.file);
    else if (r.type === "file") toggleViewed(r.file);
    else if (r.type === "comment" && r.placed.draft && !r.placed.note.path) toast("It's on the change as a whole, and goes in your review's body: D deletes it.");
    else if (r.type === "comment" && r.placed.draft) {
      const n = r.placed.note;
      setEditing({ file: r.file, target: n, noteID: n.id, note: n });
    } else if (r.type === "comment") setReplying(r.placed.note.id);
  };
  // answer is you on a submitted comment: a reply, resolving or reopening.
  // One kept elsewhere (a pull request's thread) is answered there.
  const answer = (id: string, what: "reply" | "resolve" | "reopen", body = "") => {
    const sent = source.threads && source.onThread ? source.onThread(id, what, body) : respond(api, id, what, body).then(setRS);
    return sent.then(() => {
      if (what === "reply") setReplying(undefined);
    }, fail);
  };
  const toggleResolved = (r: Row) => {
    if (r.type !== "comment" || r.placed.draft) return false;
    answer(r.placed.note.id, r.placed.note.status === "resolved" ? "reopen" : "resolve");
    return true;
  };
  const saveNote = async (n: NoteDraft) => {
    if (!editing || !diff) return;
    const f = files[editing.file];
    try {
      const next = editing.noteID
        ? await editNote(api, editing.noteID, n)
        : await addNote(api, { base: diff.base, head: diff.head }, { ...n, path: f.path, ...editing.target });
      setRS(next);
      closeEditor();
      setError(undefined);
    } catch (e) {
      fail(e as Error);
    }
  };
  // closeEditor puts the cursor back on the comment it was rewriting (the
  // editor stood in for it, so the cursor had to go elsewhere meanwhile).
  const closeEditor = () => {
    if (editing?.noteID) setCursor(`${editing.file}:c:${editing.noteID}`);
    setEditing(undefined);
  };
  const removeNote = (r: Row) => {
    if (r.type !== "comment" || !r.placed.draft) return;
    deleteNote(api, r.placed.note.id).then(setRS, fail);
  };

  // Stable callbacks: FileSection only redraws when its own rows, cursor or
  // selection change.
  const handlers = useRef({ activate, saveNote, cancel: closeEditor, comment, ask, rowClick, gutterDown, dragOver, answer, toggleFolded, act });
  handlers.current = { activate, saveNote, cancel: closeEditor, comment, ask, rowClick, gutterDown, dragOver, answer, toggleFolded, act };
  const onAct = useCallback<FindingAct>((on, what, why) => handlers.current.act(on, what, why), []);
  // toFinding shows the changes with the cursor on a finding (from the
  // list of them in Passes), opening its file if it's folded away. It
  // says whether it's there to go to: one on a file the change doesn't
  // touch, or found at another commit, isn't.
  const toFinding = (pass: string, key: string) => {
    const file = files.findIndex((f) => found[f.path]?.some((x) => x.pass === pass && x.finding.key === key));
    if (file < 0) return false;
    setOpened((o) => ({ ...o, [files[file].path]: true }));
    if (rs.viewed[files[file].path] === fileID(files[file])) toggleViewed(file);
    goTo(`${file}:f:${pass}:${key}`);
    return true;
  };
  // toComment shows the changes with the cursor on a comment, if it's in
  // them.
  const toComment = (id: string) => {
    const r = nav.find((x) => x.type === "comment" && x.placed.note.id === id);
    if (r) goTo(r.key);
    return !!r;
  };
  // The list of files is dragged wider or narrower by its edge; a double
  // click puts it back.
  const resizeFiles = (e: React.PointerEvent<HTMLDivElement>) => {
    e.preventDefault();
    const handle = e.currentTarget;
    const left = handle.parentElement?.getBoundingClientRect().left ?? 0;
    handle.setPointerCapture(e.pointerId);
    let width: number | undefined;
    const move = (ev: PointerEvent) => {
      width = Math.round(Math.min(FILES_MAX, Math.max(FILES_MIN, ev.clientX - left)));
      setFilesWidth(width);
    };
    const up = () => {
      handle.removeEventListener("pointermove", move);
      handle.removeEventListener("pointerup", up);
      if (width) keepFilesWidth(width);
      setFilesWidth(undefined);
    };
    handle.addEventListener("pointermove", move);
    handle.addEventListener("pointerup", up);
  };
  const resetFilesWidth = () => {
    setFilesWidth(undefined);
    keepFilesWidth(undefined);
  };
  const onAnswer = useCallback((id: string, what: "reply" | "resolve" | "reopen", body?: string) => handlers.current.answer(id, what, body), []);
  const onReplyCancel = useCallback(() => setReplying(undefined), []);
  const onActivate = useCallback((r: Row) => handlers.current.activate(r), []);
  const onSave = useCallback((n: NoteDraft) => handlers.current.saveNote(n), []);
  const onCancel = useCallback(() => handlers.current.cancel(), []);
  const onComment = useCallback(() => handlers.current.comment(), []);
  const onAsk = useCallback(() => handlers.current.ask(), []);
  const onFold = useCallback((file: number) => handlers.current.toggleFolded(file), []);
  const onRowClick = useCallback((key: string, shift: boolean) => handlers.current.rowClick(key, shift), []);
  const onGutterDown = useCallback((key: string, shift: boolean, side?: "old" | "new") => handlers.current.gutterDown(key, shift, side), []);
  const onDragOver = useCallback((key: string) => handlers.current.dragOver(key), []);

  const toggleSplit = () => setSplit((s) => !s);
  const toggleSince = () => sinceable !== undefined && setScope((s) => (s === "auto" ? "all" : "auto"));
  const submit = () => {
    if (!diff) return;
    if (source.readOnly) return toast(source.readOnly);
    setSubmitting(true);
  };
  const reload = load;

  // Following names: where each file's sides are read, and the hook that
  // draws the card and the panel.
  const navAts = useMemo(
    () => files.map((f): NavAt | undefined => (diff ? { old: { rev: diff.base, path: f.oldPath ?? f.path }, new: { rev: diff.head, path: f.path } } : undefined)),
    [files, diff?.base, diff?.head],
  );
  const codeNav = useCodeNav({
    api,
    diff,
    files,
    rows,
    states,
    pane,
    cursorRow: row,
    expand,
    goToRow: (key) => {
      setCursor(key);
      window.setTimeout(() => {
        const el = document.getElementById(`row-${key}`);
        if (!el) return;
        el.scrollIntoView({ block: "center" });
        el.classList.remove("nav-flash");
        void el.offsetWidth;
        el.classList.add("nav-flash");
      }, 30);
    },
    browse: source.browse ? (path, line) => `${source.browse!.split("?")[0]}?${new URLSearchParams({ path })}#L${line}` : undefined,
  });
  // The diff's keys work while it's showing; elsewhere the arrows scroll.
  const diffKeys: KeyMap = {
    j: () => move(at + 1),
    ArrowDown: () => move(at + 1),
    k: () => move(at - 1),
    ArrowUp: () => move(at - 1),
    "mod+d": () => move(at + 20),
    "mod+u": () => move(at - 20),
    g: () => move(0),
    G: () => move(nav.length - 1),
    n: () => findFrom(at, 1, (_, i) => startsChange(i)),
    p: () => {
      // To the start of the change before the one the cursor is in.
      let i = at;
      while (i > 0 && (nav[i].type === "comment" || (isChange(nav[i]) && !startsChange(i)))) i--;
      findFrom(i, -1, (_, j) => startsChange(j));
    },
    J: () => findFrom(at, 1, isHead),
    K: () => {
      const start = nav.findLastIndex((r, i) => i <= at && isHead(r));
      findFrom(start === at ? at : start + 1, -1, isHead);
    },
    x: () => {
      if (!row) return;
      if (!toggleResolved(row)) toggleViewed(row.file);
    },
    i: toggleSince,
    Enter: () => row && activate(row),
    c: comment,
    ...(source.onAsk ? { a: () => (row?.type === "finding" ? act(row.found, "ask") : ask()) } : {}),
    V: () => row?.type === "line" && select(selectFrom ? undefined : row.key),
    "shift+ArrowDown": () => extend(1),
    "shift+ArrowUp": () => extend(-1),
    D: () => (row?.type === "finding" ? setDismissing(row.key) : row && removeNote(row)),
    e: () => {
      if (!row || row.type !== "line") return;
      // The gaps either side of the hunk the cursor is in.
      const fileNav = rows[row.file];
      const i = fileNav.findIndex((r) => r.key === row.key);
      const before = fileNav.slice(0, i).findLast((r) => r.type === "gap");
      const after = fileNav.slice(i).find((r) => r.type === "gap");
      if (before?.type === "gap") expand(row.file, before.gap, 20);
      if (after?.type === "gap") expand(row.file, after.gap, 20);
    },
    u: toggleSplit,
    ...codeNav.keys,
  };
  useKeys({
    ...(view === "changes" ? diffKeys : {}),
    ...Object.fromEntries(viewKeys.map((v, i) => [`${i + 1}`, () => show(v)])),
    s: submit,
    t: () => setFinder(true),
    r: load,
    Escape: () => {
      if (source.dismiss?.()) return;
      if (selectFrom && view === "changes") select(undefined);
      else navigate(source.back.to);
    },
    ...source.keys,
  });

  const totals = files.reduce((t, f) => ({ a: t.a + f.additions, d: t.d + f.deletions }), { a: 0, d: 0 });
  const noteCounts = useMemo(() => Object.fromEntries(Object.entries(notes).map(([path, list]) => [path, list.length])), [notes]);
  const foundCounts = useMemo(() => Object.fromEntries(Object.entries(found).map(([path, list]) => [path, list.length])), [found]);
  const moved = diff && source.headNow && source.headNow !== diff.head;
  const draft = rs.draft;
  const sinceName = source.sinceName?.(latest) ?? latest?.id ?? "your last review";
  // Comments still to settle, where they're shown from: GitHub's threads,
  // or your last review kept here.
  const openComments = (threads ?? latest)?.comments.filter((c) => c.status !== "resolved").length ?? 0;
  const changes = view === "changes";
  // The views down the side: what's new in one is said until it's shown.
  const entries: ViewEntry[] = parts.map((p, i) => {
    const kept = rs.viewed[seenKey(p.key)];
    return { key: p.key, ...viewNames[p.key], n: i + 1, mark: p.mark, count: p.count, news: kept !== undefined && view !== p.key ? p.news?.(kept) : undefined };
  });

  return (
    <div className="review" style={filesWidth ? ({ "--files-w": `${filesWidth}px` } as React.CSSProperties) : undefined}>
      <aside className="files">
        {source.side}
        <ViewList views={entries} here={view} onPick={(k) => show(k as ViewKey)} />
        <FileList
          files={files}
          states={states}
          here={row?.file}
          current={changes}
          n={viewKeys.length}
          counts={noteCounts}
          found={foundCounts}
          order={order}
          guideNotes={guideNotes}
          onOrder={source.readOnly ? undefined : source.onOrder}
          onChanges={() => show("changes")}
          onPick={goTo}
          onFind={() => setFinder(true)}
        />
      </aside>
      <div className="files-resize" role="separator" aria-orientation="vertical" title="Drag to make the list of files wider or narrower; double-click to put it back" onPointerDown={resizeFiles} onDoubleClick={resetFilesWidth} />
      <div className="diff-pane" ref={pane} onScroll={onPaneScroll}>
        <Crumbs className="review-crumbs" steps={[source.back]} />
        <header className="review-head">
          <h1>{source.name}</h1>
          {diff && (
            <span className="review-scope">
              {sinceShown ? (
                <Chip tone="attention" title={diff.since ? `What's changed since ${sinceName}, at ${diff.since.slice(0, 7)}` : `Each file's change, compared with what you reviewed (${sinceName})`}>
                  <Icon name="rotate" size={11} /> Since {sinceName}
                </Chip>
              ) : null}
              <span className="dim">
                {diff.commits.length} {diff.commits.length === 1 ? "commit" : "commits"}
                {!diff.since && ` ahead of ${diff.baseRef}`}
              </span>
              <span className="num">
                <span className="plus">+{totals.a}</span> <span className="minus">−{totals.d}</span>
              </span>
            </span>
          )}
          <span className="review-actions">
            {changes && diff && sinceable !== undefined && (
              <Button variant="ghost" size="small" onClick={toggleSince} title="Switch between what's changed since your last review and all of it (i)">
                {sinceShown ? "Show all changes" : `Only since ${sinceName}`} <Keys k="i" />
              </Button>
            )}
            {source.actions}
            {source.browse && (
              <Link to={source.browse} className={buttonClass({ size: "small" }, "desktop-only")} title="Browse its files, as they are now">
                <Icon name="file" size={12} /> Browse files
              </Link>
            )}
            <Button variant="ghost" size="small" className="mobile-only" onClick={() => setFinder(true)} title="Jump to a file (t)">
              <Icon name="file" size={13} /> Files
            </Button>
            {changes && (
              <Segmented className="desktop-only" value={split ? "split" : "unified"} onChange={toggleSplit} aria-label="Diff layout">
                <Segment value="unified" title="Unified (u)">
                  <Icon name="unified" size={13} /> Unified
                </Segment>
                <Segment value="split" title="Side by side (u)">
                  <Icon name="split" size={13} /> Split
                </Segment>
              </Segmented>
            )}
            <Button variant="primary" className="submit-button" onClick={submit} unavailable={source.readOnly} title="Submit the review (s)">
              {draft ? `Submit review · ${draft.comments.length}` : "Submit review"} <Keys k="s" />
            </Button>
          </span>
        </header>
        {entries.length > 0 && (
          <nav className="review-tabs mobile-only" aria-label="Views">
            {[...entries, { key: "changes", ...viewNames.changes, n: viewKeys.length, news: undefined }].map((v) => (
              <Button variant="ghost" size="small" key={v.key} className={view === v.key ? "on" : undefined} aria-pressed={view === v.key} onClick={() => show(v.key as ViewKey)}>
                <Icon name={v.icon} size={12} /> {v.label}
                {v.news && <span className="fnews" aria-label={v.news} />}
              </Button>
            ))}
          </nav>
        )}
        {submitted && source.submitted(submitted.review, submitted.state)}
        {source.notices}
        {changes && scopeNote && (
          <Callout tone="quiet" icon="info">
            Showing all the changes: {scopeNote}
          </Callout>
        )}
        {changes && sinceShown && (
          <div className="review-status">
            <Icon name="rotate" size={13} />
            <span>
              {diff?.rebased ? (
                <>
                  It's been rebased since {sinceName}, so each file's change is compared with the one you reviewed.{" "}
                  {unchanged.size === files.length
                    ? "Nothing changed: there's nothing to re-read."
                    : `${unchanged.size} of ${files.length} ${files.length === 1 ? "file is" : "files are"} unchanged and folded; the rest show in full.`}{" "}
                </>
              ) : (
                <>
                  Re-reviewing what's changed since {sinceName}
                  {latest?.verdict ? ` (${verdictWords[latest.verdict]})` : ""}. Its comments sit on the lines they were made on;{" "}
                </>
              )}
              {openComments ? (
                <>
                  {openComments} still to resolve: <Keys k="⏎" /> replies, <Keys k="x" /> resolves.
                </>
              ) : diff?.rebased ? null : (
                "all resolved."
              )}
            </span>
          </div>
        )}
        {moved && <Callout tone="attention" title={source.movedWords} actions={<Button onClick={reload}>Reload <Keys k="r" /></Button>} />}
        {error && <Banner>{humanError(error)}</Banner>}
        {parts.map((p) => (
          <section key={p.key} className={`review-view view-body view-${p.key}`} hidden={view !== p.key} aria-label={viewNames[p.key].label}>
            {typeof p.body === "function" ? p.body({ toFinding, toComment }) : p.body}
          </section>
        ))}
        <section className="review-view" hidden={!changes} aria-label="Changes">
          {diff && diff.uncommitted.length > 0 && (
            <Callout tone="attention" icon="alert" title={`${diff.uncommitted.length} ${diff.uncommitted.length === 1 ? "file has" : "files have"} uncommitted changes, not in this review`}>
              <span className="mono">{diff.uncommitted.slice(0, 5).join(", ")}</span>
              {diff.uncommitted.length > 5 && " …"}
            </Callout>
          )}
          {!diff && !error && <Loading what="Reading the changes…" />}
          {diff && diff.commits.length > 0 && (
            <details className="commits">
              <summary className="dim">Commits</summary>
              <ul>
                {diff.commits.map((c) => (
                  <li key={c.sha}>
                    <span className="mono dim">{c.sha.slice(0, 7)}</span> {c.subject} <span className="dim">· {c.author}</span>
                  </li>
                ))}
              </ul>
            </details>
          )}
          {rs.reviews.length > 0 && <EarlierReviews reviews={rs.reviews} />}
          {diff && !files.length && <p className="dim">{source.empty}</p>}
          {files.map((f, i) => (
            <Fragment key={f.path}>
              {guideHeads?.has(i) && <GuideGroupHead group={guideHeads.get(i)!} />}
              <FileSection
                index={i}
                file={f}
                rows={rows[i]}
                state={states[i]}
                split={split}
                cursor={row?.file === i ? cursor : ""}
                selected={selection?.file === i ? selection.keys : undefined}
                selectionEnd={selection?.file === i ? selectedRows.findLast((r) => r.type === "line")?.key : undefined}
                selectionLabel={selection?.file === i && selectedTarget ? targetWords(selectedTarget) : undefined}
                editing={editing?.file === i ? editing : undefined}
                highlighting={highlighting}
                onRow={setCursor}
                onActivate={onActivate}
                onSave={onSave}
                onCancel={onCancel}
                onComment={source.readOnly ? undefined : onComment}
                onAsk={source.onAsk ? onAsk : undefined}
                onFold={onFold}
                onRowClick={onRowClick}
                onGutterDown={onGutterDown}
                onDragOver={onDragOver}
                replying={replying}
                onAnswer={onAnswer}
                onReplyCancel={onReplyCancel}
                dismissing={row?.file === i ? dismissing : undefined}
                onAct={onAct}
                onDismissing={setDismissing}
                followups={source.followups}
                note={guideNotes?.[f.path]}
                navAt={navAts[i]}
              />
            </Fragment>
          ))}
          {orphans.length > 0 && (
            <section className="file orphans">
              <div className="file-head">
                <span className="dim">Comments on files {sinceShown ? `not changed since ${sinceName}` : "not in this diff"}</span>
              </div>
              <div className="lines">
                {orphans.map(
                  (r) =>
                    r.type === "comment" && (
                      <div key={r.key}>
                        <div className="mono dim orphan-path">{r.placed.note.path || "On the change as a whole: it goes in the review's body"}</div>
                        <CommentCard
                          row={r}
                          cursor={cursor === r.key}
                          replying={replying === r.placed.note.id}
                          onRow={setCursor}
                          onAnswer={onAnswer}
                          onReplyCancel={onReplyCancel}
                        />
                      </div>
                    ),
                )}
              </div>
            </section>
          )}
        </section>
      </div>
      {source.foot && <div className="review-foot">{source.foot}</div>}
      {codeNav.card}
      {codeNav.picker}
      {codeNav.panel}
      {finder && (
        <FileJump
          files={files}
          onPick={(i) => {
            setFinder(false);
            goTo(`${i}`);
          }}
          onClose={() => setFinder(false)}
        />
      )}
      {submitting &&
        diff &&
        source.submit({
          draft,
          diff,
          onClose: () => setSubmitting(false),
          onDiscard: () =>
            discardReview(api).then((next) => {
              setRS(next);
              setSubmitting(false);
            }, fail),
          done: (next) => {
            setRS(next);
            setSubmitted({ review: next.reviews[0], state: next });
            setSubmitting(false);
          },
        })}
    </div>
  );
}

function EarlierReviews({ reviews }: { reviews: ReviewData[] }) {
  return (
    <details className="commits">
      <summary className="dim">Reviews: {reviews.map((r) => `${r.id} ${verdictWords[r.verdict!]}`).join(" · ")}</summary>
      {reviews.map((r) => (
        <div key={r.id} className="earlier">
          <div>
            <strong>{r.id}</strong>{" "}
            <span className="dim">
              {verdictWords[r.verdict!]} · {r.submitted && <time title={exact(r.submitted)}>{stamp(r.submitted)}</time>}
            </span>
          </div>
          {r.summary && <p className="summary">{r.summary}</p>}
          <ul>
            {r.comments.map((n) => (
              <li key={n.id}>
                {n.kind && <span className={`kind k-${n.kind}`}>{n.kind}</span>}{" "}
                <span className="mono dim">
                  {n.path}:{n.line}
                </span>{" "}
                {n.body}
              </li>
            ))}
          </ul>
        </div>
      ))}
    </details>
  );
}

// highlight colours a file's lines: each hunk's sides as wholes, and the
// file at head for lines opened up from gaps. (An artifact's source diff
// is coloured the same.)
export function highlight(f: DiffFile, blob: string[] | undefined): Highlighted {
  const lang = languageOf(f.path);
  const out: Highlighted = { old: new Map(), new: new Map(), shown: [] };
  for (const h of f.hunks) {
    const olds = h.lines.filter((l) => l.k !== "a");
    const news = h.lines.filter((l) => l.k !== "d");
    highlightLines(olds.map((l) => l.t), lang).forEach((html, i) => out.old.set(olds[i].o!, html));
    highlightLines(news.map((l) => l.t), lang).forEach((html, i) => out.new.set(news[i].n!, html));
  }
  if (blob) out.shown = highlightLines(blob, lang);
  return out;
}

interface FileSectionProps {
  index: number;
  file: DiffFile;
  rows: Row[];
  state: FileState;
  split: boolean;
  cursor: string; // the cursor's row, if it's in this file
  selected?: Set<string>;
  selectionEnd?: string; // the selection's last row, where its bar goes
  selectionLabel?: string;
  editing?: Editing;
  highlighting: boolean;
  onRow: (key: string) => void;
  onActivate: (r: Row) => void;
  onSave: (n: NoteDraft) => void;
  onCancel: () => void;
  onComment?: () => void; // absent where comments can't be made
  onAsk?: () => void; // ask an agent about the selected lines
  onFold: (file: number) => void; // open or fold a file unchanged since you reviewed it
  onRowClick: (key: string, shift: boolean) => void;
  onGutterDown: (key: string, shift: boolean, side?: "old" | "new") => void;
  onDragOver: (key: string) => void;
  replying?: string; // the comment you're answering
  onAnswer: (id: string, what: "reply" | "resolve" | "reopen", body?: string) => void;
  onReplyCancel: () => void;
  dismissing?: string; // the finding being dismissed, if it's in this file
  onAct: FindingAct;
  onDismissing: (key?: string) => void;
  followups?: ReviewSource["followups"];
  note?: string; // what the change does to it, in a line (a guide's)
  navAt?: NavAt;
}

// GuideGroupHead names a group of the guide's in the diff, above its
// first file: what its files are, and why they're read here.
function GuideGroupHead({ group }: { group: FileGroup }) {
  const n = group.files.length;
  return (
    <div className={`guide-group-head${group.skim ? " skim" : ""}`}>
      <h3>
        <Icon name="sparkle" size={13} /> {group.label}
        <span className="dim">
          {" "}
          · {n} {n === 1 ? "file" : "files"}
          {group.skim ? " · to skim" : ""}
        </span>
      </h3>
      {group.why && (
        <p>
          <Inline text={group.why} />
        </p>
      )}
    </div>
  );
}

const FileSection = memo(function FileSection(props: FileSectionProps) {
  const { index, file, rows, state, split, cursor, selected, selectionEnd, selectionLabel, editing, highlighting } = props;
  const { onRow, onActivate, onSave, onCancel, onComment, onAsk, onFold, onRowClick, onGutterDown, onDragOver, replying, onAnswer, onReplyCancel, dismissing, onAct, onDismissing, followups, note, navAt } = props;
  // highlighting is a dependency: once the highlighter loads, colour.
  const hl = useMemo(() => highlight(file, state.blob), [file, state.blob, highlighting]);
  const html = (c: Cell, side: "old" | "new") => cellHTML(hl, c, side);
  // The lines the editor is on, for a suggestion to start from.
  const editorLines = () => {
    if (!editing) return [];
    if (editing.note) return editing.note.excerpt;
    const t = editing.target;
    const out: string[] = [];
    for (const r of rows) {
      if (r.type !== "line") continue;
      const c = t.side === "new" ? r.right : r.left?.kind === "d" ? r.left : undefined;
      if (c && c.num >= t.line && c.num <= t.endLine) out.push(c.text);
    }
    return out;
  };
  return (
    <section id={`file-${index}`} className={`file${state.viewed ? " viewed" : ""}${state.folded && !state.viewed ? " folded" : ""}`}>
      <div id={`row-${index}`} className={`file-head ${cursor === `${index}` ? "cursor" : ""}`} onClick={() => onRow(`${index}`)}>
        <span className={`fstatus s-${file.status}`}>{capital(file.status)}</span>
        <span className="mono">
          {file.oldPath && <span className="dim">{file.oldPath} → </span>}
          {file.path}
        </span>
        <span className="counts">
          {file.additions > 0 && <span className="plus">+{file.additions}</span>}{" "}
          {file.deletions > 0 && <span className="minus">−{file.deletions}</span>}
        </span>
        {state.unchanged && (
          <Button
            variant="ghost" size="small"
            className="unchanged-mark"
            onClick={(e) => (e.stopPropagation(), onFold(index))}
            title="Its change is the same as when you reviewed it: a rebase moved it, and nothing else"
          >
            <Icon name="check" size={11} /> Unchanged since you reviewed · {state.folded ? "show" : "hide"}
          </Button>
        )}
        {file.skim && !state.unchanged && (
          <Button variant="ghost" size="small" className="skim-mark" onClick={(e) => (e.stopPropagation(), onFold(index))} title="It's one to skim, so it's folded: it still counts until you mark it viewed">
            {skimWords[file.skim]} · {state.folded ? "show" : "hide"}
          </Button>
        )}
        <Checkbox className="viewed-box" checked={!!state.viewed} onChange={() => onActivate({ type: "file", key: `${index}`, file: index })} onClick={(e) => e.stopPropagation()}>
          Viewed
        </Checkbox>
      </div>
      {note && !state.viewed && (
        <p className="file-guide" title="What the guide says the change does to it">
          <Icon name="sparkle" size={13} />
          <span>
            <Inline text={note} />
          </span>
        </p>
      )}
      {!state.viewed && !state.folded && (
        <div className={`lines ${split ? "split" : "unified"}`}>
          {rows.slice(1).map((r) =>
            r.type === "editor" && editing ? (
              <NoteEditor key={r.key} editing={editing} lines={editorLines()} onSave={onSave} onCancel={onCancel} />
            ) : r.type === "comment" ? (
              <CommentCard
                key={r.key}
                row={r}
                cursor={cursor === r.key}
                replying={replying === r.placed.note.id}
                onRow={onRow}
                onAnswer={onAnswer}
                onReplyCancel={onReplyCancel}
                followup={r.placed.draft ? undefined : followups?.[r.placed.note.id]}
              />
            ) : r.type === "finding" ? (
              <FindingCard key={r.key} row={r} cursor={cursor === r.key} dismissing={dismissing === r.key} canAsk={!!onAsk} onRow={onRow} onAct={onAct} onDismissing={onDismissing} />
            ) : (
              <RowView
                key={r.key}
                row={r}
                split={split}
                navAt={navAt}
                cursor={cursor === r.key}
                selected={!!selected?.has(r.key)}
                html={html}
                onRowClick={onRowClick}
                onGutterDown={onGutterDown}
                onDragOver={onDragOver}
                onActivate={onActivate}
                bar={
                  r.key === selectionEnd && selectionLabel ? (
                    // It floats over the lines below rather than pushing them
                    // down: nothing moves under the mouse while dragging.
                    <div className="select-bar" onClick={(e) => e.stopPropagation()}>
                      <span>{selectionLabel}</span>
                      {onComment && (
                        <Button variant="primary" onMouseDown={(e) => e.preventDefault()} onClick={onComment}>
                          Comment <Keys k="c" />
                        </Button>
                      )}
                      {onAsk && (
                        <Button onMouseDown={(e) => e.preventDefault()} onClick={onAsk} title="Ask an agent about these lines, with the code checked out here">
                          Ask <Keys k="a" />
                        </Button>
                      )}
                      <span className="dim">
                        <Keys k="esc" /> clears
                      </span>
                    </div>
                  ) : undefined
                }
              />
            ),
          )}
        </div>
      )}
    </section>
  );
});

// NavAt is where a file's sides are read, for code navigation: the
// diff's base and head, and the file's path at each.
export interface NavAt {
  old: { rev: string; path: string };
  new: { rev: string; path: string };
}

interface RowViewProps {
  row: Row;
  split: boolean;
  navAt?: NavAt;
  cursor: boolean;
  selected: boolean;
  html: (c: Cell, side: "old" | "new") => string;
  onRowClick: (key: string, shift: boolean) => void;
  onGutterDown: (key: string, shift: boolean, side?: "old" | "new") => void;
  onDragOver: (key: string) => void;
  onActivate: (r: Row) => void;
  bar?: React.ReactNode; // the selection's bar, on its last row
}

// RowView is a row of the diff: a line, a gap, or a note. (An artifact's
// source diff draws its rows with it, as a review's.)
export function RowView({ row, split, navAt, cursor, selected, html, onRowClick, onGutterDown, onDragOver, onActivate, bar }: RowViewProps) {
  const cls = (cursor ? " cursor" : "") + (selected ? " selected" : "");
  if (row.type === "gap") {
    return (
      <div id={`row-${row.key}`} className={`gap${cls}`} onClick={() => onActivate(row)}>
        <span className="gap-mark" aria-hidden>
          <Icon name={row.count === undefined ? "chevron-down" : "more"} size={13} />
        </span>
        <span className="gap-words">{row.count === undefined ? "Show more" : `Show ${row.count} unchanged ${row.count === 1 ? "line" : "lines"}`}</span>
        {row.section && <span className="gap-section">{row.section}</span>}
      </div>
    );
  }
  if (row.type === "note") {
    return (
      <div id={`row-${row.key}`} className={`note${cls}${row.large ? " clickable" : ""}`} onClick={() => row.large && onActivate(row)}>
        {row.text}
      </div>
    );
  }
  if (row.type !== "line") return null;
  // A cell says where its line is read, for following names in it.
  const code = (c?: Cell, side?: "old" | "new") => {
    if (!c) return <code className="empty" />;
    const s = side ?? (c.kind === "d" ? "old" : "new");
    const at = navAt?.[s];
    return (
      <code
        className={`${c.kind}${c.shown ? " shown" : ""}`}
        data-rev={at?.rev}
        data-path={at?.path}
        data-line={c.num}
        data-side={s}
        dangerouslySetInnerHTML={{ __html: html(c, s) + (c.noeol ? '<span class="noeol">⊘</span>' : "") }}
      />
    );
  };
  // The line numbers are where lines are picked: press there and drag to
  // select, without selecting the text (which stays copyable).
  const gutter = (side?: "old" | "new") => ({
    onMouseDown: (e: React.MouseEvent) => {
      if (e.button !== 0) return;
      e.preventDefault();
      onGutterDown(row.key, e.shiftKey, side);
    },
    onClick: (e: React.MouseEvent) => e.stopPropagation(),
  });
  const line = {
    id: `row-${row.key}`,
    onClick: (e: React.MouseEvent) => onRowClick(row.key, e.shiftKey),
    onMouseEnter: () => onDragOver(row.key),
  };
  if (split) {
    return (
      <div {...line} className={`dl${cls}`}>
        <span className={`ln gutter ${row.left?.kind ?? "empty"}`} {...gutter("old")}>
          {row.left?.num}
        </span>
        {code(row.left, "old")}
        <span className={`ln gutter ${row.right?.kind ?? "empty"}`} {...gutter("new")}>
          {row.right?.num}
        </span>
        {code(row.right, "new")}
        {bar}
      </div>
    );
  }
  const c = row.right ?? row.left!;
  const kind = row.left && !row.right ? "d" : row.right && !row.left ? "a" : "c";
  return (
    <div {...line} className={`dl ${kind}${c.shown ? " shown" : ""}${cls}`}>
      <span className="ln gutter" {...gutter()}>
        {row.left?.num}
      </span>
      <span className="ln gutter" {...gutter()}>
        {row.right?.num}
      </span>
      <span className="sign gutter" {...gutter()}>
        {kind === "a" ? "+" : kind === "d" ? "−" : ""}
      </span>
      {code({ ...c, kind }, kind === "d" ? "old" : "new")}
      {bar}
    </div>
  );
}

const linesWord = (t: Target) => (t.endLine > t.line ? `lines ${t.line}–${t.endLine}` : `line ${t.line}`);


interface CommentCardProps {
  row: Extract<Row, { type: "comment" }>;
  followup?: { kind: string; text: string; blocks?: boolean }; // what the follow-up said of it
  cursor: boolean;
  replying: boolean;
  onRow: (key: string) => void;
  onAnswer: (id: string, what: "reply" | "resolve" | "reopen", body?: string) => void;
  onReplyCancel: () => void;
}

// CommentCard is a comment: a draft's (yours to edit), or a submitted one,
// with what crew and you have said since, and resolving it.
function CommentCard({ row, cursor, replying, onRow, onAnswer, onReplyCancel, followup }: CommentCardProps) {
  const { note, draft } = row.placed;
  const resolved = note.status === "resolved";
  return (
    <NoteCard id={`row-${row.key}`} className="diff-note" cursor={cursor} draft={draft} resolved={resolved} onClick={() => onRow(row.key)}>
      <NoteHead
        end={
          <>
            {!draft &&
              (note.url ? (
                <NoteId href={note.url} title="Open it on GitHub">
                  GitHub
                </NoteId>
              ) : (
                <NoteId>{note.id}</NoteId>
              ))}
            {cursor && draft && (
              <span className="dim note-keys">
                <Keys k="⏎" /> edit <Keys k="D" /> delete
              </span>
            )}
            {!draft && (
              <Button
                variant="ghost" size="small"
                className="note-toggle"
                onClick={(e) => (e.stopPropagation(), onRow(row.key), onAnswer(note.id, resolved ? "reopen" : "resolve"))}
                title={`${resolved ? "Reopen" : "Resolve"} (x)`}
              >
                {resolved ? "Reopen" : "Resolve"}
              </Button>
            )}
          </>
        }
      >
        {note.kind ? <span className={`kind k-${note.kind}`}>{capital(note.kind)}</span> : <span className="kind k-none">Comment</span>}
        {note.by && (
          <span className="note-by" title={note.by === "agent" ? "An agent proposed it, and you added it to your review: it's yours to edit" : `${note.by} on GitHub`}>
            <Icon name={note.by === "agent" ? "sparkle" : "user"} size={11} /> {note.by === "agent" ? "Agent's" : note.by}
          </span>
        )}
        {draft ? <NoteStatus>Draft</NoteStatus> : <NoteStatus status={note.status}>{statusWords[note.status] ?? note.status}</NoteStatus>}
        <span className="dim note-where">
          {note.side === "old" ? "removed " : ""}
          {linesWord(note)}
          {!row.placed.at && ` · made on ${note.commit.slice(0, 7)}, not in this diff`}
        </span>
      </NoteHead>
      {row.unplaced && note.excerpt?.length > 0 && !(resolved && !cursor) && <Excerpt lines={note.excerpt} from={note.line} title="The lines it's on, not showing above" />}
      {followup && !resolved && (
        <div className={`note-followup k-${followup.kind}${followup.blocks ? " blocks" : ""}`} title="What the Follow-up pass said of this comment: whether what's been pushed since deals with it">
          <span className="finding-pass">
            <Icon name="sparkle" size={11} /> Follow-up
          </span>
          <strong>{followWords[followup.kind] ?? followup.kind}</strong>
          <Inline text={followup.text} className="note-said" />
        </div>
      )}
      {!(resolved && !cursor) && (
        <>
          {note.body && <NoteBody>{note.body}</NoteBody>}
          {note.suggestion != null && <Suggestion from={note.excerpt} to={note.suggestion} />}
          {note.thread.length > 0 && (
            <NoteThread>
              {note.thread.map((t, i) => (
                <NoteTurn key={i} who={t.by === "you" ? "You" : t.by} said={t.body}>
                  {t.status && <NoteStatus status={t.status === "open" ? "reopened" : t.status}>{t.status === "open" ? "Reopened" : statusWords[t.status]}</NoteStatus>}
                  {t.commit && <span className="mono dim">{t.commit.slice(0, 7)}</span>}
                </NoteTurn>
              ))}
            </NoteThread>
          )}
          {!draft && !replying && cursor && (
            <NoteHint>
              <Keys k="⏎" /> reply
            </NoteHint>
          )}
          {replying && <ReplyBox onSend={(body) => onAnswer(note.id, "reply", body)} onCancel={onReplyCancel} />}
        </>
      )}
    </NoteCard>
  );
}

function NoteEditor({ editing, lines, onSave, onCancel }: { editing: Editing; lines: string[]; onSave: (n: NoteDraft) => void; onCancel: () => void }) {
  const note = editing.note;
  const [kind, setKind] = useState<NoteKind>(note?.kind || "suggestion");
  const [text, setText] = useState(() => (note ? joinSuggestion(note.body, note.suggestion) : ""));
  const area = useRef<HTMLTextAreaElement>(null);
  // While writing, the page's keys are off: these are the editor's.
  useKeys({}, { modal: true });
  useEffect(() => {
    area.current?.focus();
    area.current?.scrollIntoView({ block: "nearest" });
  }, []);
  const save = () => {
    const { body, suggestion } = splitSuggestion(text);
    if (!body && suggestion == null) return;
    onSave({ kind, body, suggestion });
  };
  const suggest = () => {
    if (suggestionBlock.test(text)) return;
    setText((t) => joinSuggestion(t.trim(), lines.join("\n")));
    area.current?.focus();
  };
  return (
    <div className="note-editor">
      <div className="note-head">
        {noteKinds.map((k) => (
          <button key={k} className={`kind k-${k}${k === kind ? " on" : ""}`} onClick={() => setKind(k)} aria-pressed={k === kind}>
            {capital(k)}
          </button>
        ))}
        <span className="dim">
          {editing.target.side === "old" ? "removed " : ""}
          {linesWord(editing.target)} · <Keys k="tab" /> kind
        </span>
      </div>
      <TextArea
        ref={area}
        value={text}
        rows={Math.min(12, Math.max(3, text.split("\n").length + 1))}
        placeholder="What should change, or what do you want to know?"
        onChange={(e) => setText(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
            e.preventDefault();
            save();
          } else if (e.key === "Escape") {
            e.preventDefault();
            onCancel();
          } else if (e.key === "Tab") {
            e.preventDefault();
            const i = noteKinds.indexOf(kind);
            setKind(noteKinds[(i + (e.shiftKey ? noteKinds.length - 1 : 1)) % noteKinds.length]);
          }
        }}
      />
      <div className="note-actions">
        <Button onClick={suggest} disabled={suggestionBlock.test(text)}>
          Suggest a change
        </Button>
        <span className="spacer-left" />
        <Button onClick={onCancel}>
          Cancel <Keys k="esc" />
        </Button>
        <Button variant="primary" onClick={save}>
          {note ? "Save" : "Comment"} <Keys k="mod+⏎" />
        </Button>
      </div>
    </div>
  );
}

// FileJump jumps to a file by a few letters of its path.
function FileJump({ files, onPick, onClose }: { files: DiffFile[]; onPick: (i: number) => void; onClose: () => void }) {
  const [q, setQ] = useState("");
  const matches = useMemo(() => {
    return files
      .flatMap((f, i) => {
        const m = fuzzy(f.path, q);
        return m ? [{ f, i, score: m.score }] : [];
      })
      .sort((a, b) => b.score - a.score || a.i - b.i)
      .slice(0, 50);
  }, [files, q]);
  return (
    <Finder aria-label="Jump to a file" placeholder="Jump to a file" value={q} onChange={setQ} count={matches.length} onPick={(i) => onPick(matches[i].i)} onClose={onClose}>
      {matches.map((m, i) => (
        <FinderRow key={m.f.path} index={i}>
          <span className="mono">{m.f.path}</span>
        </FinderRow>
      ))}
      {!matches.length && <FinderNone>No file matches “{q}”.</FinderNone>}
    </Finder>
  );
}
