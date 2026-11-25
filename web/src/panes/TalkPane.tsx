import { type MouseEvent, useEffect, useLayoutEffect, useRef, useState } from "react";
import { Link } from "react-router";
import { ApiError, type TalkDetail, type TalkProposal } from "../api";
import { anchorChip, inCard, placed, waiting } from "../ask/ask";
import { DockBody } from "../chrome/DockBody";
import { PaneHead } from "../chrome/PaneHead";
import { useChromeStore, useGives, useKeysAsked } from "../chrome/store";
import { ConvoTurn } from "../conversation/ConvoTurn";
import { useTalk, useTalks } from "../conversation/talks";
import { Markdown } from "../markdown/LazyMarkdown";
import { ProposalCard } from "../talk/ProposalCard";
import { Button } from "../ui/Button";
import { Doing } from "../ui/Doing";
import { Icon } from "../ui/Icon";
import { toast } from "../ui/toast";
import { Loading } from "../ui/Loading";
import { humanError } from "../ui/text";
import { ErrorCard } from "../ui/ErrorCard";
import { useScroller } from "../ui/scroller";
import { Composer } from "./Composer";
import { Turn, TurnSaid } from "../ui/Turn";
import css from "./TalkPane.module.css";

type TalkPaneProps = {
  // Its tab's address, which is its own page's.
  to: string;
  // The workspace's root, which its steps' paths are shortened by.
  root?: string;
  // When it last changed, as the workspace's list says: read again when
  // that moves.
  stamp?: string;
  // It's gone (an ask nobody kept, swept an hour on): its tab closes.
  onGone: () => void;
};

