import type { Element, ElementContent, Root, RootContent } from "hast";
import type { Nodes as MdNodes, Root as MdRoot } from "mdast";
import { type ComponentProps, createContext, memo, type ReactElement, type ReactNode, useContext, useEffect, useMemo, useRef, useState } from "react";
import ReactMarkdown, { type Components } from "react-markdown";
import type { Pluggable, PluggableList } from "unified";
import { Link, useLocation, useNavigate } from "react-router";
import { rawURL } from "../api";
import { highlightCode, useHighlighter } from "../review/highlight";
import { useRefs, useWanted } from "../refs/known";
import { RefTag } from "../refs/RefTag";
import type { Pinned } from "../refs/refs";
import { remarkRefs } from "../refs/remark";
import { type Place, resolveLink, slugger } from "./links";
import { Mermaid } from "./Mermaid";
import { syntax } from "./syntax";

// Markdown as a mission's artifacts, briefs, handoffs and reviews are
// written, and as a repo's documents are written for GitHub:
// GitHub-flavoured (tables, task lists, strikethrough), code highlighted,
// ```mermaid blocks drawn as diagrams, alerts (> [!NOTE]) in their
// colours, maths drawn, and front matter folded away at the top.
//
// A document that's a file in a repo says where it is (at): its links to
// other files open them, in the same worktree, and its images are read
// from beside it. A link to a heading (#how-it-lands) goes to it.
//
// What's rendered comes from agents and repos, so raw HTML stays off
// (react-markdown's default) and links keep to safe schemes: this page can
// make writes as you.
//
// What it names (a file, a pull request, a mission, a page) is drawn as a
// tag, with a card on hover (refs/): the same names a page's editor finds.
// A turn's pinned names (what they were when it was said) are drawn as
// they were meant.

