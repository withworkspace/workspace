import type { BlockContent, Delete, Emphasis, Link, ListItem, Nodes, PhrasingContent, Root, RootContent, Strong, Table, TableCell, TableRow } from "mdast";
import type { Mark, Node as PMNode } from "prosemirror-model";
import { schema } from "./schema";

// Converting between mdast (markdown's syntax tree, as remark parses it)
// and the editor's documents.

// Origin is where a block came from in the markdown it was parsed from:
// its start and end, as offsets into the text.
export interface Origin {
  start: number;
  end: number;
}

const { nodes: N, marks: M } = schema;

// A cell in a file is its blocks between two comments: the first names its
// conversation, how many versions its agent has written into it, and what
// was asked for.
//
//   <!-- ws:cell w7 v2 | a table of the existing routes -->
//   …
//   <!-- /ws:cell -->
//
// Comments, so the file reads as plain markdown to anything else.
const cellOpen = /^<!--\s*ws:cell\s+([A-Za-z0-9-]+)\s+v(\d+)(\s+ask)?\s*(?:\|\s*([^]*?))?\s*-->$/;
const cellClose = /^<!--\s*\/ws:cell\s*-->$/;

// cellMarker is a cell's opening comment. What was asked for is kept on
// one line, without what would end the comment early.
export function cellMarker(attrs: { id: string; v: number; prompt: string; ask?: boolean }): string {
  const prompt = String(attrs.prompt).replace(/\s+/g, " ").replace(/--+>/g, "→").trim();
  return `<!-- ws:cell ${attrs.id} v${attrs.v}${attrs.ask ? " ask" : ""}${prompt ? ` | ${prompt}` : ""} -->`;
}

const cellEnd = "<!-- /ws:cell -->";

// lf is text with its line breaks as \n, whatever the file's: they're put
// back as the file has them when it's written.
const lf = (text: string) => text.replace(/\r\n?/g, "\n");

