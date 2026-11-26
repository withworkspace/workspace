import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link, useNavigate, useParams, useSearchParams } from "react-router";
import {
  askPR,
  declinePR,
  discardPR,
  type Discussion,
  dockPR,
  getPR,
  getPRDiff,
  getPRDiscussion,
  getPRGuide,
  getPRPasses,
  getPRThreads,
  getReview,
  type PR,
  type PRDetail,
  prAPI,
  resendPR,
  respondPRThread,
  type Review as ReviewData,
  type ReviewGuide,
  type ReviewPass,
  type Unsent,
  useEvents,
  type Verdict,
} from "../api";
import { type AskAbout, linesChip } from "../ask/ask";
import { AskBar, type AskBarHandle } from "../ask/AskBar";
import type { AskedFinding, ReviewDiff, ReviewSource, Selection } from "../review/source";
import { ReviewView } from "../review/ReviewView";
import { Icon } from "../ui/Icon";
import { Spinner } from "../ui/Spinner";
import { useTitle } from "../ui/useTitle";
import { toast } from "../ui/toast";
import { Loading } from "../ui/Loading";
import { Empty } from "../ui/Empty";
import { Callout } from "../ui/Callout";
import { keyProps } from "../ui/keyHint";
import { confirm } from "../ui/Confirm";
import { Status } from "../ui/Status";
import { Time } from "../ui/Time";
import { GuideMark } from "./GuideMark";
import { GuideSection } from "./GuideSection";
import { PRDescription } from "./PRDescription";
import { PRDiscussion } from "./PRDiscussion";
import { StackDescriptions } from "./StackDescriptions";
import { PassesSection } from "./PassesSection";
import { PassMarks } from "./PassMarks";
import { PullRequestUnsent } from "./PullRequestUnsent";
import { useReviewPasses } from "./usePasses";
import { layersLeft } from "./after";
import { LayerList } from "./LayerList";
import { PRSubmitDialog } from "./PRSubmitDialog";
import { PullRequestMeta, pullRequestMetaChipClass, pullRequestMetaCodeClass } from "./PullRequestMeta";
import { PullRequestMetaChecks } from "./PullRequestMetaChecks";
import { PullRequestName } from "./PullRequestName";
import { PullRequestReviewer } from "./PullRequestReviewer";
import css from "./PRReview.module.css";
import { prHref, prStatus, reviewed } from "./Reviews";
import { useLatest } from "../load";
import { PagePin } from "../shell/PagePin";
import { Button, buttonClass } from "../ui/Button";
import { clip, humanError } from "../ui/strings";
import { useWorkspace } from "../workspace/store";
import { Text, textClass } from "../ui/Text";

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
  ["[ / ]", "Layer below / above"],
  ["w", "Whole stack / single layer"],
  ["/", "Ask bar"],
  ["a", "Ask about line or selection"],
  ["A", "Ask about the pull request"],
  ["o", "Open on GitHub"],
  ["O", "Open here"],
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

