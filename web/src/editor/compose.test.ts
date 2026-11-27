import { Slice } from "prosemirror-model";
import { EditorState, TextSelection } from "prosemirror-state";
import { describe, expect, it } from "vitest";
import type { SendKey } from "../ui/sendKey";
import { pasteMarkdown } from "./clipboard";
import { enter, fit, tidy } from "./compose";
import { parseMarkdown, saveMarkdown } from "./markdown";
import { schema } from "./schema";

// A box that's written and sent (RichTextEditor.tsx): what Enter does, what's
// pasted into it, and the markdown it hands back.

// press is Enter (with shift or mod) on md, with the cursor at the end of
// the first place `at` is found: the markdown after, and whether it sent.
function press(md: string, at: string, send: SendKey, shift = false, mod = false) {
  const parsed = parseMarkdown(md);
  let state = EditorState.create({ doc: parsed.doc });
  state = state.apply(state.tr.setSelection(TextSelection.create(state.doc, end(state, at))));
  let sent: { other: boolean } | undefined;
  let next = state;
  enter({ send: () => send, onSend: (how) => (sent = how) }, shift, mod)(state, (tr) => (next = state.apply(tr)));
  return { md: saveMarkdown(next.doc, parsed).text, sent };
}

function end(state: EditorState, text: string): number {
  let at = -1;
  state.doc.descendants((node, pos) => {
    if (at < 0 && node.isText && node.text!.includes(text)) at = pos + node.text!.indexOf(text) + text.length;
    return at < 0;
  });
  if (at < 0) throw new Error(`no ${text}`);
  return at;
}

describe("Enter, where ⏎ sends", () => {
  it("sends from a paragraph", () => {
    expect(press("Hello", "Hello", "enter")).toEqual({ md: "Hello", sent: { other: false } });
  });

  it("goes on to a list's next item, ends the list from an empty one, then sends", () => {
    const parsed = parseMarkdown("- one");
    let state = EditorState.create({ doc: parsed.doc });
    state = state.apply(state.tr.setSelection(TextSelection.create(state.doc, end(state, "one"))));
    const sent: boolean[] = [];
    const key = enter({ send: () => "enter", onSend: () => sent.push(true) }, false, false);
    key(state, (tr) => (state = state.apply(tr)));
    expect(saveMarkdown(state.doc, parsed).text).toBe("- one\n-");
    key(state, (tr) => (state = state.apply(tr)));
    expect(saveMarkdown(state.doc, parsed).text).toBe("- one\n\n");
    expect(sent).toEqual([]);
    key(state, (tr) => (state = state.apply(tr)));
    expect(sent).toEqual([true]);
  });

  it("is a new line in a block of code, and out of it from an empty last line", () => {
    const line = press("```\nls\n```", "ls", "enter");
    expect(line.sent).toBeUndefined();
    expect(line.md).toBe("```\nls\n\n```");
  });

  it("makes ``` a block of code", () => {
    // Typed, not read: a paragraph of ```sh.
    const doc = schema.nodes.doc.create(null, schema.nodes.paragraph.create(null, schema.text("```sh")));
    let state = EditorState.create({ doc, selection: TextSelection.create(doc, 6) });
    let sent = false;
    enter({ send: () => "enter", onSend: () => (sent = true) }, false, false)(state, (tr) => (state = state.apply(tr)));
    expect(sent).toBe(false);
    expect(state.doc.firstChild!.type).toBe(schema.nodes.code_block);
    expect(state.doc.firstChild!.attrs.lang).toBe("sh");
  });

  it("is a new paragraph with shift held", () => {
    expect(press("Hello", "Hello", "enter", true)).toEqual({ md: "Hello\n\n", sent: undefined });
  });

  it("sends from a list with ⌘ held, the other way", () => {
    expect(press("- one", "one", "enter", false, true).sent).toEqual({ other: true });
  });
});

