import { type ReactNode, useEffect, useRef, useState } from "react";
import { ApiError, askThing, type TalkDetail } from "../api";
import { AskField } from "../ask/AskField";
import { waiting } from "../ask/ask";
import { askAt, paneAt, talkAt } from "../chrome/panes";
import { useBeside, useChromeStore, useGive } from "../chrome/store";
import { useKeys } from "../keys";
import { toast } from "../ui/toast";
import { humanError } from "../ui/text";
import { ConversationStage } from "./ConversationStage";
import { type ConversationHost, givenWith, useHost } from "./host";
import { useTalk, useTalks } from "./talks";

// PageConversation is a page's conversation (host.ts) as the page has it:
// the ask field over the page's foot, which is the way in. Asking opens
// the conversation beside the page (Beside: panes/TalkPane.tsx), and it's
// held there from then, with what this page gives it while the page is
// open: what you're holding, and the page's state for the agent. A page
// with no bar opens it beside with nothing asked yet (panes/AskPane.tsx):
// ⌘J, or the page asking for it open.
//
// What it's about is the page's thing (about); with none, there's nothing
// to ask about, and it draws only the page. A conversation is about one
// thing: another thing open is another conversation, from the start. It's
// beside this thing's page and not others, and is found again on coming
// back. While it's there to see, the bar's out of the way.

// About is what a page's conversation is about: a plugin's thing, and
// what the plugin's called, which says whose context the agent was given.
export interface About {
  thing: string;
  plugin: string;
  from: string;
}

export function PageConversation({ host, about, children }: { host: ConversationHost; about?: About; children: ReactNode }) {
  const s = useHost(host);
  const { settings } = s;
  const beside = useBeside();
  const open = useChromeStore((c) => c.open);
  const close = useChromeStore((c) => c.close);
  const [sending, setSending] = useState(false);
  // The thing's conversation: the one last asked in here, by its address
  // beside. One asked here before is still beside, about this page's
  // thing: found again by that, on coming back to the page.
  const [mine, setMine] = useState<{ thing: string; to: string }>();
  const bar = useRef<HTMLInputElement>(null);

  const thing = about?.thing;
  const offers = !!about && (settings.ask || settings.allow.length > 0);
  const before = [...beside.about].reverse().find((t) => paneAt(t)?.kind === "conversation");
  const at = (mine && mine.thing === thing ? mine.to : undefined) ?? before;
  const talk = useTalk(at)?.detail;
  // Still beside: what's asked next carries on in it.
  const has = !!at && beside.all.includes(at);
  // The one that hasn't begun, for a page with no bar.
  const blank = thing ? askAt(thing) : undefined;
  const blankOpen = !!blank && beside.all.includes(blank);
  // There to see: its reply is where what's said next is said, and the
  // bar's out of the way.
  const away = (has || blankOpen) && beside.shown;
  const working = sending || (has && waiting(talk));

  // send says it: in the conversation there is, or a new one about the
  // thing. What the page gives goes with it; its standing context reaches
  // the agent only if it's changed since it last did (the server sees to
  // that). The conversation opens beside, in the place of one that hadn't
  // begun, with the keys in its reply.
  const send = (question: string, context?: string): Promise<TalkDetail> => {
    const q = question.trim();
    if (!q || !about) return Promise.reject(new Error("Nothing to ask."));
    const given = givenWith(about.from, settings, context);
    const fresh = () => askThing({ thing: about.thing, body: q, given });
    const asked = has && at ? useTalks.getState().say(at, q, given).catch((e: Error) => (e instanceof ApiError && e.status === 404 ? fresh() : Promise.reject(e))) : fresh();
    setSending(true);
    return asked
      .then(
        (d) => {
          const to = talkAt(d.id);
          useTalks.getState().told(to, d);
          open(to, { here: true, inPlaceOf: blank, keys: true });
          setMine({ thing: about.thing, to });
          return d;
        },
        (e: Error) => {
          toast(humanError(e.message), { tone: "danger" });
          throw e;
        },
      )
      .finally(() => setSending(false));
  };

  // What only this page has, its conversation has while the page is open:
  // the one there is, and the one that hasn't begun.
  const holding = settings.holding && { label: settings.holding.label, title: settings.holding.text };
  useGive(at, { holding, given: () => (about ? givenWith(about.from, settings) : undefined) });
  useGive(offers ? blank : undefined, { thing, holding, placeholder: settings.placeholder, suggestions: settings.suggestions, start: (body) => send(body) });

  // Going to it: its reply, while it's beside; else the bar; and with no
  // bar, beside with nothing asked yet.
  const toIt = (besideAnyway = false) => {
    if (has && at) open(at, { here: true, keys: true });
    else if (settings.ask && !besideAnyway) bar.current?.focus();
    else if (blank) open(blank, { here: true, keys: true });
  };

  // The page hears where it is: beside or not, next to the page (docked)
  // or over it (popover), and whether its agent's working. They're the
  // frame protocol's words, from when the conversation was drawn three
  // ways of its own: any of them is Beside now.
  const shown = beside.shown && (has || blankOpen);
  const as = shown ? (beside.over ? "popover" : "docked") : undefined;
  useEffect(() => host.shown({ open: shown, ...(as ? { as } : {}), working }), [host, shown, as, working]);

  // What the page asked to open with: said, once, as the person's. With
  // nothing to say, it's gone to; asked for beside by name, it's there
  // whether or not there's a bar.
  const opening = s.opening;
  useEffect(() => {
    if (!opening || !about) return;
    host.took(opening.n);
    if (opening.message) void send(opening.message, opening.context).catch(() => {});
    else toIt(opening.as === "docked" || opening.as === "popover");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [opening?.n, !!about]);

  // The page asked for it closed: its tabs go.
  useEffect(() => {
    if (!s.closing) return;
    if (at) close(at);
    if (blank) close(blank);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [s.closing]);

  // The one that hasn't begun is this page's: it goes with the page, and
  // with the thing.
  useEffect(() => {
    if (!blank) return;
    return () => useChromeStore.getState().close(blank);
  }, [blank]);

  useEffect(() => {
    if (s.focus) bar.current?.focus();
  }, [s.focus]);

  useKeys(
    offers
      ? {
          "/": () => (settings.ask && !away ? bar.current?.focus() : toIt()),
          "mod+j": () => toIt(),
        }
      : {},
  );

  return (
    <ConversationStage
      foot={
        about &&
        settings.ask &&
        !away && (
          <AskField
            place="over"
            field={bar}
            placeholder={has ? "Ask a follow-up…" : (settings.placeholder ?? `Ask about ${about.thing}…`)}
            busy={working}
            holding={settings.holding && { icon: "layers", text: settings.holding.label, title: settings.holding.text ?? settings.holding.label }}
            onAsk={(q) => send(q)}
          />
        )
      }
    >
      {children}
    </ConversationStage>
  );
}
