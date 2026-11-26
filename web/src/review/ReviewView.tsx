import { Fragment, useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router";
import {
  addNote,
  deleteNote,
  type DiffFile,
  discardReview,
  editNote,
  getBlob,
  getFileDiff,
  getReview,
  type NoteDraft,
  type Review as ReviewData,
  type ReviewNote,
  type ReviewState,
  respond,
  setViewed,
} from "../api";
import { type KeyMap, useKeys } from "../keys";
import { useLatest } from "../load";
import { usePref } from "../prefs";
import { Banner } from "../ui/Banner";
import { Button, buttonClass } from "../ui/Button";
import { Callout } from "../ui/Callout";
import type { DiffPick } from "../ui/diffLayout";
import { Icon, type IconName } from "../ui/Icon";
import { Keys } from "../ui/Keys";
import { Loading } from "../ui/Loading";
import { Segment, Segmented } from "../ui/Segmented";
import { humanError } from "../ui/strings";
import { toast } from "../ui/toast";
import { useMedia } from "../ui/useMedia";
import { useTitle } from "../ui/useTitle";
import { useCodeNav } from "./CodeNav";
import { CommentCard } from "./CommentCard";
import { FileList } from "./FileList";
import { FileSection } from "./FileSection";
import type { FindingAct } from "./findings";
import { groupByOrder, orderFiles } from "./files";
import { useHighlighter } from "./highlight";
import { type Editing, type FileState, fileRows, type Found, isChange, isHead, navigable, type Placed, type Row, sameLine, type Target } from "./model";
import { ReviewCommits } from "./ReviewCommits";
import { ReviewEarlier } from "./ReviewEarlier";
import { ReviewFileJump } from "./ReviewFileJump";
import { ReviewGuideGroup } from "./ReviewGuideGroup";
import { ReviewHead } from "./ReviewHead";
import { ReviewLayout } from "./ReviewLayout";
import { ReviewOrphanPath } from "./ReviewOrphanPath";
import { ReviewOrphans } from "./ReviewOrphans";
import { ReviewScope } from "./ReviewScope";
import { ReviewStatus } from "./ReviewStatus";
import { ReviewTabs } from "./ReviewTabs";
import { ReviewViewBody } from "./ReviewViewBody";
import { rowEl } from "./rowEl";
import type { NavAt, ReviewDiff, ReviewSource, ViewKey } from "./source";
import { type ViewEntry, ViewList } from "./ViewList";
import { verdictWords } from "./words";
import css from "./ReviewView.module.css";
import { Text } from "../ui/Text";

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

const viewNames: Record<ViewKey, { label: string; icon: IconName }> = {
  overview: { label: "Overview", icon: "info" },
  guide: { label: "Guide", icon: "book" },
  discussion: { label: "Discussion", icon: "message" },
  passes: { label: "Passes", icon: "list-checks" },
  changes: { label: "Changes", icon: "code" },
};

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
    const el = row.type === "file" ? document.getElementById(`file-${row.file}`) : rowEl(row.key);
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
  const rowClick = (key: string, shift: boolean) => {
    if (shift && sameFile(cursor, key)) {
      if (!selectFrom) select(cursor);
    } else select(undefined);
    setCursor(key);
  };
  // Lines picked by their numbers (a press, shift, a drag along them):
  // the selection is from the first to the last, where the cursor goes.
  const pick = (p: DiffPick) => {
    select(p.from, p.side);
    setCursor(p.to);
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
  const handlers = useRef({ activate, saveNote, cancel: closeEditor, comment, ask, rowClick, pick, answer, toggleFolded, act });
  handlers.current = { activate, saveNote, cancel: closeEditor, comment, ask, rowClick, pick, answer, toggleFolded, act };
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
  const onPick = useCallback((p: DiffPick) => handlers.current.pick(p), []);

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
        const el = rowEl(key);
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

  const scopeLine = diff && (
    <ReviewScope
      since={sinceShown ? { name: sinceName, title: diff.since ? `What's changed since ${sinceName}, at ${diff.since.slice(0, 7)}` : `Each file's change, compared with what you reviewed (${sinceName})` } : undefined}
      commits={diff.commits.length}
      base={diff.since ? undefined : diff.baseRef}
      additions={totals.a}
      deletions={totals.d}
    />
  );
  const actions = (
    <>
      {changes && diff && sinceable !== undefined && (
        <Button variant="ghost" size="small" onClick={toggleSince} title="Switch between what's changed since your last review and all of it" keys="i">
          {sinceShown ? "Show all changes" : `Only since ${sinceName}`} <Keys k="i" />
        </Button>
      )}
      {source.actions}
      {source.browse && (
        <Link to={source.browse} className={buttonClass({ size: "small" }, css.desktopOnly)} title="Browse its files, as they are now">
          <Icon name="file" size={12} /> Browse files
        </Link>
      )}
      <Button variant="ghost" size="small" className={css.mobileOnly} onClick={() => setFinder(true)} title="Jump to a file" keys="t">
        <Icon name="file" size={13} /> Files
      </Button>
      {changes && (
        <Segmented className={css.desktopOnly} value={split ? "split" : "unified"} onChange={toggleSplit} aria-label="Diff layout">
          <Segment value="unified" title="Unified" keys="u">
            <Icon name="unified" size={13} /> Unified
          </Segment>
          <Segment value="split" title="Side by side" keys="u">
            <Icon name="split" size={13} /> Split
          </Segment>
        </Segmented>
      )}
      <Button variant="primary" className={css.submit} onClick={submit} unavailable={source.readOnly} title="Submit the review" keys="s">
        {draft ? `Submit review · ${draft.comments.length}` : "Submit review"} <Keys k="s" />
      </Button>
    </>
  );

  return (
    <ReviewLayout
      width={filesWidth}
      side={
        <>
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
        </>
      }
      pane={pane}
      onPaneScroll={onPaneScroll}
      onResize={resizeFiles}
      onResetWidth={resetFilesWidth}
      foot={source.foot}
      over={
        <>
          {codeNav.card}
          {codeNav.picker}
          {codeNav.panel}
          {finder && (
            <ReviewFileJump
              paths={files.map((f) => f.path)}
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
        </>
      }
    >
      <ReviewHead back={source.back} name={source.name} scope={scopeLine} actions={actions} />
      {entries.length > 0 && <ReviewTabs views={[...entries, { key: "changes", ...viewNames.changes, n: viewKeys.length }]} here={view} onPick={(k) => show(k as ViewKey)} />}
      {submitted && source.submitted(submitted.review, submitted.state)}
      {source.notices}
      {changes && scopeNote && (
        <Callout tone="quiet" icon="info">
          Showing all the changes: {scopeNote}
        </Callout>
      )}
      {changes && sinceShown && (
        <ReviewStatus>
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
        </ReviewStatus>
      )}
      {moved && <Callout tone="attention" title={source.movedWords} actions={<Button onClick={reload} keys="r">Reload <Keys k="r" /></Button>} />}
      {error && <Banner>{humanError(error)}</Banner>}
      {parts.map((p) => (
        <ReviewViewBody key={p.key} label={viewNames[p.key].label} hidden={view !== p.key} read>
          {typeof p.body === "function" ? p.body({ toFinding, toComment }) : p.body}
        </ReviewViewBody>
      ))}
      <ReviewViewBody label="Changes" hidden={!changes}>
        {diff && diff.uncommitted.length > 0 && (
          <Callout tone="attention" icon="alert" title={`${diff.uncommitted.length} ${diff.uncommitted.length === 1 ? "file has" : "files have"} uncommitted changes, not in this review`}>
            <Text mono>{diff.uncommitted.slice(0, 5).join(", ")}</Text>
            {diff.uncommitted.length > 5 && " …"}
          </Callout>
        )}
        {!diff && !error && <Loading what="Reading the changes…" />}
        {diff && diff.commits.length > 0 && <ReviewCommits commits={diff.commits} />}
        {rs.reviews.length > 0 && <ReviewEarlier reviews={rs.reviews} />}
        {diff && !files.length && <Text as="p" dim>{source.empty}</Text>}
        {files.map((f, i) => (
          <Fragment key={f.path}>
            {guideHeads?.has(i) && <ReviewGuideGroup label={guideHeads.get(i)!.label} files={guideHeads.get(i)!.files.length} skim={guideHeads.get(i)!.skim} why={guideHeads.get(i)!.why} />}
            <FileSection
              index={i}
              file={f}
              rows={rows[i]}
              state={states[i]}
              split={split}
              cursor={row?.file === i ? cursor : ""}
              selected={selection?.file === i ? selection.keys : undefined}
              selectSide={selection?.file === i ? selectSide : undefined}
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
              onPick={onPick}
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
          <ReviewOrphans label={`Comments on files ${sinceShown ? `not changed since ${sinceName}` : "not in this diff"}`}>
            {orphans.map(
              (r) =>
                r.type === "comment" && (
                  <div key={r.key}>
                    <ReviewOrphanPath path={r.placed.note.path} />
                    <CommentCard row={r} cursor={cursor === r.key} replying={replying === r.placed.note.id} onRow={setCursor} onAnswer={onAnswer} onReplyCancel={onReplyCancel} />
                  </div>
                ),
            )}
          </ReviewOrphans>
        )}
      </ReviewViewBody>
    </ReviewLayout>
  );
}
