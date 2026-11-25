import "prosemirror-view/style/prosemirror.css";
import "prosemirror-gapcursor/style/gapcursor.css";
import type { EditorView } from "prosemirror-view";
import { useRef, useState } from "react";
import type { CellHost } from "./cells";
import { EditorRich, type RichHandle } from "./EditorRich";
import { EditorSource } from "./EditorSource";
import css from "./MarkdownEditor.module.css";
import type { RefHost } from "./refs";
import type { Quote } from "./views";

// MarkdownEditor edits a markdown document as it reads: headings as
// headings, lists as lists, a task's box to tick. Markdown typed is
// formatting (# heading, - list, **bold**), the toolbar does the same, and
// Markdown shows the text itself, to edit as text.
//
// What it hands back is markdown that changes only what was edited
// (markdown.ts): a brief saved after one word changed differs from the
// one before by that word.

// Draft is where an edit stands: whether it's changed anything, whether
// there's anything in it, and its markdown (worked out when asked).
export interface Draft {
  dirty: boolean;
  empty: boolean;
  text: () => string;
}

export interface EditorProps {
  initial: string;
  // What it's editing, for screen readers: "The brief".
  label: string;
  placeholder?: string;
  autoFocus?: boolean;
  // Smaller, for a card (a proposal's brief).
  compact?: boolean;
  // Start in Markdown (the text itself), as for settling a clash.
  startIn?: "rich" | "source";
  // Words threads are on, highlighted as the reader highlights them.
  quotes?: Quote[];
  onChange?: (draft: Draft) => void;
  onSubmit?: () => void;
  onCancel?: () => void;
  // A page's: lines that start with >> become cells, which it fills
  // (cells.ts). onView hands it the editor while there is one (not in
  // Markdown), to write into.
  cells?: CellHost;
  onView?: (view: EditorView | undefined) => void;
  // A page's: what its passes found is drawn on the words it's about
  // (notes.ts), and this says when one's clicked.
  onNote?: (key: string) => void;
  // A page's: what its words name (a file, a mission) is drawn as what it
  // is, with a card on hover (refs.ts).
  refs?: RefHost;
  // A page with one toolbar for several documents (a mission's brief and
  // its plan): where this one's toolbar is drawn (toolbarIn, an element of
  // the page's) while it's the one being edited (toolbarOn), and onFocus
  // says when it's gone to. Without toolbarIn, its toolbar is over it.
  toolbarIn?: HTMLElement | null;
  toolbarOn?: boolean;
  onFocus?: () => void;
}

export default function MarkdownEditor(props: EditorProps) {
  const [mode, setMode] = useState<"rich" | "source">(props.startIn ?? "rich");
  // The markdown the rich editor starts from: the initial text, or what
  // was written in Markdown mode.
  const [from, setFrom] = useState(props.initial);
  const [source, setSource] = useState(props.initial);
  const rich = useRef<RichHandle>(null);

  const toSource = () => {
    const text = rich.current?.markdown() ?? from;
    setSource(text);
    setMode("source");
  };
  const toRich = () => {
    setFrom(source);
    setMode("rich");
  };

  return (
    <div className={css.editor}>
      {mode === "rich" ? (
        <EditorRich key={from} ref={rich} {...props} markdown={from} onSource={toSource} />
      ) : (
        <EditorSource {...props} text={source} setText={setSource} onRich={toRich} />
      )}
    </div>
  );
}