// fromMdast builds the document for a markdown file's tree, noting where
// each block (at every depth: list items, table rows, a quote's
// paragraphs) came from.
export function fromMdast(tree: Root, source: string, origins: WeakMap<PMNode, Origin>): PMNode {
  const at = (md: Nodes) => {
    const p = md.position;
    return p && p.start.offset !== undefined && p.end.offset !== undefined ? { start: p.start.offset, end: p.end.offset } : undefined;
  };
  const kept = (node: PMNode, md: Nodes) => {
    const o = at(md);
    if (!o) return node;
    // A container ends where its last block does. (remark ends a list in
    // a quote at the start of what follows it, blank line and all.)
    const last = node.lastChild && !node.isTextblock ? origins.get(node.lastChild) : undefined;
    if (last && last.end < o.end) o.end = last.end;
    // A task's paragraph starts after its box. (remark says so only when
    // its words start with plain text, not `code` or **bold**.)
    if (md.type === "listItem" && typeof md.checked === "boolean" && node.firstChild?.type === N.paragraph) {
      const p = origins.get(node.firstChild);
      const box = p && !/\[[ xX]\]/.test(source.slice(o.start, p.start)) ? /^\[[ xX]\][ \t]?/.exec(source.slice(p.start, p.end)) : null;
      if (p && box) p.start += box[0].length;
    }
    origins.set(node, o);
    return node;
  };
  // raw is a node's markdown as written. Nested, its lines after the
  // first lose the container's prefix (a list item's indent, a quote's >),
  // so it reads as it would on its own.
  const raw = (md: Nodes): string => {
    const o = at(md);
    if (!o) return "";
    const text = lf(source.slice(o.start, o.end));
    const indent = o.start - (source.lastIndexOf("\n", o.start - 1) + 1);
    if (!indent) return text;
    return text
      .split("\n")
      .map((line, i) => (i > 0 && /^[ \t>]*$/.test(line.slice(0, indent)) ? line.slice(indent) : line))
      .join("\n");
  };

  const inline = (list: PhrasingContent[], marks: readonly Mark[], out: PMNode[]) => {
    for (const n of list) {
      switch (n.type) {
        case "text":
          // A paragraph's line breaks are spaces, as the reader shows them:
          // where its lines broke is kept by the writer (rewrap), not here.
          if (n.value) out.push(schema.text(lf(n.value).replace(/\n/g, " "), marks));
          break;
        case "emphasis":
          inline(n.children, M.em.create().addToSet(marks), out);
          break;
        case "strong":
          inline(n.children, M.strong.create().addToSet(marks), out);
          break;
        case "delete":
          inline(n.children, M.strike.create().addToSet(marks), out);
          break;
        case "inlineCode":
          if (n.value) out.push(schema.text(lf(n.value).replace(/\n/g, " "), M.code.create().addToSet(marks)));
          else out.push(N.raw_inline.create({ text: raw(n) }, null, marks));
          break;
        case "link":
          // A link with no text has nothing to carry its mark.
          if (n.children.length) inline(n.children, M.link.create({ href: n.url, title: n.title ?? null }).addToSet(marks), out);
          else out.push(N.raw_inline.create({ text: raw(n) }, null, marks));
          break;
        case "image":
          out.push(N.image.create({ src: n.url, alt: n.alt ?? "", title: n.title ?? null }, null, marks));
          break;
        case "break":
          out.push(N.hard_break.create(null, null, marks));
          break;
        case "html":
          out.push(N.raw_inline.create({ text: lf(n.value) }, null, marks));
          break;
        default:
          // A reference-style link or image, a footnote reference.
          out.push(N.raw_inline.create({ text: raw(n) }, null, marks));
      }
    }
    return out;
  };

  // blocks are a container's blocks. At the top of a document, those
  // between a cell's comments are the cell's.
  const blocks = (list: RootContent[], top = false): PMNode[] => {
    if (!top) return list.map(block);
    const out: PMNode[] = [];
    for (let i = 0; i < list.length; i++) {
      const open = list[i].type === "html" ? cellOpen.exec(lf((list[i] as { value: string }).value).trim()) : null;
      const end = open ? list.findIndex((n, j) => j > i && n.type === "html" && (cellClose.test(lf(n.value).trim()) || cellOpen.test(lf(n.value).trim()))) : -1;
      const from = at(list[i]);
      const to = end > i ? at(list[end]) : undefined;
      // One that isn't closed (or opens another first) stays as written.
      if (!open || end < 0 || !cellClose.test(lf((list[end] as { value: string }).value).trim()) || !from || !to) {
        out.push(block(list[i]));
        continue;
      }
      const cell = N.cell.create({ id: open[1], v: Number(open[2]), ask: !!open[3], prompt: open[4] ?? "" }, list.slice(i + 1, end).map(block));
      origins.set(cell, { start: from.start, end: to.end });
      out.push(cell);
      i = end;
    }
    return out;
  };
  const block = (n: RootContent): PMNode => {
    switch (n.type) {
      case "paragraph":
        return kept(N.paragraph.create(null, inline(n.children, [], [])), n);
      case "heading":
        return kept(N.heading.create({ level: n.depth }, inline(n.children, [], [])), n);
      case "thematicBreak":
        return kept(N.horizontal_rule.create(), n);
      case "blockquote":
        return kept(N.blockquote.create(null, n.children.length ? blocks(n.children) : [N.paragraph.create()]), n);
      case "code":
        return kept(N.code_block.create({ lang: n.lang ?? "", meta: n.meta ?? "" }, n.value ? schema.text(lf(n.value)) : null), n);
      case "list":
        return kept(
          (n.ordered ? N.ordered_list : N.bullet_list).create(
            n.ordered ? { start: n.start ?? 1, spread: !!n.spread } : { spread: !!n.spread },
            n.children.map((item) => block(item)),
          ),
          n,
        );
      case "listItem":
        return kept(
          N.list_item.create({ checked: n.checked ?? null, spread: !!n.spread }, n.children.length ? blocks(n.children) : [N.paragraph.create()]),
          n,
        );
      case "table":
        return kept(
          N.table.create(
            { align: n.align ?? [] },
            n.children.map((row, i) =>
              kept(
                N.table_row.create(
                  null,
                  row.children.map((cell) => (i === 0 ? N.table_header : N.table_cell).create(null, inline(cell.children, [], []))),
                ),
                row,
              ),
            ),
          ),
          n,
        );
      case "html":
        return kept(N.raw_block.create({ text: lf(n.value) }), n);
      default:
        // A footnote's definition, a link's: kept as written.
        return kept(N.raw_block.create({ text: raw(n) }), n);
    }
  };

  const content = blocks(tree.children, true);
  return N.doc.create(null, content.length ? content : [N.paragraph.create()]);
}

// toMdastBlocks is a block as mdast, for writing as markdown: one node,
// but for a cell, which is its comments with its blocks between them.
export function toMdastBlocks(node: PMNode): RootContent[] {
  if (node.type !== N.cell) return [toMdast(node) as RootContent];
  const out: RootContent[] = [{ type: "html", value: cellMarker(node.attrs as { id: string; v: number; prompt: string; ask?: boolean }) }];
  node.forEach((c) => out.push(...toMdastBlocks(c)));
  out.push({ type: "html", value: cellEnd });
  return out;
}

