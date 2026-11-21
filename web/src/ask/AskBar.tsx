import { type KeyboardEvent, type MouseEvent, type Ref, useEffect, useImperativeHandle, useLayoutEffect, useRef, useState } from "react";
import { Doing } from "../ui/Doing";
import { Link } from "react-router";
import { ApiError, getTalk, keepAsk, sayInTalk, settleTalkProposal, type TalkDetail, type TalkProposal, type TalkTurn } from "../api";
import { Markdown } from "../markdown/LazyMarkdown";
import { ProposalCard, SettledMarker } from "../talk/Talk";
import { settled } from "../talk/proposals";
import { Calls } from "../ui/Calls";
import { Icon } from "../ui/Icon";
import { humanError, Keys, toast } from "../ui/kit";
import { type AskAbout, anchorChip, type Chip, inCard, placed, waiting } from "./ask";
import { Button, buttonClass } from "../ui/Button";

// The ask bar (#123): always there at the foot of what you're reading,
// with / to go to it. Ask about the whole of it, or pick something out of
// it first (a passage of a document, with the selection's toolbar,
// Selection.tsx, or a; lines of a pull request's diff, with a). The answer
// is a card over the page: it shows the agent's steps as it reads, and its
// citations of a document's sections scroll there. esc drops it; Ask a
// follow-up asks again in the same conversation; Keep as a conversation
// keeps it. A review comment its agent proposes is in the card, to add to
// your review there.
//
// It's a component of its own, not the reader's: a page gives it what's
// asked about (how an ask of it starts, and what's picked out of it, as
// its chip shows it) and the suggestions to offer, and it does the rest.
//
// Or it talks in a conversation there is (in): a mission's, carried on turn
// by turn, as the brief's own page talks in its planner's. Each ask is said
// in it (about.start says it), and the card shows what's been said since.
// An ask is a conversation from the start (POST /api/asks for a document,
// POST /api/prs/<repo>/<n>/ask for a pull request), but nobody's kept it:
// it isn't listed, and the daemon sweeps it an hour after its last turn.
// Kept (.../keep), it's in Conversations, about what was asked about.

// AskBarHandle is what a page does with its bar: go to it (/; whole drops
// what's in hand, to ask about all of it), ask with something in hand (a),
// and a question at once (Explain), or drop the card (esc: it says whether
// there was one).
export interface AskBarHandle<H> {
  focus: (whole?: boolean) => void;
  ask: (held?: H, question?: string) => void;
  close: () => boolean;
}

// How often an ask's followed while it's answered.
const followEvery = 1000;

// TalkIn is a conversation the bar talks in, rather than starting asks of
// its own: its mission's slug and its id, as the page has it (the card
// opens on it while it's answered), and where it's read in full. draws is
// which of what it proposes the card draws: the page draws the rest.
// onTalk hears each time the bar reads it again.
export interface TalkIn {
  slug: string;
  id: string;
  detail?: TalkDetail;
  href: string;
  label: string; // what the card's foot calls it
  draws?: (p: TalkProposal) => boolean;
  onTalk?: (d: TalkDetail) => void;
}

// lastAsked is where a conversation's latest ask starts: its last turn of
// yours (0 for none).
const lastAsked = (t: TalkDetail) => {
  for (let i = t.turns.length - 1; i >= 0; i--) if (t.turns[i].by === "you") return i;
  return 0;
};

