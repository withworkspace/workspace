import { lazy, type Ref, Suspense, useRef } from "react";
import type { RefHost } from "../editor/refs";
import type { Features } from "../editor/compose";
import { useRefHost } from "../refs/useRefHost";
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
  // Names drawn as what they are as they're written, with a card on
  // hover: true for what the web UI knows (refs/useRefHost.tsx), or a
  // host of the page's own (editor/refs.ts).
  refs?: boolean | RefHost;
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
  // Not to be written in now (it's being written for you).
  disabled?: boolean;
  // Where the words are typed (the page gives it the keys), and whether
  // they have them.
  field?: Ref<HTMLElement>;
  onFocus?: () => void;
  onBlur?: () => void;
  // A key pressed, the page's first: true if it's done something with it
  // (Tab going round a comment's kinds), and nothing else does.
  onKeyDown?: (e: KeyPress) => boolean;
  // The page's class: where it is, its padding, its type. The box has no
  // look of its own (no border, no background, no ring), unless it's
  // drawn as a field: a TextArea's border, and its ring while it has the
  // keys.
  look?: "bare" | "field";
  className?: string;
}

export type KeyPress = { key: string; shiftKey: boolean; metaKey: boolean; ctrlKey: boolean; altKey: boolean };

// Handoff is what the box knew while it loaded, for the one that's
// loaded: whether its words had the keys, and where the cursor was.
export interface Handoff {
  focused: boolean;
}

// The editor (ProseMirror and the markdown bridge) loads apart from the
// page, as the document editor does, but soon: while the page is idle.
const load = () => import("../editor/RichTextEditor");
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
  const hosting = useRefHost({ text: props.value, enabled: props.refs === true });
  const refs = props.refs === true ? hosting.host : props.refs || undefined;
  return (
    <>
      <Suspense fallback={<Plain {...props} refs={undefined} handoff={handoff.current} />}>
        <Inner {...props} refs={refs} handoff={handoff.current} />
      </Suspense>
      {props.refs === true && hosting.card}
    </>
  );
}

// InnerProps is what the editor's given: the refs' host, if there's one
// yet, and what the box knew while it loaded.
export type InnerProps = Omit<RichTextAreaProps, "refs"> & { refs?: RefHost; handoff: Handoff };

// Plain is the box while the editor loads: a textarea, its words as
// they're typed (markdown, as the editor will read them).
function Plain({ value, onChange, send: given, onSend, onEscape, onKeyDown, placeholder, rows = 1, tallest = 320, autoFocus, busy, disabled, field, onFocus, onBlur, look, className, handoff, ...rest }: InnerProps) {
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
    <div className={cx(css.area, look === "field" && css.field, disabled && css.disabled, className)} data-rich="">
      <textarea
        ref={both}
        className={cx(css.doc, css.plain)}
        style={{ "--rows": rows } as React.CSSProperties}
        rows={rows}
        value={value}
        placeholder={placeholder}
        aria-label={rest["aria-label"]}
        aria-busy={busy || undefined}
        disabled={disabled}
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
          if (onKeyDown?.(e)) {
            e.preventDefault();
            return;
          }
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