// toMdast is a block (or the document) as mdast, for writing as markdown.
export function toMdast(node: PMNode): Root | RootContent {
  const children = () => {
    const out: BlockContent[] = [];
    node.forEach((c) => out.push(...(toMdastBlocks(c) as BlockContent[])));
    return out;
  };
  switch (node.type.name) {
    case "doc":
      return { type: "root", children: children() };
    case "paragraph":
      return { type: "paragraph", children: phrasing(node) };
    case "heading":
      return { type: "heading", depth: node.attrs.level, children: phrasing(node) };
    case "blockquote":
      return { type: "blockquote", children: children() };
    case "horizontal_rule":
      return { type: "thematicBreak" };
    case "code_block":
      return { type: "code", lang: node.attrs.lang || null, meta: node.attrs.meta || null, value: node.textContent };
    case "bullet_list":
    case "ordered_list": {
      const ordered = node.type === N.ordered_list;
      const items: ListItem[] = [];
      node.forEach((c) => items.push(toMdast(c) as ListItem));
      return { type: "list", ordered, start: ordered ? node.attrs.start : null, spread: node.attrs.spread, children: items };
    }
    case "list_item": {
      // An item that's only an empty paragraph is an empty item.
      const empty = node.childCount === 1 && node.firstChild!.type === N.paragraph && node.firstChild!.content.size === 0;
      return { type: "listItem", checked: node.attrs.checked, spread: node.attrs.spread, children: empty ? [] : children() };
    }
    case "table": {
      const rows: TableRow[] = [];
      node.forEach((row) => {
        const cells: TableCell[] = [];
        row.forEach((cell) => cells.push({ type: "tableCell", children: phrasing(cell) }));
        rows.push({ type: "tableRow", children: cells });
      });
      return { type: "table", align: node.attrs.align, children: rows } satisfies Table;
    }
    case "raw_block":
      return { type: "html", value: node.attrs.text };
  }
  throw new Error(`no markdown for ${node.type.name}`);
}

// phrasing is a textblock's inline content as mdast: ProseMirror keeps
// marks flat on each piece of text, markdown nests them. A mark that runs
// further opens outside one that stops sooner, so it's written once:
// **bold *and both* bold**, not **bold** ***and both*** **bold**.
function phrasing(parent: PMNode): PhrasingContent[] {
  const items: PMNode[] = [];
  parent.forEach((c) => items.push(c));
  const root: { children: PhrasingContent[] } = { children: [] };
  const open: { mark: Mark; el: Inline }[] = [];
  const top = () => (open.length ? open[open.length - 1].el : root);
  const runs = (mark: Mark, from: number) => {
    let n = 0;
    while (from + n < items.length && mark.isInSet(items[from + n].marks)) n++;
    return n;
  };
  items.forEach((item, i) => {
    const marks = item.marks.filter((m) => m.type !== M.code);
    let keep = 0;
    while (keep < open.length && open[keep].mark.isInSet(marks)) keep++;
    open.length = keep;
    const opening = marks.filter((m) => !open.some((o) => o.mark.eq(m))).sort((a, b) => runs(b, i) - runs(a, i));
    for (const mark of opening) {
      const el = markNode(mark);
      top().children.push(el);
      open.push({ mark, el });
    }
    top().children.push(leaf(item));
  });
  return root.children;
}

// Inline is a mark as mdast: what holds the text it's on.
type Inline = Strong | Emphasis | Delete | Link;

function markNode(mark: Mark): Inline {
  switch (mark.type.name) {
    case "strong":
      return { type: "strong", children: [] };
    case "em":
      return { type: "emphasis", children: [] };
    case "strike":
      return { type: "delete", children: [] };
    case "link":
      return { type: "link", url: mark.attrs.href, title: mark.attrs.title, children: [] } satisfies Link;
  }
  throw new Error(`no markdown for the ${mark.type.name} mark`);
}

function leaf(node: PMNode): PhrasingContent {
  switch (node.type.name) {
    case "text":
      return M.code.isInSet(node.marks) ? { type: "inlineCode", value: node.text! } : { type: "text", value: node.text! };
    case "hard_break":
      return { type: "break" };
    case "image":
      return { type: "image", url: node.attrs.src, alt: node.attrs.alt, title: node.attrs.title };
    case "raw_inline":
      return { type: "html", value: node.attrs.text };
  }
  throw new Error(`no markdown for ${node.type.name}`);
}