export function AskBar<H>({
  about,
  suggestions,
  root,
  cite,
  onOpen,
  onSettled,
  in: conv,
  ref,
}: {
  about: AskAbout<H>;
  // suggestions are what to offer asking, about what's in hand, or the whole.
  suggestions: (held?: H) => string[];
  root?: string; // the workspace's, which the steps' paths are shortened by
  // cite goes to what an answer cites (#how-it-lands): false if it can't.
  cite?: (href: string) => boolean;
  onOpen?: () => void; // the card's opened: what else is open can close
  onSettled?: (p: TalkProposal) => void; // a proposal in the card's been accepted or declined
  in?: TalkIn;
  ref?: Ref<AskBarHandle<H>>;
}) {
  const [text, setText] = useState("");
  const [held, setHeld] = useState<H>();
  const [talk, setTalk] = useState<TalkDetail>();
  // from is the turn the card's ask starts at: an ask's first, or, in a
  // conversation there is, the one you said last.
  const [from, setFrom] = useState(0);
  const scope = conv?.slug ?? "";
  const [sending, setSending] = useState(false);
  const [keeping, setKeeping] = useState(false);
  const [focused, setFocused] = useState(false);
  const [pick, setPick] = useState(-1);
  const input = useRef<HTMLInputElement>(null);
  const body = useRef<HTMLDivElement>(null);
  const open = !!talk;
  const offered = focused && !text && !open && !sending ? suggestions(held) : [];

  // Follow an ask while it's answered: its steps as the agent takes them,
  // then its answer. One gone (swept, an hour on) is said to be.
  const id = talk?.id;
  const following = waiting(talk);
  useEffect(() => {
    if (!id || !following) return;
    let stale = false;
    const t = setTimeout(
      () =>
        getTalk(scope, id).then(
          (d) => {
            if (stale) return;
            setTalk(d);
            conv?.onTalk?.(d);
          },
          (e: Error) => {
            if (stale) return;
            if (e instanceof ApiError && e.status === 404) gone();
            else setTalk((was) => (was ? { ...was } : was)); // try again
          },
        ),
      followEvery,
    );
    return () => {
      stale = true;
      clearTimeout(t);
    };
  }, [id, following, talk]);

  // In a conversation there is, the page's reading of it is the card's too;
  // and one being answered when the page opens opens the card on it.
  const given = conv?.detail;
  useEffect(() => {
    if (!given) return;
    if (talk?.id === given.id) setTalk(given);
    else if (!talk && waiting(given)) {
      setFrom(lastAsked(given));
      setTalk(given);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [given]);

  // The card keeps to its latest, unless you've scrolled up in it.
  const near = useRef(true);
  useLayoutEffect(() => {
    const el = body.current;
    if (el && near.current) el.scrollTop = el.scrollHeight;
  }, [talk]);

  const close = () => {
    if (!talk) return false;
    setTalk(undefined);
    return true;
  };

  // gone is an ask that's been swept: said, and the card dropped.
  const gone = () => {
    setTalk(undefined);
    toast("That ask's gone: nobody kept it.");
  };

  // keep keeps the card's ask: a conversation in Conversations from then on.
  const keep = () => {
    if (!talk || keeping) return;
    setKeeping(true);
    keepAsk(scope, talk.id)
      .then(setTalk, (e: Error) => (e instanceof ApiError && e.status === 404 ? gone() : toast(humanError(e.message), { tone: "danger" })))
      .finally(() => setKeeping(false));
  };

  // settle accepts (with your edits) or declines a proposal in the card.
  const settle = (n: number, action: "accept" | "decline", edits?: object) => {
    if (!talk) return Promise.resolve();
    return settleTalkProposal(scope, talk.id, n, action, edits).then(
      (d) => {
        setTalk(d);
        conv?.onTalk?.(d);
        if (d.message) toast(d.message, { tone: "done" });
        const p = d.proposals?.find((x) => x.n === n);
        if (p) onSettled?.(p);
      },
      (e: Error) => (e instanceof ApiError && e.status === 404 ? gone() : toast(humanError(e.message), { tone: "danger" })),
    );
  };

  // send asks: in the card's conversation, a follow-up; with something in
  // hand, or nothing open, a new ask. A follow-up to one that's gone asks
  // afresh.
  const send = (question: string, h = held) => {
    const q = question.trim();
    if (!q || sending) return;
    const fresh = () => about.start(q, h);
    // In a conversation there is, every ask is said in it.
    const asked = conv ? fresh() : talk && h === undefined ? sayInTalk("", talk.id, q).catch((e: Error) => (e instanceof ApiError && e.status === 404 ? fresh() : Promise.reject(e))) : fresh();
    setSending(true);
    asked
      .then(
        (d) => {
          near.current = true;
          // An ask's card starts at its question; a follow-up in its
          // conversation carries on under it, and in a conversation there
          // is, it starts at what you've just said.
          if (conv) setFrom(lastAsked(d));
          else if (!talk || h !== undefined || talk.id !== d.id) setFrom(0);
          setTalk(d);
          conv?.onTalk?.(d);
          setText((t) => (t.trim() === q ? "" : t)); // not what's been typed since
          setHeld(undefined);
          setPick(-1);
          onOpen?.();
        },
        (e: Error) => toast(humanError(e.message), { tone: "danger" }),
      )
      .finally(() => setSending(false));
  };

  useImperativeHandle(ref, () => ({
    focus: (whole) => {
      if (whole) setHeld(undefined);
      input.current?.focus();
    },
    ask: (picked, question) => {
      if (picked !== undefined) setHeld(picked);
      if (question) send(question, picked);
      else input.current?.focus();
    },
    close,
  }));

  const keys = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.nativeEvent.isComposing) return;
    if (e.key === "Enter") {
      e.preventDefault();
      send(!text && offered[pick] ? offered[pick] : text);
    } else if (e.key === "Escape") {
      e.preventDefault();
      if (pick >= 0) return setPick(-1);
      e.currentTarget.blur();
      close();
    } else if (e.key === "ArrowUp" && offered.length) {
      // The suggestions stand above the bar, the first nearest it.
      e.preventDefault();
      setPick((p) => Math.min(offered.length - 1, p + 1));
    } else if (e.key === "ArrowDown" && offered.length) {
      e.preventDefault();
      setPick((p) => Math.max(-1, p - 1));
    } else if (e.key === "Backspace" && !text && held !== undefined) {
      setHeld(undefined);
    }
  };

  // A citation of one of the document's sections goes there.
  const citeClick = (e: MouseEvent) => {
    const a = (e.target as Element).closest?.('a[href^="#"]');
    if (!a || !cite || e.metaKey || e.ctrlKey || e.shiftKey) return;
    e.preventDefault();
    e.stopPropagation();
    if (!cite(a.getAttribute("href") ?? "")) toast("It cites a section the document doesn't have.");
  };

  const asked = anchorChip(talk?.anchor);
  const hand = held !== undefined ? about.chip(held) : undefined;
  // What it proposes that the card draws (review comments), under the
  // reply each came with. Anything else, once it's kept, Conversations
  // draws.
  const draws = conv?.draws ?? inCard;
  const shown = talk ? placed(talk, draws) : undefined;
  const proposed = !!talk?.proposals?.some((p) => p.state === "open" && !draws(p));
  const proposals = (list?: TalkProposal[]) => list?.map((p) => <ProposalCard key={p.n} p={p} inReview onSettle={settle} />);
  const placeholder = hand ? hand.placeholder : open ? "Ask a follow-up…" : `Ask about ${about.title}…`;
  return (
    <div className="askbar-wrap">
      {open && talk && (
        <div className="askcard" role="dialog" aria-label="The answer">
          <div className="askcard-head">
            <Icon name="sparkle" size={13} className="askcard-icon" />
            <span className="askcard-q" title={talk.turns[from]?.body}>
              {talk.turns[from]?.body}
            </span>
            {asked && <ChipView c={asked} />}
            <Button variant="ghost" square type="button" className="askcard-x" onClick={close} title="Drop it (esc)" aria-label="Drop it">
              <Icon name="x" size={13} />
            </Button>
          </div>
          <div
            className="askcard-body"
            ref={body}
            aria-live="polite"
            onClickCapture={citeClick}
            onScroll={(e) => {
              const el = e.currentTarget;
              near.current = el.scrollHeight - el.scrollTop - el.clientHeight < 40;
            }}
          >
            {talk.turns.slice(from + 1).map((t) => (
              <AskTurn key={t.n} t={t} root={root} proposals={talk.proposals}>
                {proposals(shown?.under.get(t.n))}
              </AskTurn>
            ))}
            {following && (
              <div className="ask-live">
                {talk.run?.said && (
                  <div className="ask-said dim">
                    <Markdown text={talk.run.said} />
                  </div>
                )}
                <Doing calls={talk.run?.calls ?? []} root={root} />
              </div>
            )}
            {proposals(shown?.after)}
          </div>
          <div className="askcard-foot">
            <span>{conv ? conv.label : talk.ephemeral ? "Gone when you close it, unless you keep it." : proposed ? "Kept: what it proposed is in Conversations, to confirm." : "Kept: it's in Conversations."}</span>
            <span className="spacer" />
            <Button size="small" type="button" onClick={() => input.current?.focus()} title="Ask again, in this conversation">
              Ask a follow-up
            </Button>
            {conv ? (
              <Link to={conv.href} className={buttonClass({ size: "small" }, "askcard-link")} title="Read the whole conversation">
                <Icon name="message" size={12} /> The whole conversation
              </Link>
            ) : talk.ephemeral ? (
              <Button size="small" type="button" onClick={keep} disabled={keeping} aria-busy={keeping} title={`Keep it as a conversation, in Conversations, about ${about.title}`}>
                <Icon name="message" size={12} /> Keep as a conversation
              </Button>
            ) : (
              <Link to={`/conversations/${encodeURIComponent(talk.id)}`} className={buttonClass({ size: "small" })} title="Open it in Conversations">
                <Icon name="message" size={12} /> Open it
              </Link>
            )}
          </div>
        </div>
      )}
      {offered.length > 0 && (
        <div className="ask-suggest" role="listbox" aria-label="Suggestions">
          {offered.map((s, i) => (
            <button
              type="button"
              key={s}
              role="option"
              aria-selected={i === pick}
              className={`ask-suggestion${i === pick ? " sel" : ""}`}
              onMouseDown={(e) => e.preventDefault()}
              onMouseMove={() => i !== pick && setPick(i)}
              onClick={() => send(s)}
            >
              <Icon name="sparkle" size={11} />
              {s}
            </button>
          ))}
        </div>
      )}
      <div className={`askbar${sending ? " sending" : ""}`}>
        <Icon name={sending ? "spinner" : "sparkle"} size={14} className={`askbar-icon${sending ? " spin" : ""}`} />
        {hand && (
          <ChipView c={hand}>
            <Button type="button" onClick={() => setHeld(undefined)} title={hand.drop} aria-label={hand.drop}>
              <Icon name="x" size={11} />
            </Button>
          </ChipView>
        )}
        <input
          ref={input}
          value={text}
          placeholder={placeholder}
          aria-label={placeholder}
          aria-busy={sending}
          onChange={(e) => {
            setText(e.target.value);
            setPick(-1);
          }}
          onKeyDown={keys}
          onFocus={() => setFocused(true)}
          onBlur={() => {
            setFocused(false);
            setPick(-1);
          }}
        />
        <Keys k="/" className="askbar-key" />
      </div>
    </div>
  );
}

