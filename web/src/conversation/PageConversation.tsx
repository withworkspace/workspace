import { type KeyboardEvent, type ReactNode, useEffect, useLayoutEffect, useRef, useState } from "react";
import { Doing } from "../ui/Doing";
import { Link } from "react-router";
import { ApiError, askThing, getTalk, keepAsk, sayInTalk, type TalkDetail, type TalkTurn } from "../api";
import { waiting } from "../ask/ask";
import { useKeys } from "../keys";
import { Markdown } from "../markdown/LazyMarkdown";
import { SettledMarker } from "../talk/Talk";
import { settled } from "../talk/proposals";
import { Calls } from "../ui/Calls";
import { Icon, type IconName } from "../ui/Icon";
import { humanError, Keys, toast } from "../ui/kit";
import { type ConversationHost, type Form, fit, givenWith, useHost } from "./host";
import { Button, buttonClass } from "../ui/Button";

// PageConversation draws a page's conversation (host.ts) round the page:
// the ask bar over its foot, the card the answer opens in over the bar, a
// pane docked beside it, or a popover over its corner, with a launcher
// where the bar's off. It's one conversation however it's drawn: moving it
// keeps what's been said. What it's about is the page's thing (about);
// with none, there's nothing to ask about, and it draws nothing.

// About is what a page's conversation is about: a plugin's thing, and
// what the plugin's called, which says whose context the agent was given.
export interface About {
  thing: string;
  plugin: string;
  from: string;
}

// How often a conversation's followed while its agent's answering.
const followEvery = 1000;
// Narrower than this, there's no room to dock: it floats.
const narrowAt = "(max-width: 960px)";

function useNarrow(): boolean {
  const [narrow, setNarrow] = useState(() => typeof window !== "undefined" && !!window.matchMedia?.(narrowAt).matches);
  useEffect(() => {
    const m = window.matchMedia?.(narrowAt);
    if (!m) return;
    const on = () => setNarrow(m.matches);
    m.addEventListener("change", on);
    return () => m.removeEventListener("change", on);
  }, []);
  return narrow;
}

const moves: Record<Form, { to: Form; icon: IconName; label: string }[]> = {
  card: [
    { to: "docked", icon: "panel", label: "Dock it beside the page" },
    { to: "popover", icon: "float", label: "Float it over the page" },
  ],
  docked: [
    { to: "popover", icon: "float", label: "Float it over the page" },
    { to: "card", icon: "fold", label: "Fold it into the bar" },
  ],
  popover: [
    { to: "docked", icon: "panel", label: "Dock it beside the page" },
    { to: "card", icon: "fold", label: "Fold it into the bar" },
  ],
};

