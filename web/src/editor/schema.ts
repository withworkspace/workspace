import { type MarkSpec, type NodeSpec, Schema } from "prosemirror-model";
import { codeBlockClass } from "../markdown/CodeBlock";
import css from "./MarkdownEditor.module.css";

// The editor's schema: markdown as the reader draws it (GitHub-flavoured:
// tables, task lists, strikethrough), a node for each kind of mdast node.
// What it has no node for (raw HTML, footnotes, link definitions,
// reference-style links) is kept as its source, in raw nodes, so a
// document never loses what it doesn't understand.
//
// Attributes are what markdown says and the reader shows: a list's
// looseness (spread) is kept because it decides whether its items are
// paragraphs, a fence's language because it decides the highlighting.
// How it was written (which bullet, which emphasis marker) isn't: that's
// the file's style, kept by the markdown bridge (markdown.ts).

const nodes: Record<string, NodeSpec> = {
  doc: { content: "block+" },
  paragraph: { content: "inline*", group: "block", toDOM: () => ["p", 0], parseDOM: [{ tag: "p" }] },
  heading: {
    attrs: { level: { default: 1 } },
    content: "inline*",
    group: "block",
    defining: true,
    toDOM: (n) => ["h" + n.attrs.level, 0],
    parseDOM: [1, 2, 3, 4, 5, 6].map((level) => ({ tag: "h" + level, attrs: { level } })),
  },
  blockquote: { content: "block+", group: "block", defining: true, toDOM: () => ["blockquote", 0], parseDOM: [{ tag: "blockquote" }] },
  horizontal_rule: { group: "block", toDOM: () => ["hr"], parseDOM: [{ tag: "hr" }] },
  code_block: {
    attrs: { lang: { default: "" }, meta: { default: "" } },
    content: "text*",
    marks: "",
    group: "block",
    code: true,
    defining: true,
    toDOM: () => ["pre", { class: codeBlockClass }, ["code", 0]],
    parseDOM: [{ tag: "pre", preserveWhitespace: "full" }],
  },
  bullet_list: {
    attrs: { spread: { default: false } },
    content: "list_item+",
    group: "block",
    toDOM: () => ["ul", 0],
    parseDOM: [{ tag: "ul" }],
  },
  ordered_list: {
    attrs: { start: { default: 1 }, spread: { default: false } },
    content: "list_item+",
    group: "block",
    toDOM: (n) => ["ol", n.attrs.start === 1 ? {} : { start: n.attrs.start }, 0],
    parseDOM: [{ tag: "ol", getAttrs: (el) => ({ start: Number((el as HTMLElement).getAttribute("start") ?? 1) }) }],
  },
  // checked is a task's box: null for an item that isn't a task.
  list_item: {
    attrs: { checked: { default: null }, spread: { default: false } },
    content: "block+",
    defining: true,
    toDOM: (n) => ["li", n.attrs.checked === null ? {} : { class: css.task, "data-checked": String(n.attrs.checked) }, 0],
    parseDOM: [{ tag: "li" }],
  },
  // A table's first row is its header (markdown has no other kind).
  // align is one entry per column: "left", "center", "right" or null.
  table: {
    attrs: { align: { default: [] } },
    content: "table_row+",
    group: "block",
    isolating: true,
    toDOM: () => ["table", ["tbody", 0]],
    parseDOM: [{ tag: "table" }],
  },
  table_row: { content: "(table_header | table_cell)+", toDOM: () => ["tr", 0], parseDOM: [{ tag: "tr" }] },
  table_header: { content: "inline*", isolating: true, toDOM: () => ["th", 0], parseDOM: [{ tag: "th" }] },
  table_cell: { content: "inline*", isolating: true, toDOM: () => ["td", 0], parseDOM: [{ tag: "td" }] },
  // Markdown with no node of its own, as it was written: raw HTML, a
  // footnote, a link definition.
  raw_block: {
    attrs: { text: {} },
    group: "block",
    atom: true,
    toDOM: (n) => ["pre", { class: css.rawBlock }, n.attrs.text],
  },
  // A cell of a page: what an agent wrote where you asked for it (>> a
  // table of the routes). id is its conversation (w7; "" until the server's
  // numbered it), v how many versions its agent has written into it, and
  // prompt what was first asked for. ask says it's a question (>?):
  // answered beside the page, it holds nothing until you ask for that. key
  // names one that's waiting for its
  // id, in this editor only. In the file it's its blocks between two
  // comments (cells.ts).
  cell: {
    attrs: { id: { default: "" }, v: { default: 0 }, prompt: { default: "" }, key: { default: "" }, ask: { default: false } },
    content: "block*",
    group: "block",
    isolating: true,
    defining: true,
    toDOM: (n) => ["div", { class: css.cell, "data-cell": n.attrs.id }, 0],
  },
  text: { group: "inline" },
  hard_break: { inline: true, group: "inline", selectable: false, toDOM: () => ["br"], parseDOM: [{ tag: "br" }] },
  image: {
    attrs: { src: {}, alt: { default: "" }, title: { default: null } },
    inline: true,
    group: "inline",
    draggable: true,
    toDOM: (n) => ["img", { src: n.attrs.src, alt: n.attrs.alt, title: n.attrs.title }],
  },
  raw_inline: {
    attrs: { text: {} },
    inline: true,
    group: "inline",
    atom: true,
    toDOM: (n) => ["code", { class: css.rawInline }, n.attrs.text],
  },
};

// Marks, outermost first when they nest the same way.
const marks: Record<string, MarkSpec> = {
  link: {
    attrs: { href: {}, title: { default: null } },
    inclusive: false,
    toDOM: (m) => ["a", { href: m.attrs.href, title: m.attrs.title }, 0],
    parseDOM: [{ tag: "a[href]", getAttrs: (el) => ({ href: (el as HTMLElement).getAttribute("href"), title: (el as HTMLElement).getAttribute("title") }) }],
  },
  strong: { toDOM: () => ["strong", 0], parseDOM: [{ tag: "strong" }, { tag: "b" }] },
  em: { toDOM: () => ["em", 0], parseDOM: [{ tag: "em" }, { tag: "i" }] },
  strike: { toDOM: () => ["del", 0], parseDOM: [{ tag: "del" }, { tag: "s" }] },
  code: { toDOM: () => ["code", 0], parseDOM: [{ tag: "code" }] },
};

export const schema = new Schema({ nodes, marks });
