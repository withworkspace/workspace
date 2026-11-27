import { type ReactNode, type Ref, useRef } from "react";
import type { RefHost } from "../editor/refs";
import css from "./Composer.module.css";
import { cx } from "./cx";
import { Keys } from "./Keys";
import { type Features, RichTextArea } from "./RichTextArea";
import { type SendKey, sendKeys, useSendKey } from "./sendKey";

// What a composer offers to write, unless it says: marks, lists, a
// quote, code (ui/RichTextArea.tsx), and pictures pasted in, as what's
// written in one goes to an agent.
const offered: Features = { marks: true, lists: true, quote: true, code: true, pictures: true };

type ComposerProps = {
  // What's written, as markdown, which is the page's to keep (a draft, say).
  value: string;
  onChange: (value: string) => void;
  // Sending it, by its key. other is whether it was the second of two
  // ways to send (ui/sendKey.ts): a composer that sends two ways reads it.
  onSend: (how: { other: boolean }) => void;
  // The key that sends: ⌘⏎, where ⏎ is a new line, or ⏎, where ⇧⏎ is.
  // It's yours to choose (Settings › Keyboard), so a page doesn't say:
  // this is for a story, or a test.
  send?: SendKey;
  placeholder?: string;
  "aria-label": string;
  // What can be written: all of it unless it says (offered).
  offers?: Features;
  // Names drawn as what they are as they're written: what the web UI
  // knows (the default), a host of the page's own, or none (false).
  refs?: boolean | RefHost;
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
  escape?: "blur" | "leave" | ((box: HTMLElement) => void);
  // It's being sent.
  busy?: boolean;
  // The box, for whoever gives it the keys.
  field?: Ref<HTMLElement>;
  onBlur?: () => void;
  // The page's own class, to place it.
  className?: string;
};

// Composer is where something's written and sent: a reply, a message to
// crew, what should change. It's one box around the words and what's
// pressed, which has the ring while the words have the keys, and grows
// as they're written. The words are a RichTextArea: markdown, drawn as it
// reads. The words are the page's (value, onChange); the
// key that sends them is here, the one you've chosen, and what it shows
// of itself: in a bar the keys are beside the button while the box has
// the keys, and a button beside the words says its own (SendKeys).
export function Composer({
  value,
  onChange,
  onSend,
  send: given,
  placeholder,
  offers = offered,
  refs = true,
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
  const box = useRef<HTMLElement>(null);
  // The words are this part's, to give up the keys, and the page's, to
  // give them.
  const both = (el: HTMLElement | null) => {
    box.current = el;
    if (typeof field === "function") field(el);
    else if (field) field.current = el;
  };
  const yours = useSendKey();
  const send = given ?? yours;
  const keys = sendKeys(send);
  return (
    <div className={cx(css.composer, layout === "beside" && css.beside, layout === "corner" && css.corner, className)} data-composer="">
      {over && <div className={css.over}>{over}</div>}
      <RichTextArea
        {...offers}
        refs={refs}
        field={both}
        className={css.field}
        rows={rows}
        tallest={tallest}
        value={value}
        onChange={onChange}
        send={send}
        onSend={onSend}
        placeholder={placeholder}
        aria-label={rest["aria-label"]}
        busy={busy}
        onBlur={onBlur}
        onEscape={
          escape === "leave"
            ? undefined
            : (e) => {
                if (escape === "blur") box.current?.blur();
                else {
                  e.stopPropagation();
                  if (box.current) escape(box.current);
                }
              }
        }
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
