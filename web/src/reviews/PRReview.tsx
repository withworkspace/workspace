import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link, useNavigate, useParams, useSearchParams } from "react-router";
import {
  askPR,
  type Discussion,
  dockPR,
  getPR,
  getPRDiff,
  getPRDiscussion,
  getPRGuide,
  getPRPasses,
  getPRThreads,
  getReview,
  noteKinds,
  type Posting,
  type PR,
  type PRDetail,
  prAPI,
  previewPRReview,
  respondPRThread,
  type Review as ReviewData,
  type ReviewGuide,
  type ReviewPass,
  type ReviewState,
  submitPRReview,
  useEvents,
  type Verdict,
  type Workspace,
} from "../api";
import { type AskAbout, linesChip } from "../ask/ask";
import { AskBar, type AskBarHandle } from "../ask/AskBar";
import { useKeys } from "../keys";
import { type AskedFinding, type ReviewDiff, type ReviewSource, ReviewView, type Selection } from "../review/Review";
import { Icon } from "../ui/Icon";
import { Callout, Dialog, Empty, humanError, Keys, Loading, toast, useTitle } from "../ui/kit";
import { Status } from "../ui/status";
import { Time } from "../ui/time";
import { GuideMark, GuideSection } from "./Guide";
import { PRDescription, PRDiscussion, StackDescriptions } from "./Overview";
import { PassesSection, PassMarks, standingWords } from "./Passes";
import { useReviewPasses } from "./usePasses";
import { layersLeft } from "./after";
import { Checks, prHref, prStatus, reviewed } from "./Reviews";
import { useLatest } from "../load";
import { PagePin } from "../shell/focus";
import { Chip, chipClass } from "../ui/Chip";
import { Button, buttonClass } from "../ui/Button";
import { clip } from "../ui/text";

// A pull request's review: the page a capsule's review is, on commits
// fetched into the repo's clone. A stack opens as one review, its layers
// down the side, bottom first; each layer shows only its own changes.
// Submitting posts to GitHub, as you.
//
// At its foot is the ask bar (ask/AskBar.tsx), as a document's reader has:
// ask about the pull request, or lines picked out of its diff (a), or what
// a pass found. ws docks it first, so the agent reads the code as files.
// The answer is a card over the review, and a review comment it proposes
// is added to your draft from there.

export const prReviewKeys: [string, string][] = [
  ["[ / ]", "The layer below / above, in a stack"],
  ["w", "The whole stack together, or back to the layer"],
  ["/", "Go to the ask bar"],
  ["a", "Ask about the line (or the selected lines)"],
  ["A", "Ask about the pull request"],
  ["o", "Open it on GitHub"],
  ["O", "Open here: dock it, as ws dock does"],
];

const verdictEvents: Record<Verdict, string> = { approve: "Approve", changes: "Request changes", comment: "Comment" };

// Shown is the pull request on screen (a layer, or its whole stack), and
// what it opened with, read before it was shown.
interface Shown {
  repo: string;
  number: string;
  whole: boolean;
  opened?: ReviewSource["opened"];
  passes?: ReviewPass[];
  guide?: ReviewGuide;
}

