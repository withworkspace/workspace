import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { Doing } from "../ui/Doing";
import { Link, useNavigate, useParams, useSearchParams } from "react-router";
import {
  ApiError,
  type ArtifactInfo,
  type DocKind,
  docAddress,
  proposalFrameURL,
  setTalkStatus,
  startTalk,
  type Talk,
  type TalkDetail,
  type TalkProposal,
  type TalkTurn,
} from "../api";
import { changes } from "../annotate/diff";
import { type Draft, MarkdownEditor } from "../editor/LazyEditor";
import { useKeys } from "../keys";
import { slugger } from "../markdown/links";
import { Markdown } from "../markdown/LazyMarkdown";
import { CrewBadge } from "../mission/parts";
import { Calls } from "../ui/Calls";
import { TLink } from "../ui/TLink";
import { Icon, type IconName } from "../ui/Icon";
import { Spinner } from "../ui/Spinner";
import { Inline } from "../ui/inline";
import { Callout, Empty, ErrorCard, humanError, Skeleton, toast, useDraft, useMedia, useTitle } from "../ui/kit";
import { Keys } from "../ui/Keys";
import { Status, type StatusInfo } from "../ui/status";
import { Time } from "../ui/time";
import { useHere } from "../load";
import { talkAt } from "../chrome/panes";
import { useTalk, useTalks } from "../conversation/talks";
import { RefTag } from "../refs/RefTag";
import { refKey } from "../refs/refs";
import { ArtifactProposalBody } from "./ArtifactProposal";
import { PluginProposalCard } from "./PluginProposal";
import { addedVersion, afterSettling, artifactNext, kindName, type Made, made, revisedText, type Settled, settled, startsMission } from "./proposals";
import { PagePin } from "../shell/focus";
import { Chip } from "../ui/Chip";
import { Button, buttonClass } from "../ui/Button";
import { MoreLink } from "../ui/MoreLink";
import { PageHead } from "../ui/PageHead";
import { Row, RowLine, RowMain, RowSub, RowTitle, Rows, rowLinkClass, rowStatusClass } from "../ui/Rows";
import { pageScroller } from "../ui/scroller";
import { ToggleButton } from "../ui/ToggleButton";
import { Input } from "../ui/Input";
import { TextArea } from "../ui/TextArea";
import { Composer } from "../ui/Composer";
import { PageBar } from "../ui/PageBar";
import { Turn, TurnAvatar, TurnHead, TurnSaid, turnTimeClass } from "../ui/Turn";
import { Change } from "../ui/Change";
import { Excerpt } from "../ui/Excerpt";
import { Suggestion } from "../review/Suggestion";
import { cx } from "../ui/cx";
import { useWorkspace } from "../workspace/store";
import { usePref } from "../prefs";
import { useThingMenu } from "../shell/ThingMenu";

// Conversations with an agent about the workspace: thinking something
// through before there's a mission. The agent reads anything (the repos,
// the missions, GitHub) and changes nothing; it answers in a run of its
// own, whose steps show as it goes.

export const talksKeys: [string, string][] = [
  ["n", "Start a new conversation"],
  ["j / k", "Next / previous"],
  ["⏎ / o", "Open it"],
  ["⇧A", "Show or hide resolved ones"],
];

export const talkKeys: [string, string][] = [
  ["mod+⏎", "Send"],
  ["r", "Write a reply"],
  ["x", "Resolve, or reopen"],
  ["esc", "Back to the conversations"],
];

const name = (t: { title?: string; opening?: string }) => t.title || t.opening || "New conversation";

// docName is a document's file name, from its ref (records:notes/x.md).
const docName = (ref: string) => ref.slice(ref.lastIndexOf("/") + 1).replace(/^.*:/, "");

// docAt is where a conversation about a document leads: the document, at
// the section it was asked of.
const docAt = (ref: string, section?: string) => docAddress(ref) + (section ? `#${slugger()(section)}` : "");

export function TalksPage() {
  const conversations = useWorkspace((w) => w.conversations);
  const kinds = useWorkspace((w) => w.kinds);
  useTitle("Conversations");
  const [showResolved, setShowResolved] = useState(false);
  const resolved = conversations.filter((t) => t.status !== "open").length;
  return (
    <div className="page narrow talks-page">
      <PageHead
        title="Conversations"
        actions={
          resolved > 0 && (
            <>
              <Button variant="ghost" size="small" onClick={() => setShowResolved((v) => !v)}>
                {showResolved ? "Hide resolved" : `Show resolved (${resolved})`} <Keys k="⇧A" />
              </Button>
            </>
          )
        }
      />
      <TalkList slug="" talks={conversations} showResolved={showResolved} onToggleResolved={() => setShowResolved((v) => !v)} keys docs={!!kinds?.length} />
    </div>
  );
}

// talkHref is a conversation's page: the workspace's, or a mission's.
export const talkHref = (slug: string, id: string) => (slug ? `/missions/${slug}/conversations/${id}` : `/conversations/${id}`);

export function talkStatus(t: Pick<Talk, "status" | "answering" | "waiting">): StatusInfo {
  if (t.status === "resolved") return { icon: "check-circle", tone: "quiet", words: "Resolved" };
  if (t.answering) return { icon: "spinner", tone: "progress", words: "Answering", live: true };
  if (t.waiting) return { icon: "reply", tone: "attention", words: "Your turn" };
  return { icon: "message", tone: "quiet", words: "Open" };
}