// ChipView is what's asked about as a chip: in the bar, what's in hand
// (with an x to drop it); in the card's head, what the ask was of.
function ChipView({ c, children }: { c: Chip; children?: React.ReactNode }) {
  return (
    <span className={`ask-ctx ${c.kind}`} title={c.title}>
      <Icon name={c.icon} size={11} />
      <span className="q">{c.text}</span>
      {children}
    </span>
  );
}

// AskTurn is a turn of an ask after its question: a follow-up of yours,
// the agent's answer (its steps folded above it, and what it proposed
// with it below), ws saying it couldn't, or the marker of a card you
// settled.
function AskTurn({ t, root, proposals, children }: { t: TalkTurn; root?: string; proposals?: TalkProposal[]; children?: React.ReactNode }) {
  const m = settled(t, proposals);
  if (m) return <SettledMarker m={m} t={t} compact />;
  if (t.by === "you") {
    return (
      <div className="ask-you">
        <Icon name="reply" size={12} />
        <span>{t.body}</span>
      </div>
    );
  }
  if (t.by !== "agent") return <p className="ask-ws">{t.body}</p>;
  return (
    <div className="ask-turn">
      {t.calls && t.calls.length > 0 && <Calls calls={t.calls} root={root} />}
      <div className="ask-answer">
        <Markdown text={t.body} pinned={t.refs} />
      </div>
      {children}
    </div>
  );
}