// TalkPane is a conversation, beside the page: the one place a
// conversation is held outside its own page. An ask from a page's bar
// opens here. It's read, followed while its agent answers, and replied to
// here; a review comment it proposes is confirmed here, and anything
// larger on its own page (Open).
//
// The conversation is the window's (conversation/talks.ts), not this
// pane's: the bar that asked reads the same one, and it's there as it was
// when its tab comes back to the front.
//
// One asked from a bar is nobody's until it's kept: it's gone an hour
// after its last turn. Keep makes it a conversation in Conversations.
// While the page it was asked on is open, that page gives it what only
// the page has (chrome/store.ts): what you're holding there, and where a
// citation leads.
export function TalkPane({ to, root, stamp, onGone }: TalkPaneProps) {
  const gives = useGives(to);
  const held = useTalk(to);
  const talk = held?.detail;
  const [keeping, setKeeping] = useState(false);
  const field = useRef<HTMLTextAreaElement>(null);

  // It's shown, so it's followed while it's answered; and read, unless
  // whoever opened it has just said what it is. Read again when the
  // workspace says it's changed.
  useEffect(() => useTalks.getState().watch(to), [to]);
  const first = useRef(true);
  useEffect(() => {
    const fresh = first.current && !!useTalks.getState().talks[to]?.detail;
    first.current = false;
    if (!fresh) void useTalks.getState().load(to);
  }, [to, stamp]);

  // One that's gone: an ask says so, a tab left from another day just
  // closes.
  const gone = !!held?.gone;
  useEffect(() => {
    if (!gone) return;
    if (talk?.ephemeral) toast("That ask's gone: nobody kept it.");
    useTalks.getState().forget(to);
    onGone();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [gone]);

  // Asked to take the keys (something was just asked from the page's
  // bar): the reply has them, so what's said next carries on here.
  const asked = useKeysAsked(to);
  const ready = talk?.status === "open";
  useEffect(() => {
    if (asked && ready && useChromeStore.getState().takeKeys(to)) field.current?.focus();
  }, [asked, ready, to]);

  // What couldn't be done is said; one that's gone says so itself (above).
  const said = (e: Error) => {
    if (!(e instanceof ApiError && e.status === 404)) toast(humanError(e.message), { tone: "danger" });
  };

  const keep = () => {
    if (keeping) return;
    setKeeping(true);
    useTalks
      .getState()
      .keep(to)
      // The button that had the keys is gone: the reply has them.
      .then(() => field.current?.focus(), said)
      .finally(() => setKeeping(false));
  };

  // settle accepts (with your edits) or declines something it proposed.
  const settle = (n: number, action: "accept" | "decline", edits?: object) =>
    useTalks
      .getState()
      .settle(to, n, action, edits)
      .then((d) => {
        if (d.message) toast(d.message, { tone: "done" });
        const p = d.proposals?.find((x) => x.n === n);
        if (p) gives?.onSettled?.(p);
      }, said);

  const say = (q: string) =>
    useTalks
      .getState()
      .say(to, q, gives?.given?.())
      .catch((e: Error) => {
        said(e);
        throw e;
      });

  // A citation of a section of what was asked about goes there, on the
  // page, while that's the page that's open.
  const cite = (e: MouseEvent) => {
    const a = (e.target as Element).closest?.('a[href^="#"]');
    if (!a || e.metaKey || e.ctrlKey || e.shiftKey) return;
    e.preventDefault();
    e.stopPropagation();
    const go = gives?.cite;
    if (!go) toast("It cites a section of what it was asked about: open that to go there.");
    else if (!go(a.getAttribute("href") ?? "")) toast("It cites a section the document doesn't have.");
  };

  const following = waiting(talk);
  const of = anchorChip(talk?.anchor);
  // What it proposed that isn't confirmed here: on its own page.
  const elsewhere = (talk?.proposals ?? []).filter((p) => p.state === "open" && !inCard(p)).length;
  return (
    <>
      <DockBody label={talk?.title || "Conversation"} pad>
        <PaneHead
          to={talk && !talk.ephemeral ? to : undefined}
          more={
            talk?.ephemeral && (
              <Button size="small" onClick={keep} disabled={keeping} aria-busy={keeping} title="Keep it as a conversation, in Conversations">
                <Icon name="message" size={12} /> Keep
              </Button>
            )
          }
          sub={
            talk && (
              <>
                <span>{talk.status === "resolved" ? "Put away" : following ? "Answering" : "Your turn"}</span>
                {of && (
                  <span className={`ask-ctx ${of.kind}`} title={of.title}>
                    <Icon name={of.icon} size={11} />
                    <span className="q">{of.text}</span>
                  </span>
                )}
                {talk.ephemeral && <span className={css.unkept}>Gone an hour after its last turn, unless you keep it</span>}
              </>
            )
          }
        />
        {held?.error && !talk && <ErrorCard title="Couldn't read the conversation" error={held.error} retry={() => void useTalks.getState().load(to)} />}
        {!talk && !held?.error && <Loading what="Reading the conversation…" />}
        {talk && (
          <div onClickCapture={cite}>
            <Turns talk={talk} root={root} following={following} settle={settle} />
          </div>
        )}
        {talk && elsewhere > 0 && (
          <p className={css.proposed}>
            It proposed {elsewhere === 1 ? "something" : `${elsewhere} things`} for you to confirm. <Link to={to}>Open it</Link> to see {elsewhere === 1 ? "it" : "them"}.
          </p>
        )}
      </DockBody>
      {ready && <Composer field={field} placeholder={following ? "Say something while it answers…" : talk.turns.length > 1 ? "Ask a follow-up…" : "Reply…"} holding={gives?.holding} onSay={say} />}
    </>
  );
}

// Turns is what's been said, the newest last and in sight unless you've
// scrolled up from it. A review comment it proposed is under the reply it
// came with, to add to your review from here.
function Turns({ talk, root, following, settle }: { talk: TalkDetail; root?: string; following: boolean; settle: (n: number, action: "accept" | "decline", edits?: object) => Promise<void> }) {
  const scroller = useScroller();
  const near = useRef(true);
  useEffect(() => {
    const el = scroller();
    if (!el) return;
    const onScroll = () => (near.current = el.scrollHeight - el.scrollTop - el.clientHeight < 120);
    el.addEventListener("scroll", onScroll, { passive: true });
    return () => el.removeEventListener("scroll", onScroll);
  }, [scroller]);
  useLayoutEffect(() => {
    const el = scroller();
    if (el && near.current) el.scrollTop = el.scrollHeight;
  }, [talk, scroller]);
  const shown = placed(talk, inCard);
  const cards = (list?: TalkProposal[]) => list?.map((p) => <ProposalCard key={p.n} p={p} inReview onSettle={settle} />);
  return (
    <div className={css.turns}>
      {talk.turns.map((t) => (
        <ConvoTurn key={t.n} t={t} talk={talk} root={root}>
          {cards(shown.under.get(t.n))}
        </ConvoTurn>
      ))}
      {following && (
        <Turn by="agent" dense running className={css.answering}>
          {talk.run?.said && (
            <TurnSaid className="dim">
              <Markdown text={talk.run.said} />
            </TurnSaid>
          )}
          <Doing calls={talk.run?.calls ?? []} root={root} />
        </Turn>
      )}
      {cards(shown.after)}
    </div>
  );
}