export function PRReviewPage({ workspace }: { workspace: Workspace }) {
  // The address says what's asked for; what's shown follows it once it's
  // been read. Moving to another layer, or to the whole stack and back,
  // keeps this one on screen until the next can be shown whole, then
  // shows it at once: the page never empties between them.
  const { repo: askedRepo = "", number: askedNumber = "" } = useParams();
  const [params, setParams] = useSearchParams();
  const navigate = useNavigate();
  const askedWhole = params.get("whole") === "1";
  const [shown, setShown] = useState<Shown>();
  const shownNow = useRef(shown);
  shownNow.current = shown;
  const repo = shown?.repo ?? askedRepo;
  const number = shown?.number ?? askedNumber;
  const whole = shown?.whole ?? askedWhole;
  const api = prAPI(repo, number);
  const ref = `${repo}#${number}`;
  const [d, setD] = useState<PRDetail>();
  const [error, setError] = useState<string>();
  const [threads, setThreads] = useState<ReviewData>();
  const [threadsError, setThreadsError] = useState<string>();
  const [discussion, setDiscussion] = useState<Discussion>();
  const [discussionError, setDiscussionError] = useState<string>();
  const asker = useRef<AskBarHandle<PRHeld>>(null);
  // Its passes and its guide, as a capsule's review has them too.
  const { passes, guide, guided, hidden, took, order, notes, findings, followups, loadPasses, loadGuide, putPass, setGuide, setGuidedKept, hide, onFinding, bumpTook } = useReviewPasses(api, shown);
  const [docking, setDocking] = useState(false);
  useTitle(d ? `${d.pr.title} · ${ref}` : ref);

  // Opening what's asked for: what it opens on (what it says it does, its
  // changes, your review) is read, and shown together. What's been said
  // on GitHub, on its lines and on itself, can be slow: it's shown with
  // them if it's in by then, or once it is.
  const beginOpen = useLatest();
  useEffect(() => {
    const fresh = beginOpen();
    const a = prAPI(askedRepo, askedNumber);
    const reviewsRead = getReview(a);
    const passesRead = getPRPasses(a).catch(() => [] as ReviewPass[]);
    const guideRead = getPRGuide(a).catch(() => ({}) as ReviewGuide);
    let said: { x?: Discussion; e?: string } | undefined;
    let lines: { t?: ReviewData; e?: string } | undefined;
    let open = false;
    const say = (s: { x?: Discussion; e?: string }) => {
      if (!fresh()) return;
      setDiscussion(s.x);
      setDiscussionError(s.e);
    };
    const thread = (s: { t?: ReviewData; e?: string }) => {
      if (!fresh()) return;
      setThreads(s.t);
      setThreadsError(s.e);
    };
    getPRDiscussion(a)
      .then(
        (x) => ({ x }),
        (e: Error) => ({ e: e.message }),
      )
      .then((s) => {
        said = s;
        if (open) say(s);
      });
    getPRThreads(a)
      .then(
        (t) => ({ t }),
        (e: Error) => ({ e: e.message }),
      )
      .then((s) => {
        lines = s;
        if (open) thread(s);
      });
    getPR(askedRepo, askedNumber).then(
      async (x) => {
        // What the review opens on. If it can't be read, the review
        // reads it itself, and says why.
        const opened = await Promise.all([reviewsRead, getPRDiff(a, askedWhole ? "whole" : x.since ? "since" : "layer")]).then(
          ([rs, diff]) => ({ rs, diff: diff as ReviewDiff }),
          () => undefined,
        );
        const [p, g] = await Promise.all([passesRead, guideRead]);
        if (!fresh()) return;
        open = true;
        setD(x);
        setError(undefined);
        thread(lines ?? {});
        say(said ?? {});
        setShown({ repo: askedRepo, number: askedNumber, whole: askedWhole, opened, passes: p, guide: g });
      },
      (e: Error) => fresh() && setError(e.message),
    );
  }, [askedRepo, askedNumber, askedWhole, beginOpen]);

  // Reading what's shown again (a push, a review, a reply): kept only
  // if it's still what's shown when it comes.
  const still = (s?: Shown) => !!s && shownNow.current?.repo === s.repo && shownNow.current?.number === s.number;
  const beginPR = useLatest();
  const load = useCallback(() => {
    const s = shownNow.current;
    if (!s) return Promise.resolve();
    const fresh = beginPR();
    return getPR(s.repo, s.number).then(
      (x) => {
        if (!fresh() || !still(s)) return;
        setD(x);
        setError(undefined);
      },
      (e: Error) => fresh() && still(s) && setError(e.message),
    );
    // eslint-disable-next-line react-hooks/exhaustive-deps -- still reads a ref
  }, [beginPR]);
  const beginThreads = useLatest();
  const loadThreads = useCallback(() => {
    const s = shownNow.current;
    if (!s) return Promise.resolve();
    const fresh = beginThreads();
    return getPRThreads(prAPI(s.repo, s.number)).then(
      (t) => {
        if (!fresh() || !still(s)) return;
        setThreads(t);
        setThreadsError(undefined);
      },
      (e: Error) => fresh() && still(s) && setThreadsError(e.message),
    );
    // eslint-disable-next-line react-hooks/exhaustive-deps -- still reads a ref
  }, [beginThreads]);
  const loadDiscussion = useCallback(() => {
    const s = shownNow.current;
    if (!s) return Promise.resolve();
    return getPRDiscussion(prAPI(s.repo, s.number)).then(
      (x) => {
        if (!still(s)) return;
        setDiscussion(x);
        setDiscussionError(undefined);
      },
      (e: Error) => still(s) && setDiscussionError(e.message),
    );
    // eslint-disable-next-line react-hooks/exhaustive-deps -- still reads a ref
  }, []);
  // When the list changes (a push, a review), or ws has read this again
  // behind and found it changed (a reply, a push), what's shown is read
  // again: as ws keeps it, at once.
  useEvents(`pr:${ref}`, {
    changed: () => {
      load();
      loadThreads();
      loadDiscussion();
      loadPasses();
      loadGuide();
    },
  });

  const layers = d?.stack?.layers ?? [];
  // [ and ] move from the layer asked for, so pressing ] twice goes two
  // up, even before the first has been shown.
  const at = layers.findIndex((l) => l.number === Number(askedNumber));
  // Moving between layers, or to the whole stack and back, keeps the view
  // you're on (the changes, say), where the other has it.
  const viewQ = params.get("view") ? `?${new URLSearchParams({ view: params.get("view")! })}` : "";
  const go = (l?: PR) => l && navigate(prHref(l.repo, l.number) + viewQ, { replace: true });
  // What's asked for and not yet shown, for the list of layers to say.
  const going = shown && (shown.repo !== askedRepo || shown.number !== askedNumber || shown.whole !== askedWhole) ? { number: Number(askedNumber), whole: askedWhole } : undefined;
  const toggleWhole = () =>
    d?.stack &&
    setParams(
      (p) => {
        const next = new URLSearchParams(p);
        if (next.get("whole") === "1") next.delete("whole");
        else next.set("whole", "1");
        return next;
      },
      { replace: true },
    );
  const dock = () => {
    if (!d || docking) return;
    if (d.pr.fork) return toast("It's from a fork: ws dock looks for its branch on origin, so it can't be opened here.");
    setDocking(true);
    dockPR(api)
      .then(
        (r) => {
          toast(r.message, { tone: "done" });
          return load();
        },
        (e: Error) => toast(humanError(e.message), { tone: "danger" }),
      )
      .finally(() => setDocking(false));
  };
  // ask goes to the ask bar: with lines in hand, or what a pass found, to
  // ask about them; with neither, about the whole of it.
  const ask = (lines?: Selection, finding?: AskedFinding) => {
    if (!d) return;
    if (d.noAsk) return toast(d.noAsk);
    if (!lines && !finding) asker.current?.focus(true);
    else asker.current?.ask({ lines, finding });
  };
  // What asking about it is: ws docks it first, and the agent answers with
  // the code checked out here. Docked, the page says where.
  const askAbout: AskAbout<PRHeld> = {
    title: "this pull request",
    start: (body, h) =>
      askPR(api, { body: h?.finding ? withFinding(body, h.finding) : body, ...(h?.lines ?? {}) }).then((t) => {
        load();
        return t;
      }),
    chip: (h) => ({
      ...(h.finding
        ? { icon: "list-checks" as const, text: `${h.finding.pass}: “${clip(h.finding.text, 28)}”`, title: `What the ${h.finding.pass} pass found (${h.finding.kind}): ${h.finding.text}`, kind: "finding" as const }
        : linesChip(h.lines!.path, h.lines!.line, h.lines!.endLine, h.lines!.side)),
      placeholder: h.finding ? "Ask about what it found…" : "Ask about these lines…",
      drop: "Ask about the whole pull request",
    }),
  };

  // The conversations about it, as the workspace has them now: answering,
  // or your turn.
  const talks = useMemo(() => workspace.conversations.filter((t) => t.pr === ref), [workspace.conversations, ref]);

  if (error) {
    return (
      <div className="page narrow">
        <Empty
          icon="pr"
          title={`${ref} can't be shown`}
          action={
            <Link to="/reviews" className={buttonClass()}>
              Back to Reviews
            </Link>
          }
        >
          {humanError(error)}
        </Empty>
      </div>
    );
  }
  if (!d || !shown) {
    return (
      <div className="page narrow">
        <Loading what={`Fetching ${ref}…`} delay={150} />
      </div>
    );
  }

  const pr = d.pr;
  // What others have said on it: what's new in the discussion is counted
  // from these, not from what you say yourself.
  const theirs = discussion?.items.filter((it) => !it.mine).length;
  const sinceName = d.reviewed?.id === "github" || !d.reviewed ? "your review on GitHub" : `your review ${d.reviewed.id}`;
  const source: ReviewSource = {
    api,
    title: `${pr.title} · ${ref}`,
    name: (
      <span className="pr-name">
        {whole && d.stack ? (
          <>
            {d.stack.title}
            <Chip tone="quiet" className="pr-name-chip" title="Every layer's changes together: where the stack ends up">
              <Icon name="layers" size={11} /> the whole stack · {layers.length} layers
            </Chip>
          </>
        ) : (
          <>
            {pr.title} <span className="mono dim">{ref}</span>
            {d.stack && d.layer ? (
              <Chip tone="quiet" className="pr-name-chip" title="Where this layer sits in its stack: 1 is the bottom">
                <Icon name="layers" size={11} /> {d.layer} of {layers.length}
              </Chip>
            ) : null}
          </>
        )}
      </span>
    ),
    back: { to: "/reviews", label: "Reviews" },
    headNow: whole ? undefined : d.head,
    updated: `${d.head}:${d.reviewed?.id ?? ""}:${took}`,
    // Looking again: what's changed since your last review, if anything has.
    sinceOf: () => (!whole && d.since ? `${d.reviewed?.id}:${d.since}` : undefined),
    getDiff: (since) => getPRDiff(api, whole ? "whole" : since ? "since" : "layer") as Promise<ReviewDiff>,
    diffKey: whole ? "whole" : "layer",
    sinceName: () => sinceName,
    movedWords: "It has new commits since this loaded",
    empty: "It changes nothing.",
    opened: shown.opened,
    readOnly: whole ? "The whole stack is for reading: comment on a layer ([ and ] move between them)." : undefined,
    side: d.stack ? <LayerList d={d} whole={whole} going={going} search={viewQ} onWhole={toggleWhole} /> : undefined,
    actions: (
      <>
        <PagePin r={{ kind: "pr", repo: pr.repo, number: pr.number }} />
        <Button
          variant="ghost" size="small"
          className="desktop-only"
          onClick={dock}
          disabled={docking || pr.fork}
          title={pr.fork ? "It's from a fork: ws dock looks for its branch on origin" : d.docked?.by === "you" ? `It's here: ${pr.repo}/${d.docked.capsule}` : "Dock it here, as ws dock does, to run it yourself (⇧O)"}
        >
          <Icon name="box" size={13} /> {docking ? "Docking…" : d.docked?.by === "you" ? d.docked.capsule : "Open here"}
        </Button>
        <a className={buttonClass({ size: "small" })} href={pr.url} target="_blank" rel="noreferrer" title="Open it on GitHub (o)">
          <Icon name="external" size={12} /> GitHub
        </a>
      </>
    ),
    notices: (
      <>
        <div className="pr-meta">
          {!whole && <Status s={prStatus(pr)} />}
          <span>
            {pr.mine ? "Yours" : pr.author} · <span className="mono">{pr.branch}</span> <Icon name="arrow-right" size={11} />{" "}
            <span className="mono">{whole && d.stack ? d.stack.base : pr.base}</span>
          </span>
          {pr.checks && !whole && (
            <span className="pr-meta-checks">
              <Checks checks={pr.checks} /> {pr.checks === "success" ? "Checks pass" : pr.checks === "failure" ? "Checks are failing" : "Checks are running"}
            </span>
          )}
          {!whole &&
            discussion?.reviewers.map((r) => (
              <span
                key={r.login}
                className={`reviewer k-${r.kind}${r.stale ? " stale" : ""}`}
                title={`${r.login} ${r.kind === "approved" ? "approved" : r.kind === "changes" ? "asked for changes" : "had a review dismissed"}${r.stale ? ", on earlier commits" : ""}`}
              >
                <Icon name={r.kind === "approved" ? "check-circle" : r.kind === "changes" ? "alert" : "message"} size={12} /> {r.login}
              </span>
            ))}
          {pr.updated && <Time iso={pr.updated} as="relative" className="dim" />}
          {!whole && !threads && !threadsError && (
            <span className="dim" title="The comments on its lines, and what's been said on it, are read after what it opens on: they show when they're in.">
              <Icon name="spinner" className="spin" size={12} /> Reading the comments on GitHub…
            </span>
          )}
          {pr.mission && (
            <Link to={`/missions/${pr.mission.slug}`} className={chipClass("quiet", "pr-meta-chip")}>
              <Icon name="flag" size={11} /> {pr.mission.title}
            </Link>
          )}
          {d.docked && (
            <Link to={`/files/${pr.repo}/${d.docked.capsule}`} className={chipClass("quiet", "pr-meta-chip")} title={d.docked.by === "ws" ? "ws docked it for a question: it's removed once this merges or closes, unless you change something in it" : "It's checked out here: browse its files"}>
              <Icon name="box" size={11} /> {pr.repo}/{d.docked.capsule}
            </Link>
          )}
          {talks.map((t) => (
            <Link key={t.id} to={`/conversations/${t.id}`} className={chipClass(t.waiting && t.status === "open" ? "attention" : "quiet", "pr-meta-chip")} title={t.title || t.opening}>
              <Icon name={t.answering ? "spinner" : "message"} size={11} className={t.answering ? "spin" : undefined} />
              {t.answering ? "Answering" : t.waiting && t.status === "open" ? "Answered" : t.status === "resolved" ? "Resolved" : "Asked"}: {clip(t.title || t.opening, 48)}
            </Link>
          ))}
        </div>
        {d.needsRebase && (
          <Callout tone="attention" title="This layer needs a rebase">
            The layer below has moved on, and this one hasn't followed. Its changes show against the layer below as it was when this was pushed.
          </Callout>
        )}
        {pr.fork && (
          <Callout tone="quiet" icon="info">
            It's from a fork: it can be reviewed here, but not docked, so an agent can't be asked about it.
          </Callout>
        )}
        {threadsError && (
          <Callout tone="quiet" icon="info">
            The comments on GitHub couldn't be read ({humanError(threadsError)}): what shows is your own, as kept here.
          </Callout>
        )}
      </>
    ),
    // The views beside the changes: its description, its guide, what's
    // been said on GitHub and its passes. A whole stack's is each layer's
    // description, for reading.
    views:
      whole && d.stack
        ? [{ key: "overview", body: <StackDescriptions layers={layers} /> }]
        : [
            { key: "overview", body: <PRDescription body={d.body} /> },
            { key: "guide", mark: <GuideMark guide={guide} />, body: <GuideSection api={api} guide={guide} guided={guided} onGuide={setGuide} onGuided={setGuidedKept} /> },
            {
              key: "discussion",
              count: discussion?.items.length,
              // What's new is counted from what others said: your own
              // comment, posted from here, isn't news to you.
              seen: theirs === undefined ? undefined : `${theirs}`,
              news: (seen) => {
                const n = theirs === undefined ? 0 : theirs - Number(seen);
                return n > 0 ? `${n} new on GitHub` : undefined;
              },
              body: <PRDiscussion api={api} discussion={discussion} error={discussionError} onSaid={setDiscussion} />,
            },
            {
              key: "passes",
              mark: <PassMarks passes={passes} />,
              body: (go) => (
                <PassesSection
                  api={api}
                  passes={passes}
                  hidden={hidden}
                  canAsk={!d.noAsk}
                  toFinding={go.toFinding}
                  toComment={go.toComment}
                  onResolve={(thread) =>
                    respondPRThread(api, thread, "resolve").then((t) => {
                      setThreads(t);
                      return loadPasses();
                    })
                  }
                  onPass={putPass}
                  onTook={bumpTook}
                  onHide={hide}
                  onAsk={(p, f) =>
                    ask(f.path && f.line ? { path: f.path, side: f.side === "old" ? "old" : "new", line: f.line, endLine: f.endLine ?? f.line, commit: f.commit ?? d.head } : undefined, {
                      pass: p.title,
                      kind: f.kind,
                      text: f.text,
                    })
                  }
                />
              ),
            },
          ],
    // What passes found that's still open, on the diff's lines; taking one
    // puts it in the draft.
    // The guide's order, for the list of files to follow, if you choose.
    order: whole ? undefined : order,
    onOrder: whole || !guide?.guide ? undefined : setGuidedKept,
    guideNotes: whole ? undefined : notes,
    findings: whole ? undefined : findings,
    // What a follow-up said of your comments is said on them.
    followups: whole ? undefined : followups,
    onFinding,
    threads: whole ? { id: "github", capsule: ref, baseRef: "", base: "", head: "", state: "submitted", created: "", comments: [] } : threads,
    // Resolving a thread settles what the follow-up said of it.
    onThread: (id, what, body) =>
      respondPRThread(api, id, what, body).then((t) => {
        setThreads(t);
        if (what !== "reply") loadPasses();
      }),
    submit: ({ diff, onClose, onDiscard, done }) => (
      <PRSubmitDialog
        d={d}
        passes={passes}
        head={diff.head}
        onClose={onClose}
        onDiscard={onDiscard}
        onPosted={(posted, mine) => {
          // Nothing here still needs you: back to the list, at the next
          // one. A stack keeps you until its last layer is reviewed.
          if (!layersLeft(layers, posted.map((p) => p.number)).length) {
            const url = posted.length === 1 ? posted[0].url : undefined;
            toast(posted.length === 1 ? `Your review of ${pr.repo}#${posted[0].number} is on GitHub.` : `Your ${posted.length} reviews are on GitHub.`, {
              tone: "done",
              action: url ? { label: "See it on GitHub", run: () => window.open(url, "_blank", "noopener") } : undefined,
            });
            reviewed(pr.repo, pr.number);
            navigate("/reviews");
            return;
          }
          if (mine) done(mine);
          else {
            toast("Posted to GitHub.", { tone: "done" });
            onClose();
          }
          load();
          loadThreads();
          loadDiscussion();
        }}
      />
    ),
    submitted: (r, state) => (
      <Callout
        tone="done"
        title={`Posted to GitHub: ${verdictEvents[r.verdict!].toLowerCase()}`}
        actions={
          state.url ? (
            <a className={buttonClass()} href={state.url} target="_blank" rel="noreferrer">
              See it on GitHub <Icon name="external" size={12} />
            </a>
          ) : undefined
        }
      >
        Your review of {ref} is on GitHub. When they push, it's back with you, and opens on what's changed since.
      </Callout>
    ),
    onAsk: ask,
    // The ask bar, unless it can't be asked about (a fork, or no agent).
    foot: d.noAsk ? undefined : (
      <AskBar
        key={api}
        ref={asker}
        about={askAbout}
        suggestions={suggest}
        root={workspace.root}
        // A comment added to the draft is on its lines, read again.
        onSettled={(p) => p.kind === "comment" && p.state === "accepted" && bumpTook()}
      />
    ),
    dismiss: () => asker.current?.close() ?? false,
    keys: {
      "[": () => go(layers[at - 1]),
      "]": () => go(layers[at + 1]),
      w: toggleWhole,
      "/": () => (d.noAsk ? toast(d.noAsk) : asker.current?.focus()),
      A: () => ask(),
      o: () => window.open(pr.url, "_blank", "noopener"),
      O: dock,
    },
  };
  return <ReviewView key={`${api}:${whole}`} source={source} />;
}

