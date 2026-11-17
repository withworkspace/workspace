import type { Nodes, Parent, PhrasingContent, Root } from "mdast";
import { findAll, type Known, type PageRef, type Pinned, parseAddress, refAt, refKey, repoThenPath } from "./refs";

// remarkRefs finds refs in markdown as it's read, and marks each for the
// reader to draw as a tag (RefTag): a name in the words, a name in
// backticks, a repo's name with a path in it after in backticks (demo
// `config/app.json`), and (with links) a link whose address is a name or a path on
// this disk, as an agent writes one ([the auth config](/Users/…/a.json:15)).
// Nothing in the text changes: the words a ref is drawn from are its own.
// Not in code blocks, headings or a link's words, as in the editor.
//
// links is off for a document read where it is (a file in a repo), whose
// relative links are to its own files and are followed as such; an
// address there (one of the workspace's repos on GitHub, ws:) is still
// read as what it names, and stays in ws. pinned are a turn's
// names as they were when it was said: drawn as they were meant.
export function remarkRefs({ known, links, pinned }: { known: Known; links: boolean; pinned?: Pinned[] }) {
  const at = (text: string) => pinned?.find((p) => p.said === text.trim())?.ref ?? refAt(text, known);
  const then = (before: string, code: string) => repoThenPath(before, code, known);
  return (tree: Root) => visit(tree, { at, address: (url) => parseAddress(url, known), find: (text) => findAll(text, known, pinned), then }, links);
}

interface Finder {
  at(text: string): PageRef | undefined;
  address(url: string): PageRef | undefined;
  find(text: string): ReturnType<typeof findAll>;
  then(before: string, code: string): ReturnType<typeof repoThenPath>;
}

// A ref drawn in place: its node, named for the reader (span with
// data-ref), around what it's drawn from.
const marked = (ref: PageRef, children: PhrasingContent[]) =>
  ({ type: "wsRef", data: { hName: "span", hProperties: { dataRef: refKey(ref) } }, children }) as unknown as PhrasingContent;

// decoded is an address as it was written (markdown escapes its spaces).
function decoded(url: string) {
  try {
    return decodeURI(url);
  } catch {
    return url;
  }
}

function visit(node: Parent, finder: Finder, links: boolean) {
  for (let i = 0; i < node.children.length; i++) {
    const c = node.children[i] as Nodes;
    switch (c.type) {
      case "code":
      case "heading":
      case "html":
      case "math":
      case "inlineMath":
        break;
      case "link": {
        const url = decoded(c.url);
        const ref = links ? finder.at(url) : /^(https?:|ws:)/.test(url) ? finder.address(url) : undefined;
        if (ref) c.data = { ...c.data, hProperties: { ...c.data?.hProperties, dataRef: refKey(ref) } };
        break;
      }
      case "inlineCode": {
        const ref = finder.at(c.value);
        if (ref) node.children[i] = marked(ref, [c]);
        break;
      }
      case "text": {
        // A repo's name with a path in it after, in backticks: the file,
        // drawn from the name to the path's end.
        const next = node.children[i + 1] as Nodes | undefined;
        const then = next?.type === "inlineCode" ? finder.then(c.value, next.value) : undefined;
        const value = then ? c.value.slice(0, then.from) : c.value;
        const found = finder.find(value);
        if (!found.length && !then) break;
        const parts: PhrasingContent[] = [];
        let at = 0;
        for (const f of found) {
          if (f.from > at) parts.push({ type: "text", value: value.slice(at, f.from) });
          parts.push(marked(f.ref, [{ type: "text", value: value.slice(f.from, f.to) }]));
          at = f.to;
        }
        if (at < value.length) parts.push({ type: "text", value: value.slice(at) });
        if (then) parts.push(marked(then.ref, [{ type: "text", value: c.value.slice(then.from) }, next as PhrasingContent]));
        node.children.splice(i, then ? 2 : 1, ...(parts as typeof node.children));
        i += parts.length - 1;
        break;
      }
      default:
        if ("children" in c) visit(c, finder, links);
    }
  }
}
