import { type Ref, useRef, useState } from "react";
import { DockFoot } from "../chrome/DockFoot";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import css from "./Composer.module.css";
import { Composer as Box } from "../ui/Composer";

type ComposerProps = {
  // What the box says while it's empty, and is called.
  placeholder: string;
  // Saying it: what's typed is dropped once it's said, and kept if it
  // couldn't be.
  onSay: (text: string) => Promise<unknown>;
  // What you're holding on the page, which goes with what you say.
  holding?: { label: string; title?: string };
  // The box, for whoever gives it the keys.
  field?: Ref<HTMLTextAreaElement>;
  // What's written, where the pane keeps it (a draft that outlasts the
  // pane): given with onText. Without, it's the box's own, and goes with
  // it.
  text?: string;
  onText?: (text: string) => void;
};

// How tall the box grows as you write, before it scrolls.
const tallest = 220;

// Composer is where you say something in a conversation beside the page:
// at the pane's foot, under what's been said. It's the app's composer (a
// conversation's own page has it, and a crew's): the same box, which
// grows as you write (ui/Composer.tsx). ⌘⏎ says it, ⏎ is a new line, esc
// gives the keys back to the page. There's no button for it: the keys
// that say it are in the box's corner while it has the keys.
export function Composer({ placeholder, onSay, holding, field, text: kept, onText }: ComposerProps) {
  const [own, setOwn] = useState("");
  const text = kept ?? own;
  const setText = onText ?? setOwn;
  // What's written now, for when what was said comes back.
  const latest = useRef(text);
  latest.current = text;
  const [saying, setSaying] = useState(false);
  const say = () => {
    const q = text.trim();
    if (!q || saying) return;
    setSaying(true);
    onSay(q)
      .then(
        () => latest.current.trim() === q && setText(""), // not what's been typed since
        () => {}, // whoever couldn't say it has said why
      )
      .finally(() => setSaying(false));
  };
  return (
    <DockFoot>
      <Box
        layout="corner"
        value={text}
        onChange={setText}
        onSend={say}
        placeholder={placeholder}
        aria-label={placeholder}
        tallest={tallest}
        busy={saying}
        field={field}
        escape={(box) => box.blur()}
        over={
          holding && (
            <span className={cx("ask-ctx holding", css.holding)} title={holding.title ?? holding.label}>
              <Icon name="layers" size={11} />
              <span className="q">{holding.label}</span>
            </span>
          )
        }
      />
    </DockFoot>
  );
}
