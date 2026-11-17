import { type Command, EditorState, TextSelection } from "prosemirror-state";
import { describe, expect, it } from "vitest";
import { copyMarkdown, pasteMarkdown } from "./clipboard";
import { insertRule, linkAt, listActive, setLink, toggleHeading, toggleList, toggleQuote } from "./commands";
import { parseMarkdown, saveMarkdown } from "./markdown";

// The toolbar's commands and the clipboard, on documents as markdown: what
// a command does is what it does to the file.

// edit runs cmd on md with the cursor at the first place `at` is found,
// and gives back the markdown after.
function edit(md: string, at: string, cmd: Command): string {
  const parsed = parseMarkdown(md);
  let state = EditorState.create({ doc: parsed.doc });
  state = state.apply(state.tr.setSelection(TextSelection.create(state.doc, find(state, at))));
  let next = state;
  expect(cmd(state, (tr) => (next = state.apply(tr)))).toBe(true);
  return saveMarkdown(next.doc, parsed).text;
}

function find(state: EditorState, text: string): number {
  let at = -1;
  state.doc.descendants((node, pos) => {
    if (at < 0 && node.isText && node.text!.includes(text)) at = pos + node.text!.indexOf(text);
    return at < 0;
  });
  if (at < 0) throw new Error(`no ${text}`);
  return at;
}

describe("the toolbar", () => {
  const doc = "# Plan\n\nIntro.\n\n- one\n- two\n\nAfter.\n";

  it("makes a paragraph a task", () => {
    // Right above another list of "-", it'd read as part of it: that one
    // takes "*" to stay a list of its own.
    expect(edit(doc, "Intro", toggleList("task"))).toBe("# Plan\n\n- [ ] Intro.\n\n* one\n* two\n\nAfter.\n");
    expect(edit("Intro.\n", "Intro", toggleList("task"))).toBe("- [ ] Intro.\n");
  });

  it("turns bullets into numbers, and tasks", () => {
    expect(edit(doc, "one", toggleList("ordered"))).toBe("# Plan\n\nIntro.\n\n1. one\n2. two\n\nAfter.\n");
    expect(edit(doc, "one", toggleList("task"))).toBe("# Plan\n\nIntro.\n\n- [ ] one\n- [ ] two\n\nAfter.\n");
  });

  it("takes an item out of its list", () => {
    expect(edit(doc, "two", toggleList("bullet"))).toBe("# Plan\n\nIntro.\n\n- one\n\ntwo\n\nAfter.\n");
  });

  it("knows which list it's in", () => {
    const state = EditorState.create({ doc: parseMarkdown("- [ ] a\n- [x] b\n").doc });
    const inA = state.apply(state.tr.setSelection(TextSelection.create(state.doc, find(state, "a"))));
    expect([listActive(inA, "task"), listActive(inA, "bullet"), listActive(inA, "ordered")]).toEqual([true, false, false]);
  });

  it("toggles a heading", () => {
    expect(edit(doc, "Plan", toggleHeading(1))).toBe("Plan\n\nIntro.\n\n- one\n- two\n\nAfter.\n");
    expect(edit(doc, "Intro", toggleHeading(2))).toBe("# Plan\n\n## Intro.\n\n- one\n- two\n\nAfter.\n");
  });

  it("quotes a paragraph", () => {
    expect(edit(doc, "After", toggleQuote)).toBe("# Plan\n\nIntro.\n\n- one\n- two\n\n> After.\n");
  });

  it("puts a rule in", () => {
    expect(edit("One.\n\nTwo.\n", "One.", insertRule)).toBe("---\n\nOne.\n\nTwo.\n");
    expect(edit("One.\n\nTwo.\n", "ne.", insertRule)).toBe("One.\n\n---\n\nTwo.\n");
  });

  it("links words, and finds the link again", () => {
    const parsed = parseMarkdown("Read the plan.\n");
    const state = EditorState.create({ doc: parsed.doc });
    const from = find(state, "the plan");
    let next = state;
    setLink(from, from + "the plan".length, "https://example.com/plan")(state, (tr) => (next = state.apply(tr)));
    expect(saveMarkdown(next.doc, parsed).text).toBe("Read [the plan](https://example.com/plan).\n");
    const inside = next.apply(next.tr.setSelection(TextSelection.create(next.doc, from + 3)));
    expect(linkAt(inside)).toEqual({ href: "https://example.com/plan", from, to: from + "the plan".length });
  });
});

describe("the clipboard", () => {
  const state = EditorState.create({ doc: parseMarkdown("Some words.\n\n```\ncode\n```\n").doc });

  it("reads pasted text as markdown", () => {
    const slice = pasteMarkdown("**bold** and `code`\n\n- a list", state.doc.resolve(1));
    expect(slice.openStart).toBe(1);
    expect(slice.openEnd).toBe(0);
    expect(slice.content.child(0).child(0).marks.map((m) => m.type.name)).toEqual(["strong"]);
    expect(slice.content.child(1).type.name).toBe("bullet_list");
  });

  it("pastes text into code as it is", () => {
    const slice = pasteMarkdown("**not bold**", state.doc.resolve(find(state, "code")));
    expect(slice.content.textBetween(0, slice.content.size)).toBe("**not bold**");
  });

  it("copies markdown", () => {
    const doc = parseMarkdown("Some **bold** words.\n\n- one\n- two\n").doc;
    const s = EditorState.create({ doc });
    const words = s.apply(s.tr.setSelection(TextSelection.create(doc, find(s, "bold"), find(s, " words"))));
    expect(copyMarkdown(words.selection.content())).toBe("**bold**");
    const items = s.apply(s.tr.setSelection(TextSelection.create(doc, find(s, "one"), find(s, "two") + 3)));
    expect(copyMarkdown(items.selection.content())).toBe("- one\n- two");
  });
});
