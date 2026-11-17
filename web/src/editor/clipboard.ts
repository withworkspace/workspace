import { Fragment, type Node as PMNode, type ResolvedPos, Slice } from "prosemirror-model";
import { parseMarkdown, saveMarkdown } from "./markdown";
import { schema } from "./schema";

// Text pasted into a document is read as markdown: it's what agents
// write, and what you copy from a terminal or an editor. Text copied out
// of one is written as markdown, so it pastes into an agent's prompt (or
// a file) as it reads.

const { nodes: N } = schema;

// pasteMarkdown is pasted text as a slice of document. Into a code block
// it's the text as it is. A paragraph at either end joins the one it's
// pasted into; a list or a heading stays one.
export function pasteMarkdown(text: string, $context: ResolvedPos): Slice {
  if ($context.parent.type.spec.code) return text ? new Slice(Fragment.from(schema.text(text)), 0, 0) : Slice.empty;
  const doc = parseMarkdown(text).doc;
  const open = (n: PMNode | null) => (n?.type === N.paragraph ? 1 : 0);
  return new Slice(doc.content, open(doc.firstChild), open(doc.lastChild));
}

// copyMarkdown is a copied slice as markdown: a list's items in a list, a
// table's rows in a table; words from a paragraph as they'd be written.
export function copyMarkdown(slice: Slice): string {
  const kids: PMNode[] = [];
  slice.content.forEach((n) => kids.push(n));
  let content = slice.content;
  if (kids.length && kids.every((n) => n.type === N.list_item)) content = Fragment.from(N.bullet_list.create(null, content));
  if (kids.length && kids.every((n) => n.type === N.table_row)) content = Fragment.from(N.table.create(null, content));
  if (kids.length && kids.every((n) => n.isInline)) content = Fragment.from(N.paragraph.create(null, content));
  try {
    const doc = N.doc.create(null, content);
    doc.check();
    return saveMarkdown(doc).text.replace(/\n+$/, "");
  } catch {
    return slice.content.textBetween(0, slice.content.size, "\n\n");
  }
}