// PRHeld is what's in the ask bar's hand on a pull request: lines picked
// out of its diff, or what a pass found (and the lines it's on, if it's
// on some).
interface PRHeld {
  lines?: Selection;
  finding?: AskedFinding;
}

// suggest is what the ask bar offers to ask, about what's in hand, or the
// pull request.
const suggest = (h?: PRHeld) =>
  h?.finding
    ? ["Is it right?", "How would it be fixed?", "Propose a comment on it"]
    : h?.lines
      ? ["Explain these lines", "What calls this?", "What could go wrong here?"]
      : ["Summarise it", "What could break?", "Where do I start reading?", "Is it tested?"];

// withFinding is a question about what a pass found, with what it found
// said after it: the card leads with the question.
const withFinding = (q: string, f: AskedFinding) => `${q}\n\nIt's about what the ${f.pass} pass found (${f.kind}): "${f.text}"`;

// LayerList is a stack's layers down the side, bottom first: the order to
// read them in.
// going is the layer (or the whole stack) asked for and being read, to
// show it's coming.
function LayerList({ d, whole, going, search, onWhole }: { d: PRDetail; whole: boolean; going?: { number: number; whole: boolean }; search: string; onWhole: () => void }) {
  const st = d.stack!;
  return (
    <div className="layer-list">
      <div className="files-head">
        <span title={st.workedOut ? "ws worked it out: each of these pull requests is based on the last one's branch" : `Stack ${st.number} on GitHub`}>
          Stack <span className="dim">· {st.layers.length} {st.layers.length === 1 ? "layer" : "layers"}{st.workedOut ? " · worked out" : ""}</span>
        </span>
        <span className="dim layer-keys">
          <Keys k="[" /> <Keys k="]" />
        </span>
      </div>
      <ol>
        {st.merged ? <li className="merged dim">{st.merged} merged below</li> : null}
        {st.layers.map((l, i) => {
          const s = prStatus(l);
          const coming = going && !going.whole && going.number === l.number;
          return (
            <li key={l.number} className={`${!whole && l.number === d.pr.number ? "here" : ""}${coming ? " going" : ""}`} aria-busy={coming || undefined}>
              <Link to={prHref(l.repo, l.number) + search} replace title={`${l.title} (#${l.number}): ${l.words}`}>
                <span className="layer-n">{i + 1}</span>
                {coming ? <Icon name="spinner" size={13} className="spin" /> : <Status s={s} iconOnly size={13} />}
                <span className="layer-title">{l.title}</span>
                <span className="mono dim">#{l.number}</span>
              </Link>
            </li>
          );
        })}
      </ol>
      <Button variant="ghost" size="small" className={`whole-stack ${whole ? " on" : ""}`} aria-pressed={whole} aria-busy={going?.whole || undefined} onClick={onWhole} title="Every layer's changes together, to see where the stack ends up. It's for reading: comments are made on a layer. (w)">
        <Icon name={going?.whole && !whole ? "spinner" : "layers"} size={12} className={going?.whole && !whole ? "spin" : undefined} /> Whole stack <Keys k="w" />
      </Button>
    </div>
  );
}