describe("Enter, where ⌘⏎ sends", () => {
  it("is a new paragraph, or the next item", () => {
    expect(press("Hello", "Hello", "mod+enter")).toEqual({ md: "Hello\n\n", sent: undefined });
    expect(press("- one", "one", "mod+enter").md).toBe("- one\n-");
  });

  it("sends with ⌘, even in a list, and the other way with ⇧ too", () => {
    expect(press("- one", "one", "mod+enter", false, true).sent).toEqual({ other: false });
    expect(press("Hello", "Hello", "mod+enter", true, true).sent).toEqual({ other: true });
  });
});

describe("leaving a block of code", () => {
  it("takes the empty last line off and goes on in a paragraph after", () => {
    const parsed = parseMarkdown("```\nls\n```");
    let state = EditorState.create({ doc: parsed.doc });
    // The code block's text is "ls": a new line, then Enter again.
    state = state.apply(state.tr.setSelection(TextSelection.create(state.doc, 3)));
    const on = { send: () => "enter" as const, onSend: () => {} };
    enter(on, false, false)(state, (tr) => (state = state.apply(tr)));
    enter(on, false, false)(state, (tr) => (state = state.apply(tr)));
    expect(state.doc.childCount).toBe(2);
    expect(state.doc.firstChild!.textContent).toBe("ls");
    expect(state.selection.$from.parent.type).toBe(schema.nodes.paragraph);
  });
});

describe("what's pasted", () => {
  const pasted = (text: string, f: Parameters<typeof fit>[1]) => {
    const doc = parseMarkdown("").doc;
    const slice = fit(pasteMarkdown(text, doc.resolve(1)), f);
    const state = EditorState.create({ doc });
    return saveMarkdown(state.apply(state.tr.replaceSelection(slice)).doc).text;
  };

  it("keeps what the box offers", () => {
    expect(pasted("- one\n- two\n\n`x` and **y**", { lists: true, marks: true })).toBe("- one\n- two\n\n`x` and **y**\n");
  });

  it("makes what it doesn't offer words", () => {
    expect(pasted("# Title\n\n- one\n- two\n\n**y**", {})).toBe("Title\n\none\n\ntwo\n\ny\n");
    expect(pasted("| a | b |\n| - | - |\n| 1 | 2 |", {})).toBe("a · b\n\n1 · 2\n");
  });

  it("keeps a block of code only where code is offered", () => {
    expect(pasted("```\nls -la\n```", { code: true })).toBe("```\nls -la\n```\n");
    expect(pasted("```\nls -la\n```", {})).toBe("ls -la\n");
  });

  it("pastes words into a line as they are", () => {
    const words = new Slice(schema.nodes.paragraph.create(null, schema.text("hi", [schema.marks.strong.create()])).content, 0, 0);
    expect(fit(words, {}).content.firstChild!.marks).toEqual([]);
    expect(fit(words, { marks: true }).content.firstChild!.marks).toHaveLength(1);
  });
});

describe("the markdown handed back", () => {
  it("escapes what would read as markdown the box doesn't offer", () => {
    // # typed where headings aren't offered stays words.
    const doc = schema.nodes.doc.create(null, schema.nodes.paragraph.create(null, schema.text("# not a heading")));
    expect(saveMarkdown(doc).text).toBe("\\# not a heading\n");
  });

  it("keeps a line break inside a paragraph", () => {
    const parsed = parseMarkdown("one\ntwo");
    expect(saveMarkdown(parsed.doc, parsed).text).toBe("one\ntwo");
  });

  it("keeps a space typed at a line's end or start as a space", () => {
    const doc = schema.nodes.doc.create(null, schema.nodes.paragraph.create(null, [schema.text("an "), schema.text("em", [schema.marks.em.create()]), schema.text(" ")]));
    expect(tidy(saveMarkdown(doc).text)).toBe("an *em* \n");
    expect(tidy("&#x20;lead\nkept \\&#x20;")).toBe(" lead\nkept \\&#x20;");
  });
});
