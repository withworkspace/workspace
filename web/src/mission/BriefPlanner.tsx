import { type ReactNode, type Ref, type RefObject, useEffect, useLayoutEffect, useRef, useState } from "react";
import { Doing } from "../ui/Doing";
import { Link } from "react-router";
import type { TalkDetail, TalkProposal } from "../api";
import { type Chip, placed, readsAs, waiting } from "../ask/ask";
import { Markdown } from "../markdown/LazyMarkdown";
import { ProposalCard, SettledMarker } from "../talk/Talk";
import { settled } from "../talk/proposals";
import { Calls } from "../ui/Calls";
import { Icon } from "../ui/Icon";
import { Keys } from "../ui/kit";
import { docLines, type DocLine } from "./briefpage";
import { Button } from "../ui/Button";

// The planner, beside a mission's brief and plan (BriefTab.tsx): its
// conversation as it's been said, where the brief's page used to keep it
// behind an ask bar; and the field it's asked in.

const isDoc = (p: TalkProposal) => p.kind === "brief" || p.kind === "plan";

// BriefPlanner is the mission's planner's conversation: what you and it
// have said, its steps folded over each reply, and what it proposed under
// the reply it came with. What it proposed for the brief or the plan is a
// line (the documents show it, as changes to read): the one they show
// leads to them. Anything else (crew, a decision, a mission) is its card,
// to confirm here. centre is the conversation in the documents' place,
// while there's nothing drafted to be beside; action is what its head
// offers besides (writing the brief yourself, then).
export function BriefPlanner({
  t,
  slug,
  root,
  shown,
  centre,
  field,
  action,
  onSettle,
  toDocs,
  fold,
}: {
  t: TalkDetail;
  slug: string;
  root: string;
  shown?: number; // the proposal the documents show
  centre?: boolean;
  field: ReactNode;
  action?: ReactNode;
  onSettle: (n: number, action: "accept" | "decline", edits?: object) => Promise<void>;
  toDocs: () => void;
  fold?: () => void;
}) {
  const body = useRef<HTMLDivElement>(null);
  // It keeps to its latest, unless you've scrolled up in it.
  const near = useRef(true);
  useLayoutEffect(() => {
    const el = body.current;
    if (el && near.current) el.scrollTop = el.scrollHeight;
  }, [t]);

  const following = waiting(t);
  const lines = docLines(t, shown);
  // Everything else it proposed, by the reply it came with, as its cards.
  const cards = placed(t, (p) => !isDoc(p));
  const line = (l: DocLine) => (
    <button key={l.p.n} type="button" className={`brief-proposed${l.shown ? " shown" : ""} is-${l.p.state}`} onClick={l.shown ? toDocs : undefined} disabled={!l.shown} title={l.shown ? "It's what the documents show: go to them" : undefined}>
      <Icon name={l.p.kind === "plan" ? "list-ordered" : "file"} size={13} />
      <span className="brief-proposed-name">{l.name}</span>
      <span className="brief-proposed-fate">{l.fate}</span>
      {l.shown && <Icon name="arrow-left" size={12} />}
    </button>
  );
  const card = (p: TalkProposal, replying?: boolean) => <ProposalCard key={p.n} p={p} slug={slug} replying={replying} onSettle={onSettle} />;
  return (
    <aside className={`brief-planner${centre ? " centre" : ""}`} aria-label="The planner's conversation">
      <div className="brief-planner-head">
        <span className="avatar avatar-agent" aria-hidden>
          <Icon name="sparkle" size={12} />
        </span>
        <span className="brief-planner-name">Planner</span>
        <Link to={`/missions/${slug}/conversations/${t.id}`} className="brief-planner-id" title="The whole conversation, on its own page">
          <Icon name="message" size={11} /> {t.id}
        </Link>
        <span className="spacer" />
        {action}
        {fold && (
          <Button variant="ghost" square type="button" onClick={fold} title="Fold the planner away (p)" aria-label="Fold the planner away">
            <Icon name="panel" size={14} />
          </Button>
        )}
      </div>
      <div
        className="brief-planner-body"
        ref={body}
        aria-live="polite"
        onScroll={(e) => {
          const el = e.currentTarget;
          near.current = el.scrollHeight - el.scrollTop - el.clientHeight < 60;
        }}
      >
        {t.turns.map((turn) => {
          const m = settled(turn, t.proposals);
          if (m) return <SettledMarker key={turn.n} m={m} t={turn} compact />;
          if (turn.by === "you") {
            return (
              <div key={turn.n} className="turn by-you landed">
                <div className="said">
                  <div className="said-text">
                    <YourWords body={turn.body} />
                  </div>
                </div>
              </div>
            );
          }
          return (
            <div key={turn.n} className={`turn by-${turn.by === "agent" ? "agent" : "ws"} landed`}>
              {turn.calls && turn.calls.length > 0 && <Calls calls={turn.calls} root={root} />}
              <div className="said">
                <Markdown text={turn.body} pinned={turn.refs} />
              </div>
              {lines.under.get(turn.n)?.map(line)}
              {cards.under.get(turn.n)?.map((p) => card(p))}
            </div>
          );
        })}
        {following && (
          <div className="turn by-agent running">
            {t.run?.said && (
              <div className="said dim">
                <Markdown text={t.run.said} />
              </div>
            )}
            <Doing calls={t.run?.calls ?? []} root={root} />
          </div>
        )}
        {lines.after.map(line)}
        {cards.after.map((p) => card(p, following))}
      </div>
      <div className="brief-planner-foot">{field}</div>
    </aside>
  );
}

