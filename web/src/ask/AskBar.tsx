import { type Ref, useEffect, useImperativeHandle, useRef, useState } from "react";
import { ApiError, type TalkProposal } from "../api";
import { paneAt, talkAt } from "../chrome/panes";
import { useBeside, useChromeStore, useGive } from "../chrome/store";
import { useTalk, useTalks } from "../conversation/talks";
import { toast } from "../ui/toast";
import { humanError } from "../ui/text";
import type { AskAbout } from "./ask";
import { AskField } from "./AskField";

// The ask bar (#123): always there at the foot of what you're reading,
// with / to go to it. Ask about the whole of it, or pick something out of
// it first (a passage of a document, with the selection's toolbar,
// Selection.tsx, or a; lines of a pull request's diff, with a).
//
// It's the way in, and no more: asking opens the conversation beside the
// page (panes/TalkPane.tsx), with the keys in its reply, and what's said
// next is said there. The page keeps its place and its width under what
// it's asked.
//
// It's a component of its own, not the reader's: a page gives it what's
// asked about (how an ask of it starts, and what's picked out of it, as
// its chip shows it) and the suggestions to offer, and it does the rest.
//
// An ask is a conversation from the start (POST /api/asks for a document,
// POST /api/prs/<repo>/<n>/ask for a pull request), but nobody's kept it:
// it isn't listed, and the daemon sweeps it an hour after its last turn.
// Kept (in its pane), it's in Conversations, about what was asked about.
//
// The conversation is about what the page is of, so it's beside this page
// and not others, and is found again on coming back. While it's there to
// see, the bar's out of the way: its reply is where what's said next is
// said, and / goes there. With something in hand the bar's back, for a
// new ask, which takes the place of the page's last beside, unless that
// one was kept.

// AskBarHandle is what a page does with its bar: go to it (/; whole drops
// what's in hand, to ask about all of it), ask with something in hand (a),
// and a question at once (Explain).
export interface AskBarHandle<H> {
  focus: (whole?: boolean) => void;
  ask: (held?: H, question?: string) => void;
}

export function AskBar<H>({
  about,
  suggestions,
  cite,
  onOpen,
  onSettled,
  ref,
}: {
  about: AskAbout<H>;
  // suggestions are what to offer asking, about what's in hand, or the whole.
  suggestions: (held?: H) => string[];
  // cite goes to what an answer cites (#how-it-lands): false if it can't.
  cite?: (href: string) => boolean;
  onOpen?: () => void; // it's been asked: what else is open on the page can close
  onSettled?: (p: TalkProposal) => void; // something it proposed has been accepted or declined
  ref?: Ref<AskBarHandle<H>>;
}) {
  const beside = useBeside();
  const open = useChromeStore((s) => s.open);
  const [held, setHeld] = useState<H>();
  // The page's ask: the conversation this bar last asked in, by its
  // address beside. One asked here before (the bar's forgotten it, having
  // gone with the page) is still beside, about this page's thing: found
  // again by that.
  const [mine, setMine] = useState<string>();
  const before = [...beside.about].reverse().find((t) => paneAt(t)?.kind === "conversation");
  const at = mine ?? before;
  const talk = useTalk(at)?.detail;
  // It's still beside: asking again carries on in it.
  const there = !!at && beside.all.includes(at);
  // While it's there to see, it's where what's said next is said, and the
  // bar's out of the way; with something in hand it's back, since that's
  // a new thing to ask.
  const away = there && beside.shown && held === undefined;
  const field = useRef<HTMLInputElement>(null);
  // Asked to take the keys while it was away: taken once it's drawn.
  const [wanted, setWanted] = useState(0);
  useEffect(() => {
    if (wanted && !away) field.current?.focus();
  }, [wanted, away]);

  // What only this page has, its conversation has while the page is open.
  useGive(at, { cite, onSettled });

  // ask asks: in the page's conversation while it's beside, a follow-up;
  // with something in hand, or none there, a new ask. A follow-up to one
  // that's gone asks afresh.
  const ask = (question: string, h = held) => {
    const fresh = () => about.start(question, h);
    const asked = there && at && h === undefined ? useTalks.getState().say(at, question).catch((e: Error) => (e instanceof ApiError && e.status === 404 ? fresh() : Promise.reject(e))) : fresh();
    return asked.then(
      (d) => {
        const to = talkAt(d.id);
        useTalks.getState().told(to, d);
        // A new ask takes the place of the page's last, if nobody kept that.
        open(to, { here: true, inPlaceOf: at && at !== to && talk?.ephemeral !== false ? at : undefined, keys: true });
        setMine(to);
        setHeld(undefined);
        onOpen?.();
      },
      (e: Error) => {
        toast(humanError(e.message), { tone: "danger" });
        throw e;
      },
    );
  };

  const toReply = () => at && open(at, { here: true, keys: true });
  useImperativeHandle(ref, () => ({
    focus: (whole) => {
      if (whole) setHeld(undefined);
      // Its conversation's beside: that's where to go on.
      if (there && beside.shown && (whole || held === undefined)) toReply();
      else setWanted((n) => n + 1);
    },
    ask: (picked, question) => {
      if (picked !== undefined) setHeld(picked);
      if (question) void ask(question, picked).catch(() => {});
      else if (picked === undefined && there && beside.shown) toReply();
      else setWanted((n) => n + 1);
    },
  }));

  if (away) return null;
  const hand = held !== undefined ? about.chip(held) : undefined;
  return (
    <AskField
      field={field}
      placeholder={hand ? hand.placeholder : there ? "Ask a follow-up…" : `Ask about ${about.title}…`}
      suggestions={suggestions(held)}
      holding={hand && { icon: hand.icon, text: hand.text, title: hand.title, kind: hand.kind, drop: { label: hand.drop, onDrop: () => setHeld(undefined) } }}
      onAsk={(q) => ask(q)}
    />
  );
}
