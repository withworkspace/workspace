import { type Ref, useRef, useState } from "react";
import { DockFoot } from "../chrome/DockFoot";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import { Keys, useAutosize } from "../ui/kit";
import css from "./Composer.module.css";

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
};

// How tall the box grows as you write, before it scrolls.
const tallest = 220;

// Composer is where you say something in a conversation beside the page:
// at the pane's foot, under what's been said. It's the app's composer (a
// conversation's own page has it, and a crew's): the same box, which
// grows as you write. ⌘⏎ says it, ⏎ is a new line, esc gives the keys
// back to the page. There's no button for it: the keys that say it are
// in the box's corner while it has the keys.
export function Composer({ placeholder, onSay, holding, field }: ComposerProps) {
  const [text, setText] = useState("");
  const [saying, setSaying] = useState(false);
  const box = useRef<HTMLTextAreaElement>(null);
  useAutosize(box, text, tallest);
  // The box is this part's, to fit, and its pane's, to give the keys.
  const both = (el: HTMLTextAreaElement | null) => {
    box.current = el;
    if (typeof field === "function") field(el);
    else if (field) field.current = el;
  };
  const say = () => {
    const q = text.trim();
    if (!q || saying) return;
    setSaying(true);
    onSay(q)
      .then(
        () => setText((t) => (t.trim() === q ? "" : t)), // not what's been typed since
        () => {}, // whoever couldn't say it has said why
      )
      .finally(() => setSaying(false));
  };
  return (
    <DockFoot>
      <div className={cx("composer", css.composer)}>
        {holding && (
          <div className={css.holding}>
            <span className="ask-ctx holding" title={holding.title ?? holding.label}>
              <Icon name="layers" size={11} />
              <span className="q">{holding.label}</span>
            </span>
          </div>
        )}
        <textarea
          ref={both}
          rows={1}
          value={text}
          placeholder={placeholder}
          aria-label={placeholder}
          aria-busy={saying}
          data-field
          onChange={(e) => setText(e.target.value)}
          onKeyDown={(e) => {
            if (e.nativeEvent.isComposing) return;
            if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
              e.preventDefault();
              say();
            } else if (e.key === "Escape") {
              e.preventDefault();
              e.stopPropagation();
              e.currentTarget.blur();
            }
          }}
        />
        <Keys k="mod+⏎" className={cx("composer-keys", css.keys)} />
      </div>
    </DockFoot>
  );
}
