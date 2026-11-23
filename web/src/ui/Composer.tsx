import { type ReactNode, type Ref, useRef } from "react";
import css from "./Composer.module.css";
import { cx } from "./cx";
import { useAutosize } from "./kit";
import { Keys } from "./Keys";
import { TextArea } from "./TextArea";

type ComposerProps = {
  // What's typed, which is the page's to keep (a draft, say).
  value: string;
  onChange: (value: string) => void;
  // Sending it, by its key. mod is whether ⌘ (or ctrl) was held: a
  // composer whose ⏎ sends one way and ⌘⏎ another reads it.
  onSend: (how: { mod: boolean }) => void;
  // The key that sends: ⌘⏎, where ⏎ is a new line (the default), or ⏎,
  // where ⇧⏎ is.
  send?: "mod+enter" | "enter";
  placeholder?: string;
  "aria-label": string;
  // How many lines it starts at, and how tall it grows before it scrolls.
  rows?: number;
  tallest?: number;
  // Where what's pressed is: in a bar under the words (the default),
  // beside them (a small one, in a panel), or nowhere, with the keys that
  // send in the box's corner.
  layout?: "bar" | "beside" | "corner";
  // The bar's words: what sending does.
  hint?: ReactNode;
  // What else can be done from the bar, before the keys.
  also?: ReactNode;
  // What sends it, and with "beside" every button there is.
  children?: ReactNode;
  // What goes with the words, over them in the box.
  over?: ReactNode;
  // Escape: the box gives up the keys (the default), it's left for the
  // page to hear, or it's the page's to do, and no one else hears it.
  escape?: "blur" | "leave" | ((box: HTMLTextAreaElement) => void);
  // It's being sent.
  busy?: boolean;
  // The box, for whoever gives it the keys.
  field?: Ref<HTMLTextAreaElement>;
  onBlur?: () => void;
  // The page's own class, to place it.
  className?: string;
};

// Composer is where something's written and sent: a reply, a message to
// crew, what should change. It's one box around the words and what's
// pressed, which has the ring while the words have the keys, and grows
// as they're written. The words are the page's (value, onChange); the
// key that sends them is here, and what it shows of itself: in a bar the
// keys are beside the button while the box has the keys, and a button
// beside the words says its own.
export function Composer({
  value,
  onChange,
  onSend,
  send = "mod+enter",
  placeholder,
  rows = 1,
  tallest,
  layout = "bar",
  hint,
  also,
  children,
  over,
  escape = "blur",
  busy,
  field,
  onBlur,
  className,
  ...rest
}: ComposerProps) {
  const box = useRef<HTMLTextAreaElement>(null);
  useAutosize(box, value, tallest);
  // The box is this part's, to fit, and the page's, to give the keys.
  const both = (el: HTMLTextAreaElement | null) => {
    box.current = el;
    if (typeof field === "function") field(el);
    else if (field) field.current = el;
  };
  const keys = send === "mod+enter" ? "mod+⏎" : "⏎";
  return (
    <div className={cx(css.composer, layout === "beside" && css.beside, layout === "corner" && css.corner, className)} data-composer="">
      {over && <div className={css.over}>{over}</div>}
      <TextArea
        ref={both}
        className={css.field}
        rows={rows}
        value={value}
        placeholder={placeholder}
        aria-label={rest["aria-label"]}
        aria-busy={busy || undefined}
        onChange={(e) => onChange(e.target.value)}
        onBlur={onBlur}
        onKeyDown={(e) => {
          if (e.nativeEvent.isComposing) return;
          const mod = e.metaKey || e.ctrlKey;
          if (e.key === "Enter" && (send === "enter" ? !e.shiftKey : mod)) {
            e.preventDefault();
            onSend({ mod });
          } else if (e.key === "Escape" && escape !== "leave") {
            e.preventDefault();
            if (escape === "blur") e.currentTarget.blur();
            else {
              e.stopPropagation();
              escape(e.currentTarget);
            }
          }
        }}
      />
      {layout === "bar" && (
        <div className={css.bar}>
          {hint ? <span className={css.hint}>{hint}</span> : <span className={css.spacer} />}
          {also}
          <Keys k={keys} className={css.keys} />
          {children}
        </div>
      )}
      {layout === "beside" && children}
      {layout === "corner" && <Keys k={keys} className={cx(css.keys, css.cornerKeys)} />}
    </div>
  );
}