export function PageConversation({ host, about, root, children }: { host: ConversationHost; about?: About; root?: string; children: ReactNode }) {
  const s = useHost(host);
  const { settings } = s;
  const narrow = useNarrow();
  const [talk, setTalk] = useState<TalkDetail>();
  const [text, setText] = useState("");
  const [sending, setSending] = useState(false);
  const [keeping, setKeeping] = useState(false);
  const bar = useRef<HTMLInputElement>(null);
  const composer = useRef<HTMLTextAreaElement>(null);
  const body = useRef<HTMLDivElement>(null);
  const near = useRef(true);

  // A conversation is about one thing: another thing open is another
  // conversation, from the start.
  const thing = about?.thing;
  useEffect(() => setTalk((t) => (t && t.anchor?.thing !== thing ? undefined : t)), [thing]);

  const offers = !!about && (settings.ask || settings.allow.length > 0);
  const as = about && s.open ? fit(s.want, settings.allow, narrow) : undefined;
  // The card is the answer: until there is one, opening it is the bar.
  const card = as === "card" && !!talk;
  const showBar = !!about && (as === "card" || (settings.ask && !as));
  const launcher = offers && !settings.ask && settings.allow.includes("popover") && !as;
  const following = waiting(talk);
  const working = sending || following;

  // The page hears where it is.
  useEffect(() => host.shown({ open: !!as, ...(as ? { as } : {}), working }), [host, as, working]);

  // Follow it while its agent answers: its steps as it takes them, then
  // what it said. One that's gone (an ask swept, an hour on) is said to be.
  const id = talk?.id;
  useEffect(() => {
    if (!id || !following) return;
    let stale = false;
    const t = setTimeout(
      () =>
        getTalk("", id).then(
          (d) => !stale && setTalk(d),
          (e: Error) => {
            if (stale) return;
            if (e instanceof ApiError && e.status === 404) gone();
            else setTalk((was) => (was ? { ...was } : was));
          },
        ),
      followEvery,
    );
    return () => {
      stale = true;
      clearTimeout(t);
    };
  }, [id, following, talk]);

  // It keeps to its latest, unless you've scrolled up in it.
  useLayoutEffect(() => {
    const el = body.current;
    if (el && near.current) el.scrollTop = el.scrollHeight;
  }, [talk, as]);

  const gone = () => {
    setTalk(undefined);
    toast("That ask's gone: nobody kept it.");
  };

  // send says it: in the conversation there is, or a new one about the
  // thing. What the page gives goes with it; its standing context reaches
  // the agent only if it's changed since it last did (the server sees to
  // that).
  const send = (question: string, context?: string) => {
    const q = question.trim();
    if (!q || sending || !about) return;
    const given = givenWith(about.from, settings, context);
    const fresh = () => askThing({ thing: about.thing, body: q, given });
    const asked = talk ? sayInTalk("", talk.id, q, given).catch((e: Error) => (e instanceof ApiError && e.status === 404 ? fresh() : Promise.reject(e))) : fresh();
    setSending(true);
    asked
      .then(
        (d) => {
          near.current = true;
          setTalk(d);
          setText((t) => (t.trim() === q ? "" : t));
          host.opened();
        },
        (e: Error) => toast(humanError(e.message), { tone: "danger" }),
      )
      .finally(() => setSending(false));
  };

  // What the page asked to open with: said, once, as the person's.
  const opening = s.opening;
  useEffect(() => {
    if (!opening || !about) return;
    host.took(opening.n);
    if (opening.message) send(opening.message, opening.context);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [opening?.n, !!about]);

  // Going to it: the bar, or the conversation's own box.
  const focus = () => {
    if (as === "docked" || as === "popover") composer.current?.focus();
    else bar.current?.focus();
  };
  useEffect(() => {
    if (s.focus) focus();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [s.focus]);
  // A form it's opened in takes focus once it's there.
  useEffect(() => {
    if (as === "docked" || as === "popover") composer.current?.focus();
  }, [as]);

  useKeys(
    offers
      ? {
          "/": () => (as || settings.ask ? focus() : host.open()),
          "mod+j": () => (as ? focus() : host.open()),
        }
      : {},
  );

  const keep = () => {
    if (!talk || keeping) return;
    setKeeping(true);
    keepAsk("", talk.id)
      .then(setTalk, (e: Error) => (e instanceof ApiError && e.status === 404 ? gone() : toast(humanError(e.message), { tone: "danger" })))
      .finally(() => setKeeping(false));
  };

  const typed = (e: KeyboardEvent<HTMLInputElement | HTMLTextAreaElement>) => {
    if (e.nativeEvent.isComposing) return;
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      send(text);
    } else if (e.key === "Escape") {
      e.preventDefault();
      e.currentTarget.blur();
      if (as) host.move(undefined);
    }
  };

  if (!about) return <div className="convo-stage">{<div className="convo-main">{children}</div>}</div>;

  const placeholder = talk ? "Ask a follow-up…" : (settings.placeholder ?? `Ask about ${about.thing}…`);
  const holding = settings.holding && (
    <span className="ask-ctx holding" title={settings.holding.text ?? settings.holding.label}>
      <Icon name="layers" size={11} />
      <span className="q">{settings.holding.label}</span>
    </span>
  );
  const head = (form: Form) => (
    <div className="convo-head">
      <Icon name="sparkle" size={13} className="askcard-icon" />
      <span className="convo-title" title={talk?.title || talk?.turns[0]?.body}>
        {talk?.title || (talk ? talk.turns[0]?.body : `Ask about ${about.thing}`)}
      </span>
      {talk && (
        <Button variant="ghost" square type="button" onClick={() => setTalk(undefined)} title="Start again" aria-label="Start again">
          <Icon name="plus" size={13} />
        </Button>
      )}
      {moves[form]
        .filter((m) => settings.allow.includes(m.to) && !(m.to === "docked" && narrow))
        .map((m) => (
          <Button variant="ghost" square key={m.to} type="button" onClick={() => host.move(m.to)} title={m.label} aria-label={m.label}>
            <Icon name={m.icon} size={13} />
          </Button>
        ))}
      <Button variant="ghost" square type="button" onClick={() => host.move(undefined)} title="Close (esc)" aria-label="Close">
        <Icon name="x" size={13} />
      </Button>
    </div>
  );
  const thread = (
    <div
      className="convo-body"
      ref={body}
      aria-live="polite"
      onScroll={(e) => {
        const el = e.currentTarget;
        near.current = el.scrollHeight - el.scrollTop - el.clientHeight < 40;
      }}
    >
      {!talk && (
        <div className="convo-empty">
          <p className="dim">Ask about {about.thing}, or ask for a change: it's made where you can see it.</p>
          {(settings.suggestions ?? []).map((q) => (
            <button key={q} type="button" className="ask-suggestion" onClick={() => send(q)}>
              <Icon name="sparkle" size={11} />
              {q}
            </button>
          ))}
        </div>
      )}
      {talk?.turns.map((t) => <ConvoTurn key={t.n} t={t} talk={talk} root={root} />)}
      {following && talk && (
        <div className="ask-live">
          {talk.run?.said && (
            <div className="ask-said dim">
              <Markdown text={talk.run.said} />
            </div>
          )}
          <Doing calls={talk.run?.calls ?? []} root={root} />
        </div>
      )}
    </div>
  );
  const foot = talk && (
    <div className="askcard-foot">
      <span>{talk.ephemeral ? "Gone an hour after its last turn, unless you keep it." : "Kept: it's in Conversations."}</span>
      <span className="spacer" />
      {talk.ephemeral ? (
        <Button size="small" type="button" onClick={keep} disabled={keeping} aria-busy={keeping} title={`Keep it as a conversation, in Conversations, about ${about.thing}`}>
          <Icon name="message" size={12} /> Keep
        </Button>
      ) : (
        <Link to={`/conversations/${encodeURIComponent(talk.id)}`} className={buttonClass({ size: "small" }, "askcard-link")} title="Open it in Conversations">
          <Icon name="message" size={12} /> Open it
        </Link>
      )}
    </div>
  );
  const box = (
    <div className={`convo-composer${sending ? " sending" : ""}`}>
      {holding}
      <textarea ref={composer} rows={1} value={text} placeholder={placeholder} aria-label={placeholder} onChange={(e) => setText(e.target.value)} onKeyDown={typed} />
      <Button variant="primary" size="small" type="button" onClick={() => send(text)} disabled={!text.trim() || sending} aria-busy={sending}>
        {sending ? <Icon name="spinner" size={12} className="spin" /> : "Send"}
      </Button>
    </div>
  );
  const panel = (form: "docked" | "popover") => (
    <div className={`convo-panel ${form}`} role="dialog" aria-label={`Conversation about ${about.thing}`}>
      {head(form)}
      {thread}
      {foot}
      {box}
    </div>
  );

  return (
    <div className="convo-stage">
      <div className="convo-main">
        {children}
        {as === "popover" && panel("popover")}
        {launcher && (
          <button type="button" className={`convo-launcher${working ? " working" : ""}`} onClick={() => host.open({ as: "popover" })} title={`Ask about ${about.thing} (⌘J)`} aria-label={`Ask about ${about.thing}`}>
            <Icon name={working ? "spinner" : "sparkle"} size={16} className={working ? "spin" : undefined} />
          </button>
        )}
        {showBar && (
          <div className="convo-foot">
            <div className="askbar-wrap convo-askbar">
              {card && (
                <div className="askcard convo-card" role="dialog" aria-label="The answer">
                  {head("card")}
                  {thread}
                  {foot}
                </div>
              )}
              <div className={`askbar${sending ? " sending" : ""}`}>
                <Icon name={working ? "spinner" : "sparkle"} size={14} className={`askbar-icon${working ? " spin" : ""}`} />
                {holding}
                <input ref={bar} value={text} placeholder={placeholder} aria-label={placeholder} aria-busy={sending} onChange={(e) => setText(e.target.value)} onKeyDown={typed} />
                <Keys k="/" className="askbar-key" />
              </div>
            </div>
          </div>
        )}
      </div>
      {as === "docked" && <aside className="convo-dock">{panel("docked")}</aside>}
    </div>
  );
}

// ConvoTurn is a turn: what you said (with what the page gave the agent
// with it, folded), the agent's answer with its steps above it, ws saying
// it couldn't, or the marker of a card you settled.
function ConvoTurn({ t, talk, root }: { t: TalkTurn; talk: TalkDetail; root?: string }) {
  const m = settled(t, talk.proposals);
  if (m) return <SettledMarker m={m} t={t} compact />;
  if (t.by === "you") {
    return (
      <div className="convo-you">
        <div className="convo-said">{t.body}</div>
        {t.given && <Given g={t.given} />}
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
    </div>
  );
}

// Given is what a page gave the agent with something you said: folded,
// for you to look at, as it isn't your words.
function Given({ g }: { g: NonNullable<TalkTurn["given"]> }) {
  const parts: [string, string][] = [];
  if (g.holding || g.holdingText) parts.push([`Selected${g.holding ? ` (${g.holding})` : ""}`, g.holdingText ?? ""]);
  if (g.context) parts.push(["With this message", g.context]);
  if (g.page) parts.push(["The page", g.page]);
  return (
    <details className="convo-given">
      <summary>with context from {g.from || "the page"}</summary>
      {parts.map(([label, text]) => (
        <div key={label}>
          <div className="convo-given-label">{label}</div>
          {text && <pre>{text}</pre>}
        </div>
      ))}
    </details>
  );
}