// Posted is a review that's gone to GitHub: the pull request it's of, and
// where it is there.
interface Posted {
  number: number;
  url?: string;
}

interface LayerDraft {
  pr: PR;
  draft: ReviewData | null;
  verdict?: Verdict;
  summary: string;
  posting?: Posting;
  error?: string;
  state?: "posting" | "posted";
}

// PRSubmitDialog ends the review by posting it to GitHub, as you: the one
// thing here that leaves your machine. It shows what will be posted, and
// where, before it is. GitHub reviews one pull request at a time, so a
// stack's lists its layers, each with its comments and its own verdict; a
// layer with no verdict is left alone.
function PRSubmitDialog({
  d,
  passes,
  head,
  onClose,
  onDiscard,
  onPosted,
}: {
  d: PRDetail;
  passes?: ReviewPass[]; // this pull request's, as they stand
  head: string;
  onClose: () => void;
  onDiscard: () => void;
  // Every review chosen is on GitHub: the layers posted while this was open,
  // and this pull request's reviews as they then are, if it was one of them.
  onPosted: (posted: Posted[], mine?: ReviewState) => void;
}) {
  const prs = d.stack?.layers ?? [d.pr];
  const [layers, setLayers] = useState<LayerDraft[]>(() => prs.map((pr) => ({ pr, draft: null, summary: "" })));
  const [focus, setFocus] = useState(() => Math.max(0, prs.findIndex((p) => p.number === d.pr.number)));
  const [busy, setBusy] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const area = useRef<HTMLTextAreaElement>(null);
  // What's been posted while this is open, a failed try before it included.
  const posted = useRef<Posted[]>([]);
  const mine = useRef<ReviewState | undefined>(undefined);
  const set = (i: number, patch: Partial<LayerDraft>) => setLayers((ls) => ls.map((l, j) => (j === i ? { ...l, ...patch } : l)));
  // The commit a layer's review is of: the one this page shows, for the
  // layer it's on; for another, the one its draft was written on.
  const headOf = (l: LayerDraft) => (l.pr.number === d.pr.number ? head : (l.draft?.head ?? ""));

  // Each layer's draft, as it's kept.
  useEffect(() => {
    let stopped = false;
    Promise.all(prs.map((pr) => getReview(prAPI(pr.repo, pr.number)).then((s) => s.draft, () => null))).then((drafts) => {
      if (stopped) return;
      setLayers((ls) => ls.map((l, i) => ({ ...l, draft: drafts[i] })));
      setLoaded(true);
    });
    return () => {
      stopped = true;
    };
  }, []);

  // What a layer's verdict and summary would post, asked for as they change.
  const want = layers.map((l) => `${l.verdict ?? ""}\u0000${l.summary}\u0000${l.draft?.comments.length ?? 0}`).join("\u0001");
  // (An answer that comes once they've changed again is dropped.)
  useEffect(() => {
    if (!loaded) return;
    let stale = false;
    const t = setTimeout(() => {
      layers.forEach((l, i) => {
        if (!l.verdict || l.state) return;
        previewPRReview(prAPI(l.pr.repo, l.pr.number), headOf(l), l.verdict, l.summary).then(
          (posting) => !stale && set(i, { posting, error: undefined }),
          (e: Error) => !stale && set(i, { posting: undefined, error: e.message }),
        );
      });
    }, 250);
    return () => {
      stale = true;
      clearTimeout(t);
    };
  }, [want, loaded]);

  const pick = (i: number, v?: Verdict) => {
    if (layers[i].state) return;
    set(i, { verdict: layers[i].verdict === v ? undefined : v, posting: undefined, error: undefined });
    setFocus(i);
    setTimeout(() => area.current?.focus());
  };
  const approveAll = () => {
    setLayers((ls) => ls.map((l) => (l.state || l.pr.mine || l.pr.draft ? l : { ...l, verdict: "approve" as Verdict, posting: undefined, error: undefined })));
  };
  const chosen = layers.filter((l) => l.verdict && !l.state);
  const ready = chosen.length > 0 && chosen.every((l) => l.posting && !l.error);

  // Posted bottom first, one pull request at a time: a layer that fails
  // stops there, and those before it are on GitHub.
  const submit = async () => {
    if (!ready || busy) return;
    setBusy(true);
    let failed = false;
    for (let i = 0; i < layers.length; i++) {
      const l = layers[i];
      if (!l.verdict || l.state === "posted") continue;
      set(i, { state: "posting", error: undefined });
      try {
        const next = await submitPRReview(prAPI(l.pr.repo, l.pr.number), headOf(l), l.verdict, l.summary);
        set(i, { state: "posted" });
        posted.current.push({ number: l.pr.number, url: next.url });
        if (l.pr.number === d.pr.number) mine.current = next;
      } catch (e) {
        set(i, { state: undefined, error: (e as Error).message });
        failed = true;
        break;
      }
    }
    setBusy(false);
    if (!failed) onPosted(posted.current, mine.current);
  };

  useKeys(
    {
      a: () => !layers[focus].pr.mine && pick(focus, "approve"),
      c: () => !layers[focus].pr.mine && pick(focus, "changes"),
      m: () => pick(focus, "comment"),
      A: approveAll,
      "[": () => setFocus((f) => Math.max(0, f - 1)),
      "]": () => setFocus((f) => Math.min(layers.length - 1, f + 1)),
      "mod+Enter": submit,
    },
    { modal: true },
  );

  // Passes with a finding that blocks still open: approving over one is
  // said, not stopped.
  const noGo = (passes ?? []).filter((p) => p.standing.status === "nogo");
  const stack = layers.length > 1;
  const here = layers[Math.max(0, prs.findIndex((p) => p.number === d.pr.number))];
  return (
    <Dialog title={stack ? "Submit the review of the stack" : "Submit the review"} onClose={onClose} className="submit pr-submit" wide>
      <p>
        Submitting posts to GitHub, as you. It's the one thing here that leaves your machine, and a review can't be unsent, only dismissed: what's below is exactly what goes.
        {stack && " GitHub reviews one pull request at a time, so each layer has its own verdict; a layer with none is left alone."}
      </p>
      {stack && (
        <div className="pr-submit-all">
          <Button onClick={approveAll} disabled={busy} title="Give every layer that isn't a draft or yours an approval (⇧A). Nothing's posted until you submit.">
            <Icon name="check-circle" size={13} /> Approve every layer <Keys k="A" />
          </Button>
          <span className="dim">
            <Keys k="[" /> <Keys k="]" /> pick a layer; <Keys k="a" /> <Keys k="c" /> <Keys k="m" /> its verdict
          </span>
        </div>
      )}
      <div className="pr-submit-layers">
        {layers.map((l, i) => {
          const counts = noteKinds.flatMap((k) => {
            const n = l.draft?.comments.filter((c) => c.kind === k).length ?? 0;
            return n ? [`${n} ${k}`] : [];
          });
          const total = l.draft?.comments.length ?? 0;
          return (
            <section key={l.pr.number} className={`pr-submit-layer${i === focus ? " focus" : ""}${l.state === "posted" ? " posted" : ""}`} onClick={() => setFocus(i)}>
              <header>
                {stack && <span className="layer-n">{i + 1}</span>}
                <span className="layer-title">{l.pr.title}</span>
                <span className="mono dim">
                  {l.pr.repo}#{l.pr.number}
                </span>
                <span className="dim">
                  {total ? `${total} ${total === 1 ? "comment" : "comments"}${counts.length ? ` (${counts.join(", ")})` : ""}` : "no comments"}
                </span>
                <span className="spacer-left" />
                {l.state === "posted" && (
                  <span className="settled-line done">
                    <Icon name="check-circle" size={13} /> Posted
                  </span>
                )}
                {l.state === "posting" && (
                  <span className="dim">
                    <Icon name="spinner" className="spin" size={12} /> Posting…
                  </span>
                )}
              </header>
              {!l.state && (
                <div className="verdicts" role="radiogroup" aria-label={`Verdict for #${l.pr.number}`}>
                  {(["approve", "changes", "comment"] as Verdict[]).map((v) => (
                    <button
                      key={v}
                      role="radio"
                      aria-checked={l.verdict === v}
                      disabled={l.pr.mine && v !== "comment"}
                      title={l.pr.mine && v !== "comment" ? "It's your own pull request: GitHub takes comments on it, not a verdict" : undefined}
                      className={`verdict v-${v}${l.verdict === v ? " on" : ""}`}
                      onClick={() => pick(i, v)}
                    >
                      <Icon name={v === "approve" ? "check-circle" : v === "changes" ? "alert" : "message"} size={14} /> {verdictEvents[v]}{" "}
                      {i === focus && <Keys k={v === "approve" ? "a" : v === "changes" ? "c" : "m"} />}
                    </button>
                  ))}
                  {stack && !l.verdict && <span className="dim left-alone">Left alone</span>}
                </div>
              )}
              {l.verdict && !l.state && (
                <textarea
                  ref={i === focus ? area : undefined}
                  value={l.summary}
                  rows={2}
                  placeholder="A summary, if it helps (optional)"
                  aria-label={`Summary for #${l.pr.number}`}
                  onFocus={() => setFocus(i)}
                  onChange={(e) => set(i, { summary: e.target.value })}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
                      e.preventDefault();
                      submit();
                    }
                  }}
                />
              )}
              {l.verdict === "approve" && !l.state && l.pr.number === d.pr.number && noGo.length > 0 && (
                <Callout tone="attention" icon="alert" title={noGo.length === 1 ? `${noGo[0].title} is no go` : `${noGo.length} passes are no go`}>
                  {noGo.map((p) => `${p.title}: ${standingWords(p.standing).toLowerCase()}`).join("; ")}. Approving is yours to do: take what stands into your review, or dismiss what doesn't.
                </Callout>
              )}
              {l.error && <div className="banner">{humanError(l.error)}</div>}
              {l.verdict && l.posting && !l.state && <PostingView p={l.posting} />}
            </section>
          );
        })}
      </div>
      <div className="note-actions">
        {here?.draft && !busy && (
          <Button variant="ghost" onClick={onDiscard} title={`Throw away your draft comments on #${d.pr.number}`}>
            Discard the draft
          </Button>
        )}
        <span className="spacer-left" />
        <Button variant="ghost" onClick={onClose}>
          {layers.some((l) => l.state === "posted") ? "Close" : "Cancel"}
        </Button>
        <Button variant="primary" onClick={submit} disabled={!ready || busy} title={ready ? "Post to GitHub (⌘⏎)" : "Pick a verdict first"}>
          {busy ? "Posting…" : chosen.length > 1 ? `Post ${chosen.length} reviews to GitHub` : "Post to GitHub"} <Keys k="mod+⏎" />
        </Button>
      </div>
    </Dialog>
  );
}

