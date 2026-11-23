import { type KeyboardEvent, type Ref, useState } from "react";
import { Button } from "../ui/Button";
import { cx } from "../ui/cx";
import { Icon, type IconName } from "../ui/Icon";
import { Spinner } from "../ui/Spinner";
import { Keys } from "../ui/kit";
import { Input } from "../ui/Input";

// What's in hand, as the field shows it before its words: a passage
// picked out of a document, lines of a diff, what a plugin's page says
// you're holding. With drop, it can be let go of (its x, or backspace in
// an empty field).
export type AskHolding = {
  icon: IconName;
  text: string;
  title?: string;
  // Its kind, for how it's drawn: passage, lines, finding, holding.
  kind?: string;
  drop?: { label: string; onDrop: () => void };
};

type AskFieldProps = {
  // What it asks for while it's empty, and what it's called.
  placeholder: string;
  // Asking: what's typed, or what was offered and picked. What's typed is
  // dropped once it's asked, and kept if it couldn't be.
  onAsk: (question: string) => Promise<unknown> | void;
  // It's being asked, or answered: it says so, and takes no more till
  // it's done.
  busy?: boolean;
  holding?: AskHolding;
  // What to offer asking, while it has the keys and nothing's typed.
  suggestions?: string[];
  // The field, for whoever gives it the keys (/).
  field?: Ref<HTMLInputElement>;
  // Where it sits is its page's to say: at the foot of what's read (as it
  // comes), or the page's own (brief-ask, convo-askbar).
  className?: string;
};

// AskField is the one place you ask an agent about what you're looking
// at: a line at the foot of a document, a pull request, a plugin's page,
// a mission's brief. It's the way in, and no more. What asking does is
// its page's (onAsk): on most pages, the conversation opens beside the
// page and carries on there.
//
// / goes to it (the page binds that). ↑ and ↓ go through what's offered,
// ⏎ asks, esc gives the keys back to the page.
export function AskField({ placeholder, onAsk, busy, holding, suggestions = [], field, className }: AskFieldProps) {
  const [text, setText] = useState("");
  const [asking, setAsking] = useState(false);
  const [focused, setFocused] = useState(false);
  const [pick, setPick] = useState(-1);
  const working = busy || asking;
  const offered = focused && !text && !working ? suggestions : [];

  const ask = (question: string) => {
    const q = question.trim();
    if (!q || working) return;
    setPick(-1);
    const asked = onAsk(q);
    const done = () => setText((t) => (t.trim() === q ? "" : t)); // not what's been typed since
    if (!asked) return done();
    setAsking(true);
    asked.then(done, () => {}).finally(() => setAsking(false)); // whoever couldn't ask it has said why
  };

  const keys = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.nativeEvent.isComposing) return;
    if (e.key === "Enter") {
      e.preventDefault();
      ask(!text && offered[pick] ? offered[pick] : text);
    } else if (e.key === "Escape") {
      e.preventDefault();
      if (pick >= 0) return setPick(-1);
      e.currentTarget.blur();
    } else if (e.key === "ArrowUp" && offered.length) {
      // What's offered stands over the field, the first nearest it.
      e.preventDefault();
      setPick((p) => Math.min(offered.length - 1, p + 1));
    } else if (e.key === "ArrowDown" && offered.length) {
      e.preventDefault();
      setPick((p) => Math.max(-1, p - 1));
    } else if (e.key === "Backspace" && !text && holding?.drop) {
      holding.drop.onDrop();
    }
  };

  return (
    <div className={cx("askbar-wrap", className)}>
      {offered.length > 0 && (
        <div className="ask-suggest" role="listbox" aria-label="Suggestions">
          {offered.map((s, i) => (
            <button type="button" key={s} role="option" aria-selected={i === pick} className={`ask-suggestion${i === pick ? " sel" : ""}`} onMouseDown={(e) => e.preventDefault()} onMouseMove={() => i !== pick && setPick(i)} onClick={() => ask(s)}>
              <Icon name="sparkle" size={11} />
              {s}
            </button>
          ))}
        </div>
      )}
      <div className={`askbar${working ? " sending" : ""}`}>
        {working ? <Spinner size={14} className="askbar-icon" /> : <Icon name="sparkle" size={14} className="askbar-icon" />}
        {holding && (
          <span className={`ask-ctx ${holding.kind ?? "holding"}`} title={holding.title ?? holding.text}>
            <Icon name={holding.icon} size={11} />
            <span className="q">{holding.text}</span>
            {holding.drop && (
              <Button type="button" onClick={holding.drop.onDrop} title={holding.drop.label} aria-label={holding.drop.label}>
                <Icon name="x" size={11} />
              </Button>
            )}
          </span>
        )}
        <Input
          ref={field}
          value={text}
          placeholder={placeholder}
          aria-label={placeholder}
          aria-busy={working}
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