//
// With lines, each block says which lines of the text it's from
// (data-ls, data-le), and the blocks a reader picks out (paragraphs,
// headings, list items, table rows, code and diagrams) are marked
// data-block: comments on a document are kept by those lines. What isn't
// the document's text (a code block's language, a diagram's labels) is
// marked data-skip.
//
// With headingLink, a reader's h2 and h3 headings show # when they're
// hovered, which gives it the heading's name (to copy a link to it).
export const Markdown = memo(function Markdown({ text, lines, at, pinned, headingLink }: { text: string; lines?: boolean; at?: Place; pinned?: Pinned[]; headingLink?: (anchor: string) => void }) {
  useHighlighter();
  const box = useRef<HTMLDivElement>(null);
  // To the heading the address names, when the document opens or the
  // address changes: not one you've just clicked a link to, which has gone
  // there already.
  const location = useLocation();
  const hash = at && !/^#L\d/.test(location.hash) ? location.hash.slice(1) : "";
  const clicked = (location.state as { clicked?: boolean } | null)?.clicked;
  const refs = useRefs();
  useWanted(text);
  const plugins = useMemo<PluggableList>(() => [...syntax, foldFrontMatter, displayMaths, ...(refs ? [[remarkRefs, { known: refs.known, links: !at, pinned }] as Pluggable] : [])], [refs, at, pinned]);
  useEffect(() => {
    if (hash && !clicked && box.current) toHeading(box.current, decodeURIComponent(hash));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [hash, at?.api, at?.path]);
  return (
    <div className="markdown" ref={box}>
      <PlaceContext.Provider value={at}>
        <HeadingLinkContext.Provider value={headingLink}>
          <ReactMarkdown remarkPlugins={plugins} rehypePlugins={lines ? [anchors, alerts, sourceLines] : [anchors, alerts]} components={components}>
            {text}
          </ReactMarkdown>
        </HeadingLinkContext.Provider>
      </PlaceContext.Provider>
    </div>
  );
});

const PlaceContext = createContext<Place | undefined>(undefined);
const HeadingLinkContext = createContext<((anchor: string) => void) | undefined>(undefined);

// toHeading scrolls to a heading by the name GitHub gives it. It says
// whether there's one.
function toHeading(root: Element | HTMLElement, id: string): boolean {
  const h = (root as HTMLElement).querySelector(`[data-anchor="${CSS.escape(id.toLowerCase())}"]`);
  h?.scrollIntoView({ block: "start" });
  return !!h;
}

// Front matter is drawn as a code block of its own kind, which Block folds
// away.
function foldFrontMatter() {
  return (tree: MdRoot) => {
    const first = tree.children[0];
    if (first?.type === "yaml") tree.children[0] = { type: "code", lang: "front-matter", value: first.value, position: first.position };
  };
}

// Maths between double dollars in a line ($$x$$) is set out as maths on
// lines of its own is, as GitHub does.
function displayMaths() {
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
function anchors() {
  return (tree: Root) => {
    const slug = slugger();
    const visit = (node: Root | RootContent) => {
      if (node.type === "element" && /^h[1-6]$/.test(node.tagName)) node.properties.dataAnchor = slug(textOf(node));
      if ("children" in node) node.children.forEach(visit);
    };
    visit(tree);
  };
}

const alertKinds: Record<string, string> = { note: "Note", tip: "Tip", important: "Important", warning: "Warning", caution: "Caution" };

// alerts draws a quote that starts with [!NOTE] (or TIP, IMPORTANT,
// WARNING, CAUTION) on a line of its own as GitHub does: an alert of that
// kind, titled, without the marker.
function alerts() {
  const visit = (node: Root | RootContent) => {
    if ("children" in node) node.children.forEach(visit);
    if (node.type !== "element" || node.tagName !== "blockquote") return;
    const p = node.children.find((c): c is Element => c.type === "element");
    const first = p?.tagName === "p" ? p.children[0] : undefined;
    const m = first?.type === "text" ? /^\[!(note|tip|important|warning|caution)\][ \t]*(\n|$)/i.exec(first.value) : null;
    if (!p || first?.type !== "text" || !m) return;
    const kind = m[1].toLowerCase();
    first.value = first.value.slice(m[0].length);
    // The marker alone in its paragraph (a blank line after it) leaves nothing.
    if (!first.value) p.children.shift();
    if (p.children[0]?.type === "element" && p.children[0].tagName === "br") p.children.shift();
    if (!p.children.length) node.children.splice(node.children.indexOf(p), 1);
    node.properties.className = ["alert", `alert-${kind}`];
    node.children.unshift({ type: "element", tagName: "p", properties: { className: ["alert-title"], dataSkip: "" }, children: [{ type: "text", value: alertKinds[kind] }] });
  };
  return (tree: Root) => visit(tree);
}

// Blocks with lines; those a reader picks out (list items only when their
// words aren't in paragraphs of their own).
const blocks = new Set(["p", "h1", "h2", "h3", "h4", "h5", "h6", "pre", "li", "tr", "blockquote", "table", "ul", "ol"]);
const picked = new Set(["p", "h1", "h2", "h3", "h4", "h5", "h6", "pre", "tr"]);

function sourceLines() {
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

// dataOf is a block's data- attributes, to keep on what draws it.
function dataOf(props: object): Record<string, unknown> {
  return Object.fromEntries(Object.entries(props).filter(([k]) => k.startsWith("data-")));
}

const components: Components = {
  pre: ({ children, node: _node, ...props }) => <Block data={dataOf(props)}>{children}</Block>,
  h2: ({ node: _node, ...props }) => <Heading level={2} {...props} />,
  h3: ({ node: _node, ...props }) => <Heading level={3} {...props} />,
  a: ({ href, children, ...props }) => {
    const ref = (props as Record<string, unknown>)["data-ref"];
    return typeof ref === "string" ? (
      <RefTag refKey={ref} words>
        {children}
      </RefTag>
    ) : (
      <DocLink href={href}>{children}</DocLink>
    );
  },
  // A ref found in the words (refs/remark.ts).
  span: ({ node: _node, children, ...props }) => {
    const ref = (props as Record<string, unknown>)["data-ref"];
    return typeof ref === "string" ? <RefTag refKey={ref}>{children}</RefTag> : <span {...props}>{children}</span>;
  },
  img: ({ src, alt, title }) => <DocImage src={src} alt={alt} title={title} />,
  // Maths in a line; any other code as it is.
  code: ({ node: _node, className, children, ...props }) =>
    /\bmath-inline\b/.test(className ?? "") ? (
      <Maths code={String(children ?? "")} display={/\bmath-display\b/.test(className ?? "")} inline />
    ) : (
      <code className={className} {...props}>
        {children}
      </code>
    ),
  // A task list's boxes show, but can't be ticked here.
  input: ({ type, checked }) => (type === "checkbox" ? <input type="checkbox" checked={!!checked} readOnly disabled /> : null),
};

// Heading is an h2 or h3. Where the reader links to headings it has a #,
// drawn by CSS so it isn't among the heading's words, and marked as not
// the document's text.
function Heading({ level, children, ...props }: { level: 2 | 3; children?: ReactNode } & ComponentProps<"h2">) {
  const link = useContext(HeadingLinkContext);
  const H = level === 2 ? "h2" : "h3";
  const anchor = (props as Record<string, unknown>)["data-anchor"];
  if (!link || typeof anchor !== "string") return <H {...props}>{children}</H>;
  return (
    <H {...props} className={props.className ? `${props.className} linked` : "linked"}>
      <button type="button" className="heading-link" data-skip="" title="Copy a link to this section" aria-label="Copy a link to this section" onClick={() => link(anchor)} />
      {children}
    </H>
  );
}

// DocLink is a link in a document. One to another file opens it in the
// same worktree, where the document says where it is; one to a heading
// goes there; one that leaves the repo has nowhere to go.
function DocLink({ href, children }: { href?: string; children?: ReactNode }) {
  const at = useContext(PlaceContext);
  const location = useLocation();
  const navigate = useNavigate();
  if (!href) return <a>{children}</a>;
  const t = resolveLink(at?.path ?? "", href);
  if (t.kind === "external") {
    return (
      <a href={href} target="_blank" rel="noopener noreferrer">
        {children}
      </a>
    );
  }
  if (t.kind === "heading") {
    return (
      <a
        href={`#${t.id}`}
        onClick={(e) => {
          if (e.metaKey || e.ctrlKey || e.shiftKey) return;
          e.preventDefault();
          const found = toHeading(e.currentTarget.closest(".markdown") ?? document.body, t.id);
          // The address names the heading, for a document that has one.
          if (found && at) navigate({ search: location.search, hash: `#${t.id}` }, { replace: true, state: { clicked: true } });
        }}
      >
        {children}
      </a>
    );
  }
  // Read without knowing where it is (a reply, a brief), a path is only words.
  if (!at) return <a href={href}>{children}</a>;
  if (t.kind === "outside" || !t.path) {
    return (
      <span className="dead-link" title="This leads outside the repo">
        {children}
      </span>
    );
  }
  return <Link to={at.to(t.path) + (t.hash ? `#${t.hash}` : "")}>{children}</Link>;
}

// DocImage is an image in a document: one that's a file in the repo is
// read from there.
function DocImage({ src, alt, title }: { src?: string; alt?: string; title?: string }) {
  const at = useContext(PlaceContext);
  if (!src) return null;
  const t = resolveLink(at?.path ?? "", src);
  if (t.kind === "external") return <img src={src} alt={alt ?? ""} title={title} loading="lazy" />;
  if (!at || t.kind !== "file" || !t.path) return <span className="dead-link" title="An image this can't find">{alt || src}</span>;
  return <img src={rawURL(at.api, t.path)} alt={alt ?? ""} title={title} loading="lazy" />;
}

// Block is a fenced code block: a diagram for mermaid, maths for math
// ($$…$$ too), front matter folded, else code.
function Block({ children, data }: { children?: ReactNode; data: Record<string, unknown> }) {
  const code = children as ReactElement<{ className?: string; children?: ReactNode }> | undefined;
  const lang = /language-([\w+-]+)/.exec(code?.props?.className ?? "")?.[1];
  const text = String(code?.props?.children ?? "").replace(/\n$/, "");
  if (lang === "mermaid") return <Mermaid code={text} data={data} />;
  if (lang === "math") return <Maths code={text} display data={data} />;
  if (lang === "front-matter") {
    const keys = [...text.matchAll(/^([A-Za-z_][\w-]*):/gm)].map((m) => m[1]);
    return (
      <details className="front-matter" {...data} data-skip="">
        <summary>
          Front matter{keys.length > 0 && <span className="dim"> · {keys.join(", ")}</span>}
        </summary>
        <pre className="code-block">
          <code dangerouslySetInnerHTML={{ __html: highlightCode(text, "yaml") }} />
        </pre>
      </details>
    );
  }
  return (
    <pre className="code-block" {...data}>
      {lang && (
        <span className="code-lang" data-skip="">
          {lang}
        </span>
      )}
      <code dangerouslySetInnerHTML={{ __html: highlightCode(text, lang) }} />
    </pre>
  );
}

type Katex = typeof import("katex").default;
let katex: Katex | undefined;
let katexLoading: Promise<Katex> | undefined;

// loadKatex loads KaTeX, and its styles, the first time a document has
// maths.
function loadKatex(): Promise<Katex> {
  katexLoading ??= Promise.all([import("katex"), import("katex/dist/katex.min.css")]).then(([m]) => (katex = m.default));
  return katexLoading;
}

// drawn is maths as KaTeX draws it. What it can't read shows as written,
// in its error colour, not as a failure.
const drawn = (k: Katex, code: string, display: boolean) => k.renderToString(code, { displayMode: display, throwOnError: false });

// Maths is maths drawn: in a line (inline), or on lines of its own, and
// set out large (display) or to fit the line. As written until KaTeX has
// loaded. It isn't the document's words (a comment is kept by what it
// quotes, and this reads differently drawn).
function Maths({ code, display = false, inline = false, data = {} }: { code: string; display?: boolean; inline?: boolean; data?: Record<string, unknown> }) {
  const [html, setHtml] = useState(() => (katex ? drawn(katex, code, display) : undefined));
  useEffect(() => {
    let live = true;
    loadKatex().then(
      (k) => live && setHtml(drawn(k, code, display)),
      () => {},
    );
    return () => {
      live = false;
    };
  }, [code, display]);
  if (html === undefined) {
    return inline ? (
      <code data-skip="">{code}</code>
    ) : (
      <pre className="code-block maths-loading" {...data} data-skip="">
        {code}
      </pre>
    );
  }
  return inline ? <span className="maths-inline" data-skip="" dangerouslySetInnerHTML={{ __html: html }} /> : <div className="maths" {...data} data-skip="" dangerouslySetInnerHTML={{ __html: html }} />;
}