// TalkList starts a conversation and lists them: the workspace's, or a
// mission's on its Talk tab (where the page's own keys apply, not these).
// docs: the workspace has kinds of document, so its agent can propose one.
// planner is the mission's planner's conversation, which opens on the
// brief's own page (where what it proposes is read and confirmed).
export function TalkList({
  slug,
  talks: all,
  showResolved,
  onToggleResolved,
  keys,
  docs,
  planner,
}: {
  slug: string;
  talks: Talk[];
  showResolved: boolean;
  onToggleResolved?: () => void;
  keys?: boolean;
  docs?: boolean;
  planner?: string;
}) {
  const menu = useThingMenu();
  const hrefOf = (id: string) => (slug && id === planner ? `/missions/${slug}/brief` : talkHref(slug, id));
  const navigate = useNavigate();
  const [params, setParams] = useSearchParams();
  const fresh = params.get("new") === "1";
  const talks = useMemo(() => all.filter((t) => showResolved || t.status === "open"), [all, showResolved]);
  const [index, setIndex] = useState(0);
  const [text, setText] = useDraft(`new:${slug}`);
  const [starting, setStarting] = useState(false);
  const box = useRef<HTMLTextAreaElement>(null);
  // From "New mission": straight into words.
  useEffect(() => {
    if (fresh) box.current?.focus();
  }, [fresh]);
  const open = () => talks[index] && navigate(hrefOf(talks[index].id));
  const move = (i: number) => {
    const n = Math.min(Math.max(i, 0), talks.length - 1);
    setIndex(n);
    document.getElementById(`talk-${talks[n]?.id}`)?.scrollIntoView({ block: "nearest" });
  };
  useKeys(
    keys
      ? {
          n: () => box.current?.focus(),
          j: () => move(index + 1),
          ArrowDown: () => move(index + 1),
          k: () => move(index - 1),
          ArrowUp: () => move(index - 1),
          Enter: open,
          o: open,
          A: () => onToggleResolved?.(),
        }
      : {},
  );
  const start = () => {
    if (!text.trim() || starting) return;
    setStarting(true);
    startTalk(slug, text)
      .then((d) => {
        setText("");
        navigate(talkHref(slug, d.id));
      })
      .catch((e: Error) => toast(humanError(e.message), { tone: "danger" }))
      .finally(() => setStarting(false));
  };
  const placeholder = slug
    ? "Think about this mission: is the plan still right, what should happen next, what about…"
    : fresh
      ? "Describe the mission you have in mind: what, and why. The agent reads the code and proposes a brief."
      : "What are you thinking about? An idea, a problem, a question about the code…";
  return (
    <>
      <Composer
        className={cx("new-talk", fresh && "fresh")}
        field={box}
        value={text}
        onChange={setText}
        onSend={start}
        rows={2}
        tallest={280}
        aria-label="Start a conversation"
        placeholder={placeholder}
        onBlur={() => fresh && !text && setParams({}, { replace: true })}
        hint={
          <>
            <Icon name="eye" size={12} />
            {slug
              ? "The agent reads the mission and everything around it and changes nothing. It can propose a decision, a brief, crew, a replan or a follow-up."
              : `The agent reads the repos, the missions and GitHub, and changes nothing. It can propose a mission${docs ? ", a decision or a document" : " or a decision"}.`}
          </>
        }
      >
        <Button variant="primary" disabled={!text.trim() || starting} onClick={start}>
          {starting ? "Starting…" : "Start"}
        </Button>
      </Composer>
      {talks.length === 0 ? (
        all.length ? (
          <Empty icon="check-circle" title="Nothing open">
            Every conversation's resolved.{" "}
            {onToggleResolved && (
              <MoreLink onClick={onToggleResolved}>
                Show them
              </MoreLink>
            )}
          </Empty>
        ) : (
          <Empty icon="message" title="No conversations yet">
            {slug
              ? "Talk the mission through with an agent: whether the plan's still right, what's next, what was decided."
              : "Think out loud with an agent that's read the code: an idea, a problem, how something works. When it's ready, it proposes a mission."}
          </Empty>
        )
      ) : (
        <Rows className="talks" role="listbox" aria-label="Conversations">
          {talks.map((t, i) => {
            const st = talkStatus(t);
            const on = keys && i === index;
            return (
              <Row
                selected={on}
                key={t.id}
                id={`talk-${t.id}`}
                role="option"
                aria-selected={!!on}
                className="talk-row"
                onMouseMove={() => i !== index && setIndex(i)}
              >
                <TLink to={hrefOf(t.id)} className={rowLinkClass("talk-link")} onContextMenu={menu(hrefOf(t.id), t.title || t.opening)}>
                  <Status s={st} iconOnly size={16} className={rowStatusClass()} />
                  <RowMain>
                    <RowLine>
                      <RowTitle>{name(t)}</RowTitle>
                      {t.waiting && t.status === "open" && <Chip tone="attention">Your turn</Chip>}
                      {!!t.proposed && <Chip tone="attention">{t.proposed} to confirm</Chip>}
                      {t.pr && (
                        <Chip tone="quiet" title="The pull request it's about">
                          <Icon name="pr" size={11} />
                          {t.pr}
                        </Chip>
                      )}
                      {t.doc && (
                        <Chip tone="quiet" title={`The document it's about: ${t.doc}${t.section ? `, § ${t.section}` : ""}`}>
                          <Icon name="book" size={11} />
                          {docName(t.doc)}
                        </Chip>
                      )}
                      {t.became && (
                        <Chip tone="quiet" title="The mission it became">
                          <Icon name="flag" size={11} />
                          {t.became}
                        </Chip>
                      )}
                    </RowLine>
                    {t.summary && (
                      <RowSub>
                        <Inline text={t.summary} lines={1} />
                      </RowSub>
                    )}
                  </RowMain>
                  <Time iso={t.updated || t.at} as="age" className="age" />
                </TLink>
              </Row>
            );
          })}
        </Rows>
      )}
      {!keys && all.some((t) => t.status !== "open") && onToggleResolved && talks.length > 0 && (
        <MoreLink onClick={onToggleResolved}>
          {showResolved ? "Hide" : "Show"} resolved
        </MoreLink>
      )}
    </>
  );
}

// The page scrolls in the app's main pane. Where a conversation rests as
// it goes on: at its end, but never past the start of the reply that's
// just come (a long one lands whole, to be read from its top). head is
// the height of the bar stuck to the top; reply is the latest reply's
// words, if the latest turn isn't yours.
const restAt = (m: HTMLElement, reply: HTMLElement | null, head: number) => {
  const end = m.scrollHeight - m.clientHeight;
  if (!reply) return end;
  // (Its turn's heading and folded steps are above its words.)
  const top = reply.getBoundingClientRect().top - m.getBoundingClientRect().top + m.scrollTop - head - 56;
  return Math.max(0, Math.min(end, top));
};

const proposalIcons: Record<string, IconName> = { decision: "gavel", mission: "flag", brief: "pencil", dispatch: "play", replan: "rotate", comment: "message", plan: "list-ordered", document: "file", artifact: "artifact" };

