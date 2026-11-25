import type { Meta, StoryObj } from "@storybook/react-vite";
import type { EditorView } from "prosemirror-view";
import { useEffect, useState } from "react";
import { setCells } from "./cells";
import { MarkdownEditor } from "./LazyEditor";
import { setNotes } from "./notes";
import { schema } from "./schema";

// A document as it's edited: every kind of block the editor draws, a
// card's (compact), its markdown as text, empty, with threads and a
// pass's findings on its words, with a page's cells, and its toolbar in a
// page's bar.

const meta: Meta = { title: "Editor/MarkdownEditor" };
export default meta;

const none = () => {};

const doc = `# Limit each key

A paragraph with **strong words**, *a slant*, \`inline code\` and [a link](https://example.com).

## What changes

- A bullet
- Another

1. First
2. Second

- [ ] Still to do
- [x] Done

> A quote.

\`\`\`go
func limit(key string) int {
	return 100
}
\`\`\`

\`\`\`mermaid
graph LR
  A[Asked] --> B[Limited]
\`\`\`

<div>Some HTML, as written.</div>

---

The end.
`;

const frame = (children: React.ReactNode) => <div style={{ padding: 32, maxWidth: 760 }}>{children}</div>;

export const EveryBlock: StoryObj = { render: () => frame(<MarkdownEditor initial={doc} label="The brief" />) };

export const Compact: StoryObj = { render: () => frame(<MarkdownEditor initial={doc} label="The brief" compact />) };

export const AsMarkdown: StoryObj = { render: () => frame(<MarkdownEditor initial={doc} label="The brief" startIn="source" />) };

export const CompactAsMarkdown: StoryObj = { render: () => frame(<MarkdownEditor initial={doc} label="The brief" startIn="source" compact />) };

export const Empty: StoryObj = { render: () => frame(<MarkdownEditor initial="" label="The brief" placeholder="What should it do?" />) };

export const Threads: StoryObj = {
  render: () =>
    frame(
      <MarkdownEditor
        initial={doc}
        label="The brief"
        quotes={[
          { exact: "strong words", kind: "ask" },
          { exact: "inline code", kind: "change" },
          { exact: "Another", kind: "note" },
        ]}
      />,
    ),
};

const page = `<!-- ws:cell w1 v1 | a table of the routes -->

What the cell wrote, a line of it.

<!-- /ws:cell -->

<!-- ws:cell w2 v1 | the same, again -->

Being written again.

<!-- /ws:cell -->

<!-- ws:cell w3 v1 | an outline -->
<!-- /ws:cell -->

<!-- ws:cell w4 v1 | what it read -->

It stopped.

<!-- /ws:cell -->

<!-- ws:cell w5 v1 ask | which one is right -->
<!-- /ws:cell -->

<!-- ws:cell w6 v1 ask | and this -->
<!-- /ws:cell -->

<!-- ws:cell w7 v1 | written from what's changed -->

Old words.

<!-- /ws:cell -->

A finding on these words, a cold read on these, and a formal one here.
`;

function Page() {
  const [view, setView] = useState<EditorView>();
  useEffect(() => {
    if (!view) return;
    // Lines that ask for a cell, as they're typed (markdown would read
    // them as quotes).
    const line = (words: string) => schema.nodes.paragraph.create(null, schema.text(words));
    view.dispatch(view.state.tr.insert(0, [line(">> a summary of what changed"), line(">? which keys are limited")]));
    setCells(
      view,
      new Map([
        ["w1", { status: "filled", note: "Changed since you read it" }],
        ["w2", { status: "filling", note: "Writing…" }],
        ["w3", { status: "starting" }],
        ["w4", { status: "failed", note: "It stopped" }],
        ["w6", { status: "filled", note: "Answered" }],
        ["w7", { status: "stale", note: "What it read has changed" }],
      ]),
    );
    setNotes(view, [
      { key: "n1", quote: "these words", pass: "check" },
      { key: "n2", quote: "a cold read", pass: "cold" },
      { key: "n3", quote: "a formal one", pass: "formal" },
    ]);
  }, [view]);
  return <MarkdownEditor initial={page} label="The page" cells={{ asked: none, open: none }} onView={setView} onNote={none} />;
}

export const Cells: StoryObj = { render: () => frame(<Page />) };

function InABar() {
  const [slot, setSlot] = useState<HTMLElement | null>(null);
  return (
    <>
      <div className="brief-toolbar" style={{ padding: "8px 32px" }}>
        <div className="brief-toolbar-slot" ref={setSlot} />
      </div>
      {frame(<MarkdownEditor initial={doc} label="The brief" toolbarIn={slot} toolbarOn />)}
    </>
  );
}

export const ToolbarInABar: StoryObj = { render: () => <InABar /> };