// PostingView is exactly what will be posted on a pull request, and where.
function PostingView({ p }: { p: Posting }) {
  return (
    <div className="posting">
      <div className="posting-where dim">
        Goes to{" "}
        <a href={p.url} target="_blank" rel="noreferrer" className="link">
          {p.url.replace(/^https:\/\//, "")}
        </a>{" "}
        as “{p.event === "APPROVE" ? "Approve" : p.event === "REQUEST_CHANGES" ? "Request changes" : "Comment"}”, at commit <span className="mono">{p.commit.slice(0, 7)}</span>.
      </div>
      {p.body && <pre className="posting-body">{p.body}</pre>}
      {p.comments.map((c) => (
        <div key={c.id} className="posting-comment">
          <div className="mono dim">
            {c.path}:{c.startLine ? `${c.startLine}–${c.line}` : c.line}
            {c.side === "LEFT" ? " (removed lines)" : ""}
          </div>
          <pre>{c.body}</pre>
        </div>
      ))}
      {!p.body && !p.comments.length && <div className="dim">No words: just the verdict.</div>}
      {!!p.loose && (
        <div className="dim posting-loose">
          <Icon name="info" size={12} /> {p.loose === 1 ? "One comment is" : `${p.loose} comments are`} on lines outside the pull request's diff on GitHub, so {p.loose === 1 ? "it goes" : "they go"} in the review's body instead.
        </div>
      )}
    </div>
  );
}