export function PRReviewPage() {
  const conversations = useWorkspace((w) => w.conversations);
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
    if (d.pr.fork) return toast("Could not open here: pull request is from a fork.");
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
  // It isn't yours to review: your name comes off its reviewers on GitHub,
  // which is where that's said, and it stops needing you. Others see it,
  // so it's asked first.
  const [declining, setDeclining] = useState(false);
  const decline = async () => {
    if (!d || declining) return;
    const yes = await confirm({
      title: "Not yours to review?",
      body: (
        <>
          Removes you from the reviewers of <Text mono>{ref}</Text> on GitHub. {d.pr.author} and others on it
          will see. No one is asked in your place.
        </>
      ),
      confirm: "Remove me",
    });
    if (!yes) return;
    setDeclining(true);
    declinePR(api)
      .then(
        () => {
          toast(`Removed from ${ref}'s reviewers.`, { tone: "done" });
          navigate("/reviews");
        },
        (e: Error) => toast(humanError(e.message), { tone: "danger" }),
      )
      .finally(() => setDeclining(false));
  };
  // What was said from here that GitHub didn't take: sent again, or let
  // go of. Either way the page reads what's left of them.
  const [settling, setSettling] = useState<number>();
  const settle = (u: Unsent, how: (api: string, id: number) => Promise<unknown>) => {
    setSettling(u.id);
    how(api, u.id)
      .then(
        () => load(),
        (e: Error) => {
          toast(humanError(e.message), { tone: "danger" });
          return load();
        },
      )
      .finally(() => setSettling(undefined));
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
        ? { icon: "list-checks" as const, text: `${h.finding.pass}: “${clip(h.finding.text, 28)}”`, title: `${h.finding.pass} pass finding (${h.finding.kind}): ${h.finding.text}`, kind: "finding" as const }
        : linesChip(h.lines!.path, h.lines!.line, h.lines!.endLine, h.lines!.side)),
      placeholder: h.finding ? "Ask about this finding…" : "Ask about these lines…",
      drop: "Ask about the whole pull request",
    }),
  };

  // The conversations about it, as the workspace has them now: answering,
  // or your turn.
  const talks = useMemo(() => conversations.filter((t) => t.pr === ref), [conversations, ref]);

  if (error) {
    return (
      <div className="page narrow">
        <Empty
          icon="pr"
          title={`Could not load ${ref}`}
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
        <Loading what={`Loading ${ref}…`} delay={150} />
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
    name: <PullRequestName title={whole && d.stack ? d.stack.title : pr.title} at={ref} layer={d.stack && d.layer ? { n: d.layer, of: layers.length } : undefined} whole={whole && d.stack ? { layers: layers.length } : undefined} />,
    back: { to: "/reviews", label: "Reviews" },
    headNow: whole ? undefined : d.head,
    updated: `${d.head}:${d.reviewed?.id ?? ""}:${took}`,
    // Looking again: what's changed since your last review, if anything has.
    sinceOf: () => (!whole && d.since ? `${d.reviewed?.id}:${d.since}` : undefined),
    getDiff: (since) => getPRDiff(api, whole ? "whole" : since ? "since" : "layer") as Promise<ReviewDiff>,
    diffKey: whole ? "whole" : "layer",
    sinceName: () => sinceName,
    movedWords: "New commits since loaded",
    empty: "No changes.",
    opened: shown.opened,
    readOnly: whole ? "Whole stack is read-only: comment on a layer ([ and ] switch layers)." : undefined,
    side: d.stack ? <LayerList d={d} whole={whole} going={going} search={viewQ} onWhole={toggleWhole} /> : undefined,
    actions: (
      <>
        <PagePin r={{ kind: "pr", repo: pr.repo, number: pr.number }} />
        <Button
          variant="ghost" size="small"
          className={css.desktopOnly}
          onClick={dock}
          disabled={docking}
          unavailable={pr.fork ? "Cannot open forks here" : undefined}
          title={d.docked?.by === "you" ? `Checked out in ${pr.repo}/${d.docked.capsule}` : "Check out here to run yourself"}
          keys="O"
        >
          <Icon name="box" size={13} /> {docking ? "Opening…" : d.docked?.by === "you" ? d.docked.capsule : "Open here"}
        </Button>
        <a className={buttonClass({ size: "small" })} href={pr.url} target="_blank" rel="noreferrer" {...keyProps("o", "Open on GitHub")}>
          <Icon name="external" size={12} /> GitHub
        </a>
      </>
    ),
    notices: (
      <>
        {!whole && d.unsent && <PullRequestUnsent unsent={d.unsent} busy={settling} onResend={(u) => settle(u, resendPR)} onDiscard={(u) => settle(u, discardPR)} />}
        <PullRequestMeta>
          {!whole && <Status s={prStatus(pr)} />}
          {!whole && pr.asked === "you" && pr.state === "needs-review" && (
            <Button variant="ghost" size="small" onClick={decline} disabled={declining} title="Remove me from reviewers on GitHub">
              {declining ? "Removing…" : "Not mine"}
            </Button>
          )}
          <span>
            {pr.mine ? "Yours" : pr.author} · <Text mono className={pullRequestMetaCodeClass}>{pr.branch}</Text> <Icon name="arrow-right" size={11} />{" "}
            <Text mono className={pullRequestMetaCodeClass}>{whole && d.stack ? d.stack.base : pr.base}</Text>
          </span>
          {pr.checks && !whole && <PullRequestMetaChecks checks={pr.checks} />}
          {!whole &&
            discussion?.reviewers.map((r) => <PullRequestReviewer key={r.login} login={r.login} kind={r.kind} stale={r.stale} />)}
          {pr.updated && <Time iso={pr.updated} as="relative" className={textClass("dim")} />}
          {!whole && !threads && !threadsError && (
            <Text dim title="Comments still loading">
              <Spinner size={12} /> Loading comments…
            </Text>
          )}
          {pr.mission && (
            <Link to={`/missions/${pr.mission.slug}`} className={pullRequestMetaChipClass("quiet")}>
              <Icon name="flag" size={11} /> {pr.mission.title}
            </Link>
          )}
          {d.docked && (
            <Link to={`/files/${pr.repo}/${d.docked.capsule}`} className={pullRequestMetaChipClass("quiet")} title={d.docked.by === "ws" ? "Checked out for a question. Removed when this merges or closes, unless you edit it." : "Browse files"}>
              <Icon name="box" size={11} /> {pr.repo}/{d.docked.capsule}
            </Link>
          )}
          {talks.map((t) => (
            <Link key={t.id} to={`/conversations/${t.id}`} className={pullRequestMetaChipClass(t.waiting && t.status === "open" ? "attention" : "quiet")} title={t.title || t.opening}>
              {t.answering ? <Spinner size={11} /> : <Icon name="message" size={11} />}
              {t.answering ? "Answering" : t.waiting && t.status === "open" ? "Answered" : t.status === "resolved" ? "Resolved" : "Asked"}: {clip(t.title || t.opening, 48)}
            </Link>
          ))}
        </PullRequestMeta>
        {d.needsRebase && (
          <Callout tone="attention" title="This layer needs a rebase">
            The layer below has new commits. Changes shown against the layer below as of this push.
          </Callout>
        )}
        {pr.fork && (
          <Callout tone="quiet" icon="info">
            From a fork: you can review here, but agents cannot answer questions about it.
          </Callout>
        )}
        {threadsError && (
          <Callout tone="quiet" icon="info">
            Could not load GitHub comments ({humanError(threadsError)}): showing only yours.
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
            // Posted behind, it's on its way: the foot says if it doesn't get there.
            const where = posted.some((p) => p.sending) ? "on its way to GitHub" : "on GitHub";
            toast(posted.length === 1 ? `Your review of ${pr.repo}#${posted[0].number} is ${where}.` : `Your ${posted.length} reviews are ${where.replace("its", "their")}.`, {
              tone: "done",
              action: url ? { label: "View on GitHub", run: () => window.open(url, "_blank", "noopener") } : undefined,
            });
            reviewed(pr.repo, pr.number);
            navigate("/reviews");
            return;
          }
          if (mine) done(mine);
          else {
            toast(posted.some((p) => p.sending) ? "On its way to GitHub." : "Posted to GitHub.", { tone: "done" });
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
        title={`${state.sending ? "On its way to GitHub" : "Posted to GitHub"}: ${verdictEvents[r.verdict!].toLowerCase()}`}
        actions={
          state.url ? (
            <a className={buttonClass()} href={state.url} target="_blank" rel="noreferrer">
              View on GitHub <Icon name="external" size={12} />
            </a>
          ) : undefined
        }
      >
        Your review of {ref} is {state.sending ? "on its way to GitHub" : "on GitHub"}. Back with you when
        they push, showing changes since.
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
        place="framed"
        // A comment added to the draft is on its lines, read again.
        onSettled={(p) => p.kind === "comment" && p.state === "accepted" && bumpTook()}
      />
    ),
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
    ? ["Is this right?", "How to fix this?", "Suggest a comment"]
    : h?.lines
      ? ["Explain these lines", "What calls this?", "What could go wrong here?"]
      : ["Summarise", "What could break?", "Where do I start reading?", "Is this tested?"];

// withFinding is a question about what a pass found, with what it found
// said after it: the card leads with the question.
const withFinding = (q: string, f: AskedFinding) => `${q}\n\nIt's about what the ${f.pass} pass found (${f.kind}): "${f.text}"`;
