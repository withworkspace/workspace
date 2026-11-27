import { lazy, type Ref, Suspense, useRef } from "react";
import type { RefHost } from "../editor/refs";
import type { Features } from "../editor/compose";
import { cx } from "./cx";
import css from "./RichTextArea.module.css";
import { type SendKey, sent, useSendKey } from "./sendKey";
import { useAutosize } from "./useAutosize";

export type { Features } from "../editor/compose";

export interface RichTextAreaProps extends Features {
  // What's written, as markdown, which is the page's to keep (a draft,
  // say). It's read again only when it's other than what was last said
  // by onChange: a box emptied once it's sent, a draft come back.
  value: string;
  onChange: (markdown: string) => void;
  // Names drawn as what they are, with a card on hover (editor/refs.ts).
  refs?: RefHost;
  // The key that sends (ui/sendKey.ts), yours unless a story says, and
  // sending. With no onSend, Enter is only ever a new line.
  send?: SendKey;
  onSend?: (how: { other: boolean }) => void;
  // Escape, if the box hears it: otherwise it's the page's. The press is
  // given, for a box that keeps it from the page (stopPropagation).
  onEscape?: (e: { stopPropagation(): void }) => void;
  placeholder?: string;
  "aria-label": string;
  // How many lines it starts at, and how tall (in pixels) it grows before
  // it scrolls.
  rows?: number;
  tallest?: number;
  autoFocus?: boolean;
  busy?: boolean;
  // Where the words are typed (the page gives it the keys), and whether
  // they have them.
  field?: Ref<HTMLElement>;
  onFocus?: () => void;
  onBlur?: () => void;
  // The page's class: where it is, its padding, its type. The box has no
  // look of its own: no border, no background, no ring.
  className?: string;
}

// Handoff is what the box knew while it loaded, for the one that's
// loaded: whether its words had the keys, and where the cursor was.
export interface Handoff {
  focused: boolean;
}

// The editor (ProseMirror and the markdown bridge) loads apart from the
// page, as the document editor does, but soon: while the page is idle.
const load = () => import("../editor/RichText");
const Inner = lazy(load);
if (typeof window !== "undefined") {
  const idle = window.requestIdleCallback ?? ((fn: () => void) => window.setTimeout(fn, 200));
  idle(() => void load());
}

// RichTextArea is where something's written in markdown, drawn as it
// reads: **bold** typed is bold, - a list. What it offers is its props'
// (marks, lists, quote, code, refs), and what it hands back is markdown.
// It has no look but its words': a Composer or a ReplyBox draws the box.
// Until the editor's loaded it's a plain text box, which looks the same
// and works, and the editor takes over its words and its keys.
export function RichTextArea(props: RichTextAreaProps) {
  const handoff = useRef<Handoff>({ focused: false });
  return (
    <Suspense fallback={<Plain {...props} handoff={handoff.current} />}>
      <Inner {...props} handoff={handoff.current} />
    </Suspense>
  );
}

// Plain is the box while the editor loads: a textarea, its words as
// they're typed (markdown, as the editor will read them).
function Plain({ value, onChange, send: given, onSend, onEscape, placeholder, rows = 1, tallest = 320, autoFocus, busy, field, onFocus, onBlur, className, handoff, ...rest }: RichTextAreaProps & { handoff: Handoff }) {
  const box = useRef<HTMLTextAreaElement>(null);
  useAutosize(box, value, tallest);
  const yours = useSendKey();
  const send = given ?? yours;
  const both = (el: HTMLTextAreaElement | null) => {
    box.current = el;
    if (typeof field === "function") field(el);
    else if (field) field.current = el;
  };
  return (
    <div className={cx(css.area, className)} data-rich="">
      <textarea
        ref={both}
        className={cx(css.doc, css.plain)}
        style={{ "--rows": rows } as React.CSSProperties}
        rows={rows}
        value={value}
        placeholder={placeholder}
        aria-label={rest["aria-label"]}
        aria-busy={busy || undefined}
        autoFocus={autoFocus}
        onChange={(e) => onChange(e.target.value)}
        onFocus={() => {
          handoff.focused = true;
          onFocus?.();
        }}
        onBlur={(e) => {
          // Not when it's taken away for the editor: that keeps the keys.
          if (e.currentTarget.isConnected) handoff.focused = false;
          onBlur?.();
        }}
        onKeyDown={(e) => {
          if (e.nativeEvent.isComposing) return;
          const how = onSend && sent(e, send);
          if (how) {
            e.preventDefault();
            onSend(how);
          } else if (e.key === "Escape" && onEscape) {
            e.preventDefault();
            onEscape(e);
          }
        }}
      />
    </div>
  );
}
