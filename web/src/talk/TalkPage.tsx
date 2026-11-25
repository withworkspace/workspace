import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import { Link, useNavigate, useParams } from "react-router";
import { ApiError, proposalFrameURL, setTalkStatus, type TalkDetail, type TalkProposal, type TalkTurn } from "../api";
import { talkAt } from "../chrome/panes";
import { useTalk, useTalks } from "../conversation/talks";
import { useShortcut, useKeys } from "../keys";
import { useHere } from "../load";
import { Markdown } from "../markdown/LazyMarkdown";
import { usePref } from "../prefs";
import { PagePin } from "../shell/PagePin";
import { Button, buttonClass } from "../ui/Button";
import { Callout } from "../ui/Callout";
import { Composer } from "../ui/Composer";
import { Doing } from "../ui/Doing";
import { ErrorCard } from "../ui/ErrorCard";
import { Icon } from "../ui/Icon";
import { pageScroller } from "../ui/scroller";
import { sendKeys, useSendKey } from "../ui/sendKey";
import { Skeleton } from "../ui/Skeleton";
import { Status } from "../ui/Status";
import { humanError } from "../ui/text";
import { toast } from "../ui/toast";
import { ToggleButton } from "../ui/ToggleButton";
import { Turn, TurnSaid } from "../ui/Turn";
import { useDraft } from "../ui/useDraft";
import { useMedia } from "../ui/useMedia";
import { useTitle } from "../ui/useTitle";
import { useWorkspace } from "../workspace/store";
import { ProposalCard } from "./ProposalCard";
import { afterSettling, settled } from "./proposals";
import { SideOf } from "./SideOf";
import { TalkAbout } from "./TalkAbout";
import { TalkAnchor } from "./TalkAnchor";
import { TalkBar } from "./TalkBar";
import { TalkLayout } from "./TalkLayout";
import { TalkMain } from "./TalkMain";
import { TalkReply } from "./TalkReply";
import { TalkSummary } from "./TalkSummary";
import { TalkTurnGroup } from "./TalkTurnGroup";
import { TalkTurns } from "./TalkTurns";
import { TurnView } from "./TurnView";
import { Who } from "./Who";
import { docAt, talkName, talkStatus } from "./words";
import { TalkModelSwitch } from "./TalkModelSwitch";

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

// TalkPage is a conversation: what it knows (the conversation, followed
// as it goes on; where the page rests; the reply being written; the
// column beside it, shown or not) and what it does (send, settle a card,
// resolve or reopen, keep an ask). It's the workspace's, or a mission's.
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
  // It's the window's (conversation/talks.ts), as the pane beside a page
  // has it: what's said in one shows in the other. Shown here, it's
  // followed: told each turn, and its agent's steps as it answers.
  const to = talkAt(id, slug);
  const held = useTalk(to);
  const d = held?.detail;
  const error = held?.gone || !d ? held?.error : undefined;
  const setD = (x: TalkDetail) => useTalks.getState().told(to, x);
  const load = useCallback(() => useTalks.getState().load(to), [to]);
  useEffect(() => useTalks.getState().watch(to), [to]);
  useEffect(() => {
    void load();
  }, [load]);
  const title = d ? talkName({ title: d.title, opening: d.turns[0]?.body }) : "";
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
    Escape: () => navigate(back),
  });
  // Resolving it, or reopening it: x, and the button in the bar.
  const resolve = useShortcut("x", toggle);
  const sendKey = useSendKey();
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
  const anchor = d?.anchor;
  const card = (p: TalkProposal, replying?: boolean) => (
    <ProposalCard key={p.n} p={p} revisedFrom={revisedFrom(p)} kinds={workspace.kinds} artifacts={workspace.artifacts} frameOf={frameOf} slug={slug} onSettle={settle} replying={replying} anchor={`proposal-${p.n}`} />
  );
  return (
    <TalkLayout side={side} ref={page}>
      <TalkBar
        ref={head}
        back={{ to: back, title: slug ? `Back to ${mission?.title ?? slug}` : "Back to the conversations", keys: "esc" }}
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
              <Button onClick={resolve} title={d.status === "resolved" ? "Reopen" : "Resolve"}>
                <Icon name={d.status === "resolved" ? "rotate" : "check"} size={13} />
                {d.status === "resolved" ? "Reopen" : "Resolve"}
              </Button>
            )}
          </>
        }
      >
        {!slug && <PagePin r={{ kind: "talk", id }} />}
        <TalkAbout
          mission={slug ? { href: `/missions/${slug}`, title: mission?.title ?? slug } : undefined}
          pr={anchor?.pr ? { href: `/reviews/${anchor.pr.replace("#", "/")}`, pr: anchor.pr, path: anchor.path, line: anchor.line, endLine: anchor.endLine } : undefined}
          doc={anchor?.doc ? { href: docAt(anchor.doc, anchor.section), path: anchor.path ?? anchor.doc, section: anchor.section } : undefined}
          at={d?.at}
        />
      </TalkBar>
      <TalkMain>
        {error && <ErrorCard error={error} retry={load} />}
        {d?.summary && !side && <TalkSummary text={d.summary} />}
        {!d && !error && <Skeleton rows={6} />}
        {d?.via && <p className="dim">Started from the {d.via} plugin, which worded the first message.</p>}
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
        <TalkAnchor
          excerpt={anchor?.excerpt?.length ? { lines: anchor.excerpt, from: anchor.line ?? 1, title: `${anchor.path}, the lines the question is about` } : undefined}
          quote={anchor?.quote?.exact}
        />
        <TalkTurns>
          {d?.turns.map((t) => (
            <TalkTurnGroup key={t.n} settled={!!settled(t, proposals)}>
              <TurnView t={t} root={workspace.root} proposals={proposals} said={t.n === latestReply ? reply : undefined} landed={landed.current.turns.has(t.n)} />
              {t.by !== "you" && under(t).map((p) => card(p))}
            </TalkTurnGroup>
          ))}
          {running && (
            <Turn by="agent" running aria-live="polite" head={<Who by="agent" />}>
              {d.run?.said && (
                <TurnSaid className="dim" ref={reply}>
                  <Markdown text={d.run.said} />
                </TurnSaid>
              )}
              {pending.map((p) => card(p, true))}
              <Doing calls={d.run?.calls ?? []} root={workspace.root} />
            </Turn>
          )}
        </TalkTurns>
        <TalkReply
          onNewReply={
            unseen
              ? () => {
                  pageScroller()?.scrollTo({ top: rest(), behavior: "smooth" });
                  setUnseen(false);
                }
              : undefined
          }
        >
          <Composer
            field={box}
            value={text}
            onChange={setText}
            onSend={send}
            aria-label="Reply"
            placeholder={d?.answering ? "Say more: it's sent once the agent's done" : d?.status === "resolved" ? "Reply to reopen it…" : "Reply…"}
          >
            {d && <TalkModelSwitch slug={slug} id={id} chosen={d.model?.model} />}
            <Button variant="primary" disabled={!text.trim() || sending} onClick={send} keys={sendKeys(sendKey)}>
              {sending ? "Sending…" : "Send"}
            </Button>
          </Composer>
        </TalkReply>
      </TalkMain>
      {side && <SideOf d={d} slug={slug} id={id} artifacts={workspace.artifacts} goTo={goTo} />}
    </TalkLayout>
  );
}