export function TalkPage() {
  const workspace = useWorkspace();
  const { id = "", slug = "" } = useParams();
  const navigate = useNavigate();
  const listed = slug ? undefined : workspace.conversations.find((t) => t.id === id);
  const mission = slug ? workspace.missions.find((m) => m.slug === slug) : undefined;
  const back = slug ? `/missions/${slug}?tab=talk` : "/conversations";
  const [text, setText] = useDraft(`talk:${slug}/${id}`);
  const [sending, setSending] = useState(false);
  const [unseen, setUnseen] = useState(false);
  const box = useRef<HTMLTextAreaElement>(null);
  // What's come of it, beside it: when there's room, unless it's put away.
  const wide = useMedia("(min-width: 1240px)");
  const [sideOn, setSideOn] = usePref("talkSide");
  const toggleSide = () => setSideOn((on) => !on);
  const side = wide && sideOn;
  // Load again whenever the workspace says it's changed: a turn, or the
  // run's output as it goes.
  const key = (slug ? mission?.updated : listed?.updated) ?? "";
  // It's the window's (conversation/talks.ts), as the pane beside a page
  // has it: what's said in one shows in the other. Shown here, it's
  // followed while it's answered.
  const to = talkAt(id, slug);
  const held = useTalk(to);
  const d = held?.detail;
  const error = held?.gone || !d ? held?.error : undefined;
  const setD = (x: TalkDetail) => useTalks.getState().told(to, x);
  const load = useCallback(() => useTalks.getState().load(to), [to]);
  useEffect(() => useTalks.getState().watch(to), [to]);
  useEffect(() => {
    void load();
  }, [load, key]);
  const title = d ? name({ title: d.title, opening: d.turns[0]?.body }) : "";
  useTitle(title || "Conversation");

  // Where the page rests. It opens at the end, and stays there while
  // what's in it is still arriving (pictures, diagrams, frames). After
  // that it follows what comes, a step at a time and without gliding
  // (the agent's steps land every couple of seconds: a glide after each
  // is the page never still), unless you've scrolled up to read, when a
  // pill says there's more. It only ever moves down by itself.
  const page = useRef<HTMLDivElement>(null);
  const head = useRef<HTMLDivElement>(null);
  const reply = useRef<HTMLDivElement>(null); // the latest reply's words
  const follow = useRef(true);
  const opened = useRef(0); // when it was first drawn; 0 before
  const replyTop = useRef<number>(undefined); // where the reply's words were, a run going, as last seen
  const rest = () => {
    const m = pageScroller();
    return m ? restAt(m, Date.now() - opened.current < 1500 ? null : reply.current, head.current?.offsetHeight ?? 0) : 0;
  };
  const seen = () => {
    replyTop.current = reply.current?.getBoundingClientRect().top;
  };
  useEffect(() => {
    opened.current = 0;
    follow.current = true;
    setUnseen(false);
  }, [slug, id]);
  useEffect(() => {
    const m = pageScroller();
    if (!m) return;
    const onScroll = () => {
      follow.current = m.scrollTop >= rest() - 120;
      if (follow.current) setUnseen(false);
      seen();
    };
    m.addEventListener("scroll", onScroll, { passive: true });
    return () => m.removeEventListener("scroll", onScroll);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  // Whatever makes the page taller (a turn, a step, a picture loading,
  // the box you're writing in growing) is followed the same way.
  const loaded = !!d;
  useEffect(() => {
    const m = pageScroller();
    if (!m || !page.current || !loaded) return;
    const settle = () => {
      if (follow.current) {
        const to = rest();
        if (to > m.scrollTop + 1) m.scrollTop = to;
      }
      seen();
    };
    if (!opened.current) {
      opened.current = Date.now();
      m.scrollTop = m.scrollHeight;
    }
    const watch = new ResizeObserver(settle);
    watch.observe(page.current);
    // Once it's open, the latest reply's start counts again.
    const t = setTimeout(settle, 1600);
    return () => {
      watch.disconnect();
      clearTimeout(t);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [loaded, slug, id]);
  // A reply, done: its steps fold away above it, and its words stay
  // where they were being read.
  const turns = d?.turns.length ?? 0;
  const answering = !!d?.answering;
  const was = useRef({ answering, turns });
  const landed = useRef({ of: "", turns: new Set<number>() });
  useLayoutEffect(() => {
    const m = pageScroller();
    const before = was.current;
    was.current = { answering, turns };
    if (!m || !opened.current) return;
    if (before.answering && !answering && turns > before.turns && replyTop.current !== undefined && reply.current) {
      m.scrollTop += reply.current.getBoundingClientRect().top - replyTop.current;
      // Its start's where you were looking: it isn't then moved to the top.
      follow.current = m.scrollTop >= rest() - 120;
    }
    // A turn you weren't there for: say so, where you are.
    else if (turns > before.turns && !follow.current) setUnseen(true);
    seen();
  }, [answering, turns]);

  const send = () => {
    if (!text.trim() || sending) return;
    setSending(true);
    follow.current = true;
    useTalks
      .getState()
      .say(to, text)
      .then(
        () => setText(""),
        (e: Error) => toast(humanError(e.message), { tone: "danger" }),
      )
      .finally(() => setSending(false));
  };
  // Settling a card: a brief confirmed that starts its mission goes to the
  // mission's summary; a conversation that moved (its mission renamed) is
  // followed. Either, only if you're still here when ws has answered.
  const here = useHere();
  const settle = (n: number, action: "accept" | "decline", edits = {}) => {
    const p = d?.proposals?.find((x) => x.n === n);
    return useTalks.getState().settle(to, n, action, edits).then(
      (x) => {
        if (x.message) toast(x.message, { tone: "done" });
        const to = afterSettling(p, action, slug, x.path);
        if (to && here()) navigate(to, { replace: to === x.path });
      },
      (e: Error) => {
        toast(humanError(e.message), { tone: "danger" });
        // A plugin's items done before one wasn't are kept: show them.
        void load();
      },
    );
  };
  // An artifact proposed runs from the conversation's records.
  const frameOf = (n: number) => proposalFrameURL(slug, id, n);
  // Each proposal sits under the reply it came with: the first of the
  // agent's turns since it was made (or the run going now).
  // (Times compared as times: Go drops trailing zeros from fractions of a
  // second, so their strings don't sort.)
  const proposals = d?.proposals ?? [];
  const agentTurns = (d?.turns ?? []).filter((t) => t.by !== "you");
  const ms = (iso: string) => new Date(iso).getTime();
  const under = (t: TalkTurn) => {
    const i = agentTurns.indexOf(t);
    const after = i > 0 ? ms(agentTurns[i - 1].at) : -Infinity;
    return proposals.filter((p) => ms(p.at) > after && ms(p.at) <= ms(t.at));
  };
  const pending = proposals.filter((p) => !agentTurns.length || ms(p.at) > ms(agentTurns[agentTurns.length - 1].at));
  const became = proposals.filter((p) => p.kind === "mission" && p.state === "accepted").pop()?.result;
  // The proposal a revision revises, to say what it changed.
  const revisedFrom = (p: TalkProposal) => (p.revises ? proposals.find((q) => q.n === p.revises) : undefined);
  const toggle = () =>
    d &&
    setTalkStatus(slug, id, d.status === "resolved" ? "open" : "resolved").then(
      (x) => {
        setD(x);
        toast(x.status === "resolved" ? "Resolved." : "Reopened.", { tone: "done" });
      },
      (e: Error) => toast(humanError(e.message), { tone: "danger" }),
    );
  const keep = () =>
    useTalks.getState().keep(to).then(
      () => toast("Kept: it's in Conversations.", { tone: "done" }),
      (e: Error) => {
        if (!(e instanceof ApiError && e.status === 404)) return toast(humanError(e.message), { tone: "danger" });
        toast("That ask's gone: nobody kept it.");
        navigate(back);
      },
    );
  useKeys({
    r: () => box.current?.focus(),
    x: toggle,
    Escape: () => navigate(back),
  });
  // To a proposal's card, from beside the conversation: clear of the bar.
  const goTo = (n: number) => {
    const m = pageScroller();
    const card = document.getElementById(`proposal-${n}`);
    if (!m || !card) return;
    m.scrollTo({ top: card.getBoundingClientRect().top - m.getBoundingClientRect().top + m.scrollTop - (head.current?.offsetHeight ?? 0) - 12, behavior: "smooth" });
    card.classList.remove("flash");
    void card.offsetWidth; // so it flashes again, asked twice
    card.classList.add("flash");
  };

  const st = d ? talkStatus({ status: d.status, answering: d.answering, waiting: listed?.waiting }) : undefined;
  // The reply whose start the page won't scroll past: the run's words so
  // far, or the latest turn's, when that isn't yours.
  const last = d?.turns[d.turns.length - 1];
  const running = !!d?.answering && d.status === "open";
  const latestReply = !running && last && last.by !== "you" && !settled(last, proposals) ? last.n : undefined;
  // A turn that was the run going a moment before is already there to
  // read: it doesn't come in again.
  if (landed.current.of !== `${slug}/${id}`) landed.current = { of: `${slug}/${id}`, turns: new Set() };
  if (was.current.answering && !answering && latestReply !== undefined) landed.current.turns.add(latestReply);
  return (
    <div className={`page talk-page${side ? " with-side" : ""}`} ref={page}>
      <PageBar
        ref={head}
        className="talk-pagebar"
        back={{ to: back, title: slug ? `Back to ${mission?.title ?? slug}` : "Back to the conversations" }}
        status={st && <Status s={st} iconOnly size={16} />}
        title={d ? title : undefined}
        actions={
          <>
            {slug && d?.goal === "brief" && (
              <Link to={`/missions/${slug}/brief`} className={buttonClass({ size: "small" })} title="The brief and plan it's about, on their own page: what it proposes for them is read and confirmed there">
                <Icon name="file" size={13} /> The brief
              </Link>
            )}
            {became && (
              <Link to={`/missions/${became}`} className={buttonClass({ size: "small" })}>
                <Icon name="flag" size={13} /> Its mission
              </Link>
            )}
            {wide && (
              <ToggleButton variant="ghost" size="small" pressed={sideOn} onChange={toggleSide} title="Beside the conversation: what's to confirm, what it's made, what it names, and its summary so far">
                <Icon name="panel" size={12} /> Details
              </ToggleButton>
            )}
            {d && (
              <Button onClick={toggle} title={`${d.status === "resolved" ? "Reopen" : "Resolve"} (x)`}>
                <Icon name={d.status === "resolved" ? "rotate" : "check"} size={13} />
                {d.status === "resolved" ? "Reopen" : "Resolve"}
              </Button>
            )}
          </>
        }
      >
        {!slug && <PagePin r={{ kind: "talk", id }} />}
        <div className="talk-about">
          {slug && (
            <Link to={`/missions/${slug}`} className="talk-mission" title="The mission it's about">
              <Icon name="flag" size={11} /> {mission?.title ?? slug}
            </Link>
          )}
          {d?.anchor?.pr && (
            <Link to={`/reviews/${d.anchor.pr.replace("#", "/")}`} className="talk-mission" title="The pull request it's about">
              <Icon name="pr" size={11} /> {d.anchor.pr}
              {d.anchor.path && (
                <span className="mono">
                  {" "}
                  · {d.anchor.path}:{d.anchor.line}
                  {d.anchor.endLine && d.anchor.endLine > (d.anchor.line ?? 0) ? `–${d.anchor.endLine}` : ""}
                </span>
              )}
            </Link>
          )}
          {d?.anchor?.doc && (
            <Link to={docAt(d.anchor.doc, d.anchor.section)} className="talk-mission" title={`The document it's about${d.anchor.section ? ", at the section it was asked of" : ""}`}>
              <Icon name="book" size={11} /> <span className="mono">{d.anchor.path ?? d.anchor.doc}</span>
              {d.anchor.section && <span> · § {d.anchor.section}</span>}
            </Link>
          )}
          {d && <Time iso={d.at} as="relative" className="dim" />}
        </div>
      </PageBar>
      <div className="talk-main">
        {error && <ErrorCard error={error} retry={load} />}
        {d?.summary && !side && (
          <details className="talk-summary">
            <summary>
              <Icon name="chevron-right" size={12} className="fold-chevron" /> Summary so far
            </summary>
            <Markdown text={d.summary} />
          </details>
        )}
        {!d && !error && <Skeleton rows={6} />}
        {d?.via && (
          <p className="dim talk-via">
            Started from the {d.via} plugin, which worded the first message.
          </p>
        )}
        {d?.ephemeral && (
          <Callout
            tone="quiet"
            icon="sparkle"
            title="An ask nobody's kept"
            actions={
              <Button size="small" onClick={keep}>
                <Icon name="message" size={12} /> Keep as a conversation
              </Button>
            }
          >
            It isn't listed with the rest, and it goes an hour after its last turn unless you keep it.
          </Callout>
        )}
        {!!d?.anchor?.excerpt?.length && (
          <Excerpt className="talk-anchor" lines={d.anchor.excerpt} from={d.anchor.line ?? 1} title={`${d.anchor.path}, the lines the question is about`} />
        )}
        {!d?.anchor?.excerpt?.length && d?.anchor?.quote?.exact && (
          <blockquote className="talk-anchor talk-quote" title="The passage it was asked of">
            {d.anchor.quote.exact}
          </blockquote>
        )}
        <div className="talk">
          {d?.turns.map((t) => (
            <div key={t.n} className={settled(t, proposals) ? "turn-group settled" : "turn-group"}>
              <TurnView t={t} root={workspace.root} proposals={proposals} said={t.n === latestReply ? reply : undefined} landed={landed.current.turns.has(t.n)} />
              {t.by !== "you" &&
                under(t).map((p) => <ProposalCard key={p.n} p={p} revisedFrom={revisedFrom(p)} kinds={workspace.kinds} artifacts={workspace.artifacts} frameOf={frameOf} slug={slug} onSettle={settle} anchor={`proposal-${p.n}`} />)}
            </div>
          ))}
          {running && (
            <Turn by="agent" running aria-live="polite" head={<Who by="agent" />}>
                {d.run?.said && (
                  <TurnSaid className="dim" ref={reply}>
                    <Markdown text={d.run.said} />
                  </TurnSaid>
                )}
                {pending.map((p) => (
                  <ProposalCard key={p.n} p={p} revisedFrom={revisedFrom(p)} kinds={workspace.kinds} artifacts={workspace.artifacts} frameOf={frameOf} slug={slug} onSettle={settle} replying anchor={`proposal-${p.n}`} />
                ))}
                <Doing calls={d.run?.calls ?? []} root={workspace.root} />
            </Turn>
          )}
        </div>
        <div className="talk-reply">
          {unseen && (
            <button
              className="new-pill"
              onClick={() => {
                pageScroller()?.scrollTo({ top: rest(), behavior: "smooth" });
                setUnseen(false);
              }}
            >
              <Icon name="arrow-down" size={12} /> New reply
            </button>
          )}
          <Composer
            field={box}
            value={text}
            onChange={setText}
            onSend={send}
            aria-label="Reply"
            placeholder={d?.answering ? "Say more: it's sent once the agent's done" : d?.status === "resolved" ? "Reply to reopen it…" : "Reply…"}
          >
            <Button variant="primary" disabled={!text.trim() || sending} onClick={send}>
              {sending ? "Sending…" : "Send"}
            </Button>
          </Composer>
        </div>
      </div>
      {side && <TalkSide d={d} slug={slug} id={id} artifacts={workspace.artifacts} goTo={goTo} />}
    </div>
  );
}

// TalkSide is what's come of a conversation, beside it: what's waiting
// to be confirmed, what it's made (each where it is now), what it names,
// and its summary so far. A thing to confirm goes to its card; a thing
// made goes to the thing, with its card a click beside it.
function TalkSide({ d, slug, id, artifacts, goTo }: { d?: TalkDetail; slug: string; id: string; artifacts?: ArtifactInfo[]; goTo: (n: number) => void }) {
  const [allNamed, setAllNamed] = useState(false);
  const items = useMemo(() => made(d?.proposals ?? [], slug, (a) => artifacts?.find((x) => x.id === a)?.title), [d?.proposals, slug, artifacts]);
  // What its turns name, once each, in the order first named: but for
  // whole repos (every turn names one) and the conversation itself.
  const named = useMemo(() => {
    const seen = new Map<string, string>();
    for (const t of d?.turns ?? [])
      for (const r of t.refs ?? []) {
        if (r.ref.kind === "repo" || (r.ref.kind === "talk" && r.ref.id === id)) continue;
        const k = refKey(r.ref);
        if (!seen.has(k)) seen.set(k, r.said);
      }
    return [...seen];
  }, [d?.turns, id]);
  const open = items.filter((m) => m.open);
  const done = items.filter((m) => !m.open);
  const shown = allNamed ? named : named.slice(0, 12);
  const row = (m: Made) => (
    <>
      <Icon name={proposalIcons[m.kind] ?? "pencil"} size={12} />
      <span>
        {m.name}
        {m.tag && <span className="side-tag"> {m.tag}</span>}
      </span>
    </>
  );
  return (
    <aside className="page-map talk-side" aria-label="Beside the conversation">
      {open.length > 0 && (
        <section>
          <h3>To confirm</h3>
          <ul>
            {open.map((m) => (
              <li key={m.n}>
                <button className="map-item side-open" onClick={() => goTo(m.n)}>
                  {row(m)}
                </button>
              </li>
            ))}
          </ul>
        </section>
      )}
      {done.length > 0 && (
        <section>
          <h3>Made here</h3>
          <ul>
            {done.map((m) => (
              <li key={m.n} className="side-made">
                {m.to ? (
                  <Link to={m.to} className="map-item">
                    {row(m)}
                  </Link>
                ) : (
                  <button className="map-item" onClick={() => goTo(m.n)}>
                    {row(m)}
                  </button>
                )}
                {m.to && (
                  <Button variant="ghost" square className="side-where" onClick={() => goTo(m.n)} title="Where it was proposed, in the conversation">
                    <Icon name="arrow-up" size={12} />
                    <span className="sr-only">Where it was proposed</span>
                  </Button>
                )}
              </li>
            ))}
          </ul>
        </section>
      )}
      {(d?.capsules?.length ?? 0) > 0 && (
        <section>
          <h3 title="Capsules its agent lifted to try things in: the conversation's own, not a mission's, and put away with it">Capsules</h3>
          <ul className="side-named">
            {d!.capsules!.map((c) => (
              <li key={`${c.repo}/${c.name}`} title={c.away ? `Put away with the conversation, its branch (${c.branch}) kept${c.kept ? ", with what wasn't committed" : ""}` : `On branch ${c.branch}`}>
                <code>
                  {c.repo}/{c.name}
                </code>
                {c.away && <span className="dim"> put away</span>}
              </li>
            ))}
          </ul>
        </section>
      )}
      {named.length > 0 && (
        <section>
          <h3>Mentioned</h3>
          <ul className="side-named inline-md">
            {shown.map(([k, said]) => (
              <li key={k}>
                <RefTag refKey={k}>{said}</RefTag>
              </li>
            ))}
          </ul>
          {named.length > shown.length && (
            <MoreLink onClick={() => setAllNamed(true)}>
              {named.length - shown.length} more
            </MoreLink>
          )}
        </section>
      )}
      {d?.summary && (
        <section>
          <h3>Summary so far</h3>
          <div className="side-summary">
            <Markdown text={d.summary} />
          </div>
        </section>
      )}
      {d && !items.length && !named.length && !d.summary && !d.capsules?.length && <p className="map-empty">As it goes on, this is what's come of the conversation: what's to confirm, what it's made, what it names, and its summary so far.</p>}
    </aside>
  );
}

const whoName = (by: string) => ({ you: "You", agent: "Agent", ws: "ws" })[by] ?? by;

// Who is whoever said a turn, over it: you, the agent, ws, or crew by
// their badge.
function Who({ by, at }: { by: string; at?: string }) {
  const known = by === "you" || by === "agent" || by === "ws";
  return <TurnHead avatar={known ? <TurnAvatar by={by} /> : <CrewBadge id={by} />} who={whoName(by)} at={at} />;
}

// TurnView is a turn: yours as a bubble, the agent's (its steps folded
// above it) or ws's as text; one ws wrote for a card you settled, as its
// marker. said is given its words' box (the conversation's latest reply:
// where the page rests); landed is one that was the run going until now,
// which doesn't come in again.
function TurnView({ t, root, proposals, said, landed }: { t: TalkTurn; root: string; proposals: TalkProposal[]; said?: React.Ref<HTMLDivElement>; landed?: boolean }) {
  const m = settled(t, proposals);
  if (m) return <SettledMarker m={m} t={t} />;
  const who = t.by === "you" || t.by === "agent" || t.by === "ws" ? t.by : "crew";
  return (
    <Turn by={who} landed={landed} head={<Who by={t.by} at={t.at} />}>
      {t.calls && t.calls.length > 0 && <Calls calls={t.calls} root={root} />}
      <TurnSaid ref={said} written={t.by === "you"}>
        {t.by === "you" ? t.body : <Markdown text={t.body} pinned={t.refs} />}
      </TurnSaid>
    </Turn>
  );
}

// SettledMarker is a turn ws wrote for you settling a card, which the
// agent read as yours: a muted line saying what became of it ("Accepted:
// mission token-cache"), never your bubble. Its title is the whole of
// its first line, as the agent read it; what you edited first, which the
// agent was told too, opens under it. compact leaves out its time (in an
// ask's card).
export function SettledMarker({ m, t, compact }: { m: Settled; t: TalkTurn; compact?: boolean }) {
  const [open, setOpen] = useState(false);
  return (
    <div className={`settled-marker ${m.accepted ? "accepted" : "declined"}`}>
      <div className="settled-marker-line" title={t.body.split("\n")[0]}>
        <Icon name={m.accepted ? "check-circle" : "x-circle"} size={13} className="settled-marker-icon" />
        <span className="settled-marker-text">
          {m.accepted ? "Accepted" : "Declined"}: {m.to ? <Link to={m.to}>{m.what}</Link> : m.what}
        </span>
        {m.edits && (
          <Button variant="link" type="button" className="settled-marker-more" aria-expanded={open} onClick={() => setOpen(!open)}>
            edited <Icon name={open ? "chevron-down" : "chevron-right"} size={11} />
          </Button>
        )}
        {!compact && <Time iso={t.at} className={turnTimeClass("settled-marker-time")} />}
      </div>
      {open && m.edits && (
        <div className="settled-marker-edits">
          <Markdown text={m.edits} />
        </div>
      )}
    </div>
  );
}

// ProposalCard is something the agent proposed, under its reply: a
// decision to record, a mission to create (ready, with its brief) or save
// as an idea, an edit to the mission's brief (briefing it, with its plan),
// or its plan, crew to start, a replan by the mission's planner, a review
// comment, a document to add (of one of the workspace's kinds, named by
// kinds), or an artifact (or a version of one, of artifacts), running in
// the frame frameOf gives it. Edit it first, or not; confirming does it,
// as you. inReview is a review comment's card on the review itself (in its
// ask bar's card): it doesn't lead there. slug is the mission the
// conversation's about, if it's a mission's: a replan leads to its
// planner's conversation there. replying: the agent's still writing the
// reply it came with, so it can't be confirmed until that's there to read.
// anchor is its id on the page, where it's gone to from beside the
// conversation.
export function ProposalCard(props: ProposalCardProps) {
  if (props.p.kind === "plugin")
    return (
      <PluginProposalCard
        p={props.p}
        changes={props.revisedFrom && props.p.state === "open" ? <RevisionChanges was={props.revisedFrom} now={props.p} /> : undefined}
        replying={props.replying}
        anchor={props.anchor}
        onSettle={props.onSettle}
      />
    );
  return <WsProposalCard {...props} />;
}

interface ProposalCardProps {
  p: TalkProposal;
  revisedFrom?: TalkProposal;
  kinds?: DocKind[];
  artifacts?: ArtifactInfo[];
  frameOf?: (n: number) => string;
  inReview?: boolean;
  slug?: string;
  replying?: boolean;
  anchor?: string;
  onSettle: (n: number, action: "accept" | "decline", edits?: object) => Promise<void>;
}

function WsProposalCard({
  p,
  revisedFrom,
  kinds,
  artifacts,
  frameOf,
  inReview,
  slug,
  replying,
  anchor,
  onSettle,
}: {
  p: TalkProposal;
  revisedFrom?: TalkProposal;
  kinds?: DocKind[];
  artifacts?: ArtifactInfo[];
  frameOf?: (n: number) => string;
  inReview?: boolean;
  slug?: string;
  replying?: boolean;
  anchor?: string;
  onSettle: (n: number, action: "accept" | "decline", edits?: object) => Promise<void>;
}) {
  const [editingOpen, setEditing] = useState(false);
  // Once it's settled, it shows what was proposed, edited or not.
  const editing = editingOpen && p.state === "open";
  const [busy, setBusy] = useState(false);
  const [what, setWhat] = useState(p.what ?? "");
  const [why, setWhy] = useState(p.why ?? "");
  const [title, setTitle] = useState(p.title ?? "");
  // The brief, as the editor has it: its markdown's worked out on accept.
  const [brief, setBrief] = useState<Draft>();
  const [text, setText] = useState(p.text ?? "");
  // The plan is the brief's editor's too.
  const proposedText = p.kind === "plan" ? (p.plan ?? "") : (p.brief ?? "");
  const briefText = () => (brief ? brief.text() : proposedText);
  const briefEmpty = brief ? brief.empty : !proposedText.trim();
  // A document, as the editor has it; and its kind, as the workspace names it.
  const [doc, setDoc] = useState<Draft>();
  const docText = () => (doc ? doc.text() : (p.text ?? ""));
  const docEmpty = doc ? doc.empty : !p.text?.trim();
  const kindTitle = kinds?.find((k) => k.name === p.docKind)?.title ?? p.docKind ?? "Document";
  // An artifact: its note, as the editor has it; and the artifact a
  // version's of, as the workspace has it now.
  const [note, setNote] = useState(p.note ?? "");
  const artifact = p.to ? artifacts?.find((a) => a.id === p.to) : undefined;
  const edits = () => {
    if (!editing) return {};
    switch (p.kind) {
      case "decision":
        return { what, why };
      case "mission":
        return { title, brief: briefText() };
      case "brief":
        return { brief: briefText() };
      case "plan":
        return { plan: briefText() };
      case "dispatch":
      case "replan":
      case "comment":
        return { text };
      case "document":
        return { text: docText() };
      case "artifact":
        return { title, note };
    }
  };
  const run = (action: "accept" | "decline") => {
    if (busy) return;
    setBusy(true);
    onSettle(p.n, action, action === "accept" ? edits() : {}).finally(() => setBusy(false));
  };
  // Briefing a mission, the brief comes with its name and capsules.
  const briefing = startsMission(p);
  const label = {
    decision: "Decision",
    mission: p.brief ? "Mission" : "Mission idea",
    brief: briefing ? "Mission brief" : "Edit to the brief",
    dispatch: "Crew",
    replan: "Replan",
    comment: "Review comment",
    plan: p.was ? "Edit to the plan" : "The plan",
    document: p.to ? `Edit to the ${kindName(kindTitle)}` : kindTitle,
    artifact: p.to ? `A version of ${p.to}` : "Artifact",
    plugin: p.label ?? "Proposal", // drawn by PluginProposalCard
  }[p.kind] ?? "Proposal"; // a kind ws no longer has, from before
  const icon: IconName = proposalIcons[p.kind] ?? "pencil";
  // A review comment: the pull request's page, at the lines it's on.
  const [prRepo, prNumber] = (p.pr ?? "").split("#");
  const prHref = p.pr ? `/reviews/${prRepo}/${prNumber}` : "";
  const lines = p.line ? (p.endLine && p.endLine > p.line ? `${p.line}–${p.endLine}` : `${p.line}`) : "";
  const accept = {
    decision: "Record decision",
    mission: (editing ? !briefEmpty : p.brief) ? "Create mission" : "Save as an idea",
    brief: briefing ? "Confirm and start" : p.plan ? "Confirm the goal and its plan" : "Accept the brief",
    dispatch: "Start crew",
    replan: "Ask the planner",
    comment: "Add to my review",
    plan: "Accept the plan",
    document: p.to ? "Save the edit" : `Add ${kindName(kindTitle)}`,
    artifact: p.to ? `Add as v${artifactNext(p, artifact)}` : "Add artifact",
    plugin: "Accept",
  }[p.kind];
  const ready = {
    decision: what.trim(),
    mission: title.trim(),
    brief: !briefEmpty,
    dispatch: text.trim(),
    replan: text.trim(),
    comment: text.trim() || p.suggestion != null,
    plan: !briefEmpty,
    document: !docEmpty,
    artifact: note.trim() && (p.to || title.trim()),
    plugin: true,
  }[p.kind];
  const outcome = () => {
    if (p.state === "declined")
      return (
        <span className="settled-line dim">
          <Icon name="x" size={13} /> Declined
        </span>
      );
    if (p.state === "superseded")
      return (
        <span className="settled-line dim">
          <Icon name="arrow-down" size={13} />
          <span>
            Superseded by <a href={`#proposal-${p.result}`}>proposal {p.result}</a>, proposed after it: nothing to confirm here
          </span>
        </span>
      );
    if (p.state === "withdrawn")
      return (
        <span className="settled-line dim">
          <Icon name="x" size={13} /> Withdrawn{p.because ? `: ${p.because}` : ""}
        </span>
      );
    const done = (children: React.ReactNode) => (
      <span className="settled-line done">
        <Icon name="check-circle" size={14} /> {children}
      </span>
    );
    switch (p.kind) {
      case "mission":
        return p.result ? (
          <>
            {done("Mission created")}
            <Link to={`/missions/${p.result}`} className={buttonClass()}>
              Open it <Icon name="arrow-right" size={13} />
            </Link>
          </>
        ) : (
          done("Saved as an idea")
        );
      case "brief":
        return done(briefing ? (p.plan ? "Briefed and planned: it's ready for crew" : "Briefed: it's ready for crew") : "The brief is updated");
      case "plan":
        return done("The plan is updated");
      case "dispatch":
        return done(p.result === "queued" ? "Queued: the mission's page launches it" : `Started ${p.result}`);
      case "replan":
        return (
          <>
            {done(`Asked the planner, in ${p.result}: it revises the plan, or says why not`)}
            {slug && p.result && (
              <Link to={talkHref(slug, p.result)} className={buttonClass()}>
                Open it <Icon name="arrow-right" size={13} />
              </Link>
            )}
          </>
        );
      case "comment":
        return (
          <>
            {done("In your draft review: it's posted when you submit")}
            {!inReview && (
              <Link to={prHref} className={buttonClass()}>
                Open the review <Icon name="arrow-right" size={13} />
              </Link>
            )}
          </>
        );
      case "document": {
        // Landed (repo:path), or left in a capsule (repo/capsule:path) to
        // open a pull request from. An edit to one in a capsule was saved
        // there, where it was.
        const capsule = p.result?.split(":")[0].includes("/") ? p.result.split(":")[0] : "";
        const leftIn = capsule && p.result !== p.to ? `in a capsule, ${capsule}, to open a pull request from` : "";
        return (
          <>
            {done(p.to ? (leftIn ? `Saved ${leftIn}` : "Saved") : leftIn ? `Added ${leftIn}` : "Added")}
            {p.result && (
              <Link to={docAddress(p.result)} className={buttonClass()}>
                Open it <Icon name="arrow-right" size={13} />
              </Link>
            )}
          </>
        );
      }
      case "artifact": {
        // Added, as a version: a3.v1, drawn as its card.
        const v = addedVersion(p.result);
        return v ? (
          <>
            {done(
              <span>
                Added as{" "}
                <RefTag refKey={refKey({ kind: "artifact", id: v.id, version: v.n })} inline>
                  {p.result}
                </RefTag>
              </span>,
            )}
            <Link to={`/artifacts/${v.id}/v${v.n}`} className={buttonClass()}>
              Open it <Icon name="arrow-right" size={13} />
            </Link>
          </>
        ) : (
          done("Added")
        );
      }
    }
    if (!p.result) return done("Decided: kept with this conversation until it becomes a mission");
    const [on, ref] = p.result.split(":");
    return done(
      <>
        Recorded as {ref} on <Link to={`/missions/${on}`} className="link">{on}</Link>
      </>,
    );
  };
  // The brief (a mission's, or briefing one) is shown whole: it's what
  // confirming commits you to, so it isn't folded away.
  const briefBody = (b: string) => (
    <div className="prose proposal-brief whole">
      <Markdown text={b} />
    </div>
  );
  const meta = (items: (string | false | undefined)[]) => {
    const list = items.filter(Boolean) as string[];
    return list.length ? (
      <div className="proposal-meta">
        {list.map((m) => (
          <Chip tone="quiet" key={m}>
            {m}
          </Chip>
        ))}
      </div>
    ) : null;
  };
  const body = () => {
    if (editing)
      switch (p.kind) {
        case "decision":
          return (
            <div className="proposal-edit">
              <TextArea value={what} rows={2} onChange={(e) => setWhat(e.target.value)} placeholder="The decision, in a sentence" aria-label="The decision" />
              <TextArea value={why} rows={2} onChange={(e) => setWhy(e.target.value)} placeholder="Why" aria-label="Why" />
            </div>
          );
        case "mission":
          return (
            <div className="proposal-edit">
              <Input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="Title" aria-label="Title" />
              <MarkdownEditor initial={p.brief ?? ""} label="The brief" placeholder="The brief (none: an idea, to brief later)" compact onChange={setBrief} onCancel={() => setEditing(false)} />
            </div>
          );
        case "brief":
          return (
            <div className="proposal-edit">
              <MarkdownEditor initial={p.brief ?? ""} label="The brief" compact autoFocus onChange={setBrief} onCancel={() => setEditing(false)} />
            </div>
          );
        case "plan":
          return (
            <div className="proposal-edit">
              <MarkdownEditor initial={p.plan ?? ""} label="The plan" compact autoFocus onChange={setBrief} onCancel={() => setEditing(false)} />
            </div>
          );
        case "dispatch":
          return (
            <div className="proposal-edit">
              <TextArea value={text} rows={Math.min(10, Math.max(3, text.split("\n").length + 1))} onChange={(e) => setText(e.target.value)} placeholder="What the crew's to do" aria-label="What the crew's to do" />
            </div>
          );
        case "replan":
          return (
            <div className="proposal-edit">
              <TextArea value={text} rows={Math.min(10, Math.max(3, text.split("\n").length + 1))} onChange={(e) => setText(e.target.value)} placeholder="What to change in the plan" aria-label="What to change in the plan" />
            </div>
          );
        case "comment":
          return (
            <div className="proposal-edit">
              <TextArea value={text} rows={Math.min(10, Math.max(3, text.split("\n").length + 1))} onChange={(e) => setText(e.target.value)} placeholder="What the comment says" aria-label="What the comment says" />
            </div>
          );
        case "document":
          return (
            <div className="proposal-edit">
              <MarkdownEditor initial={p.text ?? ""} label="The document" compact autoFocus onChange={setDoc} onCancel={() => setEditing(false)} />
              <div className="dim">
                {p.to
                  ? "The whole document, as it's saved: it stays where it is, whatever its first heading says."
                  : "Its first heading is its title: where it goes, and its commit's message, are made from it when it's added."}
              </div>
            </div>
          );
        case "artifact":
          return (
            <div className="proposal-edit">
              {!p.to && <Input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="Its title" aria-label="Its title" />}
              <TextArea
                value={note}
                rows={Math.min(6, Math.max(2, note.split("\n").length + 1))}
                onChange={(e) => setNote(e.target.value)}
                placeholder={p.to ? "What changed, and why" : "What it tries out"}
                aria-label="Its note"
              />
              <div className="dim">The HTML is the agent's: ask it for another version to change that.</div>
            </div>
          );
      }
    switch (p.kind) {
      case "decision":
        return (
          <>
            <div className="proposal-what">{p.what}</div>
            {p.why && <div className="proposal-why">{p.why}</div>}
          </>
        );
      case "mission":
        return (
          <>
            {p.brief ? briefBody(p.brief) : <div className="dim">An idea, to brief later: a briefing agent starts from this conversation.</div>}
            {meta([p.about?.length ? `About ${p.about.join(", ")}` : undefined, p.capsules?.length ? `Lifts ${p.capsules.join(", ")}` : undefined])}
          </>
        );
      case "brief":
        if (briefing)
          return (
            <>
              {briefBody(p.brief ?? "")}
              {p.plan ? (
                <details className="fold proposal-brief" open>
                  <summary>The plan, confirmed with it</summary>
                  <div className="prose">
                    <Markdown text={p.plan} />
                  </div>
                </details>
              ) : (
                <div className="dim">No plan: crew work from the brief.</div>
              )}
              {meta([p.capsules?.length ? `Lifts ${p.capsules.join(", ")}` : undefined, p.docks?.length ? `Builds on ${p.docks.join(", ")}` : undefined, p.slug && `Renamed ${p.slug}`])}
            </>
          );
        return <BriefDiff was={p.was ?? ""} now={p.brief ?? ""} />;
      case "plan":
        return (
          <>
            {p.why && <div className="proposal-why">{p.why}</div>}
            {p.was ? (
              <BriefDiff was={p.was} now={p.plan ?? ""} />
            ) : (
              <div className="prose">
                <Markdown text={p.plan ?? ""} />
              </div>
            )}
            {p.step ? meta([`Written at step ${p.step}, the planner's: accepting it passes that step`]) : null}
          </>
        );
      case "dispatch":
        return (
          <>
            <div className="proposal-what said-text">{p.text}</div>
            {meta([p.capsule ? `In ${p.capsule}` : "In a capsule no crew is in", p.reads?.length ? `Reading ${p.reads.join(", ")}` : undefined])}
          </>
        );
      case "replan":
        return (
          <>
            <div className="proposal-what said-text">{p.text}</div>
            {p.state === "open" && <div className="dim">Confirming asks the mission's planner this, as you: it revises the plan in its own conversation, or says why not.</div>}
          </>
        );
      case "comment":
        return (
          <>
            <div className="proposal-where">
              <span className={`kind k-${p.noteKind}`}>{p.noteKind}</span>
              <Link to={prHref} className="link mono">
                {p.pr}
              </Link>
              <span className="mono dim">
                {p.path}:{lines}
                {p.side === "old" ? " (removed)" : ""}
              </span>
            </div>
            {!!p.excerpt?.length && (
              <Excerpt lines={p.excerpt} from={p.line ?? 1} />
            )}
            {p.text && <div className="proposal-what said-text">{p.text}</div>}
            {p.suggestion != null && <Suggestion from={p.excerpt} to={p.suggestion} />}
          </>
        );
      case "document": {
        // An edit to one: what it changes, from the document as the agent
        // read it (saving merges in what's changed there since).
        if (p.to)
          return (
            <>
              <BriefDiff was={p.was ?? ""} now={p.text ?? ""} />
              {meta([`Changes ${p.to}`, p.subject && `Committed as “${p.subject}”`, p.guide && `Written as ${p.guide} says`])}
            </>
          );
        // Shown whole: adding it commits it, where its kind says.
        return (
          <>
            <div className="prose proposal-brief whole">
              <Markdown text={p.text ?? ""} />
            </div>
            {meta([p.path && `Adds ${p.path}`, p.subject && `Committed as “${p.subject}”`, p.guide && `Written as ${p.guide} says`])}
          </>
        );
      }
      case "artifact":
        return <ArtifactProposalBody p={p} src={frameOf?.(p.n) ?? ""} artifact={artifact} />;
    }
  };
  return (
    <div className={`proposal k-${p.kind} s-${p.state}`} id={anchor}>
      <div className="proposal-head">
        <Icon name={icon} size={14} className="proposal-icon" />
        <span className="proposal-kind">{label}</span>
        {(p.kind === "mission" || p.kind === "document" || briefing) && !editing && p.title && <span className="proposal-title">{p.title}</span>}
        {p.kind === "artifact" && !editing && (p.title || artifact) && <span className="proposal-title">{p.title || artifact?.title}</span>}
        <span className="spacer" />
        {p.revises && (
          <a className="proposal-revises dim" href={`#proposal-${p.revises}`} title={`It takes the place of proposal ${p.revises}`}>
            Revises {p.revises}
          </a>
        )}
        {p.state === "open" && !editing && (
          <Button variant="ghost" size="small" onClick={() => setEditing(true)}>
            <Icon name="pencil" size={12} /> Edit
          </Button>
        )}
      </div>
      {revisedFrom && p.state === "open" && <RevisionChanges was={revisedFrom} now={p} />}
      <div className="proposal-body">{body()}</div>
      {p.state === "open" ? (
        <div className="proposal-actions">
          {busy && (
            <span className="dim busy">
              <Spinner />{" "}
              {{ decision: "Recording…", mission: "Creating…", brief: "Writing…", dispatch: "Starting…", replan: "Asking…", comment: "Adding…", plan: "Writing…", document: p.to ? "Saving…" : "Adding…", artifact: "Adding…", plugin: "Doing it…" }[p.kind]}
            </span>
          )}
          <span className="spacer" />
          <Button variant="ghost" disabled={busy} onClick={() => run("decline")}>
            Decline
          </Button>
          <Button variant="primary" disabled={busy || !ready} unavailable={replying ? "Read its reply first: the agent's still writing it" : undefined} onClick={() => run("accept")}>
            {accept}
          </Button>
        </div>
      ) : (
        <div className="proposal-actions settled">{outcome()}</div>
      )}
    </div>
  );
}

// RevisionChanges is what a revision changed of the proposal it revises,
// folded: the words each is mostly made of, as they read and as they do.
function RevisionChanges({ was, now }: { was: TalkProposal; now: TalkProposal }) {
  const a = revisedText(was);
  const b = revisedText(now);
  if (!a && !b) return null;
  return (
    <details className="proposal-changes">
      <summary className="dim">What changed since proposal {was.n}</summary>
      {a === b ? <div className="dim">Its words are as they were.</div> : <BriefDiff className="proposal-changed" was={a} now={b} />}
    </details>
  );
}

// BriefDiff is what an edit to the brief changes: each changed passage,
// as it read and as it would.
function BriefDiff({ was, now, className }: { was: string; now: string; className?: string }) {
  const lines = now.replace(/\r\n/g, "\n").replace(/\n$/, "").split("\n");
  const hunks = changes(was, now);
  if (!hunks.length) return <div className="dim">No change.</div>;
  return (
    <Change className={className} wrap struck hunks={hunks.map((h) => ({ was: h.old, now: lines.slice(h.newStart - 1, h.newEnd) }))} />
  );
}
