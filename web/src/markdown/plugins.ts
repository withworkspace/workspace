import type { Element, ElementContent, Root, RootContent } from "hast";
import type { Nodes as MdNodes, Root as MdRoot } from "mdast";
import { slugger } from "./links";
import css from "./Markdown.module.css";

// What the reader does to a document between reading it and drawing it
// (Markdown.tsx): remark's, on its markdown, and rehype's, on its HTML.

// foldFrontMatter: front matter is drawn as a code block of its own kind,
// which MarkdownBlock folds away.
export function foldFrontMatter() {
  return (tree: MdRoot) => {
    const first = tree.children[0];
    if (first?.type === "yaml") tree.children[0] = { type: "code", lang: "front-matter", value: first.value, position: first.position };
  };
}

// displayMaths: maths between double dollars in a line ($$x$$) is set out
// as maths on lines of its own is, as GitHub does.
export function displayMaths() {
  return (tree: MdRoot, file: { toString(): string }) => {
    const source = String(file);
    const visit = (n: MdNodes) => {
      if (n.type === "inlineMath" && source.startsWith("$$", n.position?.start.offset ?? -1)) {
        n.data = { ...n.data, hProperties: { className: ["language-math", "math-inline", "math-display"] } };
      }
      if ("children" in n) n.children.forEach(visit);
    };
    visit(tree);
  };
}

const textOf = (n: ElementContent | Root): string => (n.type === "text" ? n.value : "children" in n ? n.children.map((c) => textOf(c as ElementContent)).join("") : "");

// anchors names each heading as GitHub does (data-anchor, not an id: a
// document's headings mustn't be mistaken for the page's own parts).
export function anchors() {
  return (tree: Root) => {
    const slug = slugger();
    const visit = (node: Root | RootContent) => {
      if (node.type === "element" && /^h[1-6]$/.test(node.tagName)) node.properties.dataAnchor = slug(textOf(node));
      if ("children" in node) node.children.forEach(visit);
    };
    visit(tree);
  };
}

const alertKinds: Record<string, { title: string; className?: string }> = {
  note: { title: "Note" },
  tip: { title: "Tip", className: css.tip },
  important: { title: "Important", className: css.important },
  warning: { title: "Warning", className: css.warning },
  caution: { title: "Caution", className: css.caution },
};

// alerts draws a quote that starts with [!NOTE] (or TIP, IMPORTANT,
// WARNING, CAUTION) on a line of its own as GitHub does: an alert of that
// kind, titled, without the marker.
export function alerts() {
  const visit = (node: Root | RootContent) => {
    if ("children" in node) node.children.forEach(visit);
    if (node.type !== "element" || node.tagName !== "blockquote") return;
    const p = node.children.find((c): c is Element => c.type === "element");
    const first = p?.tagName === "p" ? p.children[0] : undefined;
    const m = first?.type === "text" ? /^\[!(note|tip|important|warning|caution)\][ \t]*(\n|$)/i.exec(first.value) : null;
    if (!p || first?.type !== "text" || !m) return;
    const kind = alertKinds[m[1].toLowerCase()];
    first.value = first.value.slice(m[0].length);
    // The marker alone in its paragraph (a blank line after it) leaves nothing.
    if (!first.value) p.children.shift();
    if (p.children[0]?.type === "element" && p.children[0].tagName === "br") p.children.shift();
    if (!p.children.length) node.children.splice(node.children.indexOf(p), 1);
    node.properties.className = kind.className ? [css.alert, kind.className] : [css.alert];
    node.children.unshift({ type: "element", tagName: "p", properties: { className: [css.alertTitle], dataSkip: "" }, children: [{ type: "text", value: kind.title }] });
  };
  return (tree: Root) => visit(tree);
}

// Blocks with lines; those a reader picks out (list items only when their
// words aren't in paragraphs of their own).
const blocks = new Set(["p", "h1", "h2", "h3", "h4", "h5", "h6", "pre", "li", "tr", "blockquote", "table", "ul", "ol"]);
const picked = new Set(["p", "h1", "h2", "h3", "h4", "h5", "h6", "pre", "tr"]);

// sourceLines says which lines of the text each block is from (data-ls,
// data-le), and marks those a reader picks out data-block.
export function sourceLines() {
  const visit = (node: Root | RootContent) => {
    if (node.type === "element" && node.position && blocks.has(node.tagName)) {
      node.properties.dataLs = node.position.start.line;
      node.properties.dataLe = node.position.end.line;
      if (picked.has(node.tagName) || (node.tagName === "li" && !node.children.some((c) => c.type === "element" && c.tagName === "p"))) {
        node.properties.dataBlock = "";
      }
    }
    if ("children" in node) node.children.forEach(visit);
  };
  return (tree: Root) => visit(tree);
}