// quoted is a passage as a message quotes it, over what's said of it.
export const quoted = (passage: string, body: string) =>
  `${passage
    .split("\n")
    .map((l) => `> ${l}`)
    .join("\n")}\n\n${body}`;

// YourWords are a turn of yours: the passage it quotes, then what you said.
function YourWords({ body }: { body: string }) {
  const quote = /^((?:> .*\n)+)\n([\s\S]*)$/.exec(body);
  if (!quote) return <>{body}</>;
  return (
    <>
      <span className="brief-quoted">{readsAs(quote[1].replace(/^> /gm, ""))}</span>
      {quote[2]}
    </>
  );
}

// PlannerField is where the planner's asked: the ask bar's field, with
// what to ask offered over it as it's gone to, and a passage in hand as a
// chip. placeholder is what it says while empty; busy, while what was
// asked is being sent.
export function PlannerField({
  held,
  dropHeld,
  suggestions,
  ask,
  input,
  busy,
  placeholder = "Ask the planner…",
}: {
  held?: Chip & { placeholder: string; drop: string };
  dropHeld: () => void;
  suggestions: string[];
  ask: (text: string) => void;
  input?: Ref<HTMLInputElement>;
  busy: boolean;
  placeholder?: string;
}) {
  const [text, setText] = useState("");
  const [focused, setFocused] = useState(false);
  const [pick, setPick] = useState(-1);
  const offered = focused && !text && !busy ? suggestions : [];
  const send = (q: string) => {
    if (!q.trim() || busy) return;
    ask(q.trim());
    setText("");
    setPick(-1);
  };
  const says = held ? held.placeholder : placeholder;
  return (
    <div className="askbar-wrap brief-ask">
      {offered.length > 0 && (
        <div className="ask-suggest" role="listbox" aria-label="Suggestions">
          {offered.map((s, i) => (
            <button type="button" key={s} role="option" aria-selected={i === pick} className={`ask-suggestion${i === pick ? " sel" : ""}`} onMouseDown={(e) => e.preventDefault()} onMouseMove={() => i !== pick && setPick(i)} onClick={() => send(s)}>
              <Icon name="sparkle" size={11} />
              {s}
            </button>
          ))}
        </div>
      )}
      <div className={`askbar${busy ? " sending" : ""}`}>
        <Icon name={busy ? "spinner" : "sparkle"} size={14} className={`askbar-icon${busy ? " spin" : ""}`} />
        {held && (
          <span className={`ask-ctx ${held.kind}`} title={held.title}>
            <Icon name={held.icon} size={11} />
            <span className="q">{held.text}</span>
            <Button type="button" onClick={dropHeld} title={held.drop} aria-label={held.drop}>
              <Icon name="x" size={11} />
            </Button>
          </span>
        )}
        <input
          ref={input}
          value={text}
          placeholder={says}
          aria-label={says}
          aria-busy={busy}
          onChange={(e) => {
            setText(e.target.value);
            setPick(-1);
          }}
          onKeyDown={(e) => {
            if (e.nativeEvent.isComposing) return;
            if (e.key === "Enter") {
              e.preventDefault();
              send(!text && offered[pick] ? offered[pick] : text);
            } else if (e.key === "Escape") {
              e.preventDefault();
              if (pick >= 0) return setPick(-1);
              e.currentTarget.blur();
            } else if (e.key === "ArrowUp" && offered.length) {
              // What's offered stands over the field, the first nearest it.
              e.preventDefault();
              setPick((x) => Math.min(offered.length - 1, x + 1));
            } else if (e.key === "ArrowDown" && offered.length) {
              e.preventDefault();
              setPick((x) => Math.max(-1, x - 1));
            } else if (e.key === "Backspace" && !text && held) {
              dropHeld();
            }
          }}
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

// PassageTool is what's offered of a passage picked out of the documents:
// asking the planner about it. It goes as the documents scroll under it.
export function PassageTool({ within, ask }: { within: RefObject<HTMLElement | null>; ask: (passage: string) => void }) {
  const [at, setAt] = useState<{ left: number; top: number; passage: string }>();
  useEffect(() => {
    const el = within.current;
    if (!el) return;
    const look = () =>
      setTimeout(() => {
        const passage = pickedIn(el);
        const sel = getSelection();
        if (!passage || !sel?.rangeCount) return setAt(undefined);
        const r = sel.getRangeAt(0).getBoundingClientRect();
        setAt({ left: Math.max(120, Math.min(r.left + r.width / 2, window.innerWidth - 120)), top: Math.max(el.getBoundingClientRect().top + 40, r.top - 8), passage });
      });
    const mine = (e: Event) => !!(e.target as Element | null)?.closest?.(".seltool");
    // (A drag that started in the text can end outside it.)
    const up = (e: MouseEvent) => !mine(e) && look();
    const keyed = (e: KeyboardEvent) => e.shiftKey && look();
    const away = (e: MouseEvent) => !mine(e) && setAt(undefined);
    const gone = () => setAt(undefined);
    document.addEventListener("mouseup", up);
    document.addEventListener("keyup", keyed);
    document.addEventListener("mousedown", away);
    // Whatever scrolls them: the documents together, or each by itself.
    el.addEventListener("scroll", gone, { passive: true, capture: true });
    document.querySelector("main")?.addEventListener("scroll", gone, { passive: true });
    window.addEventListener("resize", gone);
    return () => {
      document.removeEventListener("mouseup", up);
      document.removeEventListener("keyup", keyed);
      document.removeEventListener("mousedown", away);
      el.removeEventListener("scroll", gone, { capture: true });
      document.querySelector("main")?.removeEventListener("scroll", gone);
      window.removeEventListener("resize", gone);
    };
  }, [within]);
  if (!at) return null;
  return (
    <div className="seltool" role="toolbar" aria-label="The passage picked out" style={{ left: at.left, top: at.top }} onMouseDown={(e) => e.preventDefault()}>
      <Button
        type="button"
        onClick={() => {
          setAt(undefined);
          ask(at.passage);
          getSelection()?.removeAllRanges();
        }}
        title="Ask the planner about this passage (a)"
      >
        <Icon name="sparkle" size={12} /> Ask about this <Keys k="a" />
      </Button>
    </div>
  );
}

// pickedIn is the passage picked out of the documents, as it reads; none
// for a selection that isn't in them, or is too little to be one.
export function pickedIn(docs: HTMLElement | null): string | undefined {
  const sel = getSelection();
  if (!docs || !sel || sel.isCollapsed || !sel.anchorNode) return undefined;
  const at = sel.anchorNode.nodeType === Node.ELEMENT_NODE ? (sel.anchorNode as Element) : sel.anchorNode.parentElement;
  if (!at || !docs.contains(at) || !at.closest(".brief-doc .markdown")) return undefined;
  const passage = readsAs(sel.toString());
  return passage.length >= 3 ? passage : undefined;
}
