import { memo, useEffect, useMemo, useRef } from "react";
import ReactMarkdown, { type Components } from "react-markdown";
import type { Pluggable, PluggableList } from "unified";
import { useLocation } from "react-router";
import { useHighlighter } from "../review/highlight";
import { useRefs, useWanted } from "../refs/known";
import { RefTag } from "../refs/RefTag";
import type { Pinned } from "../refs/refs";
import { remarkRefs } from "../refs/remark";
import { cx } from "../ui/cx";
import { BoxContext, HeadingLinkContext, PlaceContext, toHeading } from "./context";
import type { Place } from "./links";
import css from "./Markdown.module.css";
import { MarkdownBlock } from "./MarkdownBlock";
import { MarkdownHeading } from "./MarkdownHeading";
import { MarkdownImage } from "./MarkdownImage";
import { MarkdownLink } from "./MarkdownLink";
import { MarkdownMaths } from "./MarkdownMaths";
import { alerts, anchors, displayMaths, foldFrontMatter, lineBreaks, sourceLines } from "./plugins";
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
// With breaks, a line break in a paragraph is one (lineBreaks), as in
// something someone said (RichText), rather than a space.
//
// With headingLink, a reader's h2 and h3 headings show # when they're
// hovered, which gives it the heading's name (to copy a link to it).
//
// It's drawn in its module's look, and says markdown too: that's the name
// other places still know a document by, until each is told a class of
// its own (their rules, such as the brief's documents' in
// mission/Brief/MissionBriefDoc.module.css and the refs' .markdown
// .named-ref, and the brief's code that finds its steps and headings).
export interface MarkdownProps {
  text: string;
  lines?: boolean;
  at?: Place;
  pinned?: Pinned[];
  headingLink?: (anchor: string) => void;
  breaks?: boolean;
}

export const Markdown = memo(function Markdown({ text, lines, at, pinned, headingLink, breaks }: MarkdownProps) {
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
  const plugins = useMemo<PluggableList>(
    () => [...syntax, foldFrontMatter, displayMaths, ...(breaks ? [lineBreaks] : []), ...(refs ? [[remarkRefs, { known: refs.known, links: !at, pinned }] as Pluggable] : [])],
    [refs, at, pinned, breaks],
  );
  useEffect(() => {
    if (hash && !clicked && box.current) toHeading(box.current, decodeURIComponent(hash));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [hash, at?.api, at?.path]);
  return (
    <div className={cx("markdown", css.markdown)} ref={box}>
      <BoxContext.Provider value={box}>
        <PlaceContext.Provider value={at}>
          <HeadingLinkContext.Provider value={headingLink}>
            <ReactMarkdown remarkPlugins={plugins} rehypePlugins={lines ? [anchors, alerts, sourceLines] : [anchors, alerts]} components={components}>
              {text}
            </ReactMarkdown>
          </HeadingLinkContext.Provider>
        </PlaceContext.Provider>
      </BoxContext.Provider>
    </div>
  );
});

// dataOf is a block's data- attributes, to keep on what draws it.
function dataOf(props: object): Record<string, unknown> {
  return Object.fromEntries(Object.entries(props).filter(([k]) => k.startsWith("data-")));
}

const components: Components = {
  pre: ({ children, node: _node, ...props }) => <MarkdownBlock data={dataOf(props)}>{children}</MarkdownBlock>,
  h2: ({ node: _node, ...props }) => <MarkdownHeading level={2} {...props} />,
  h3: ({ node: _node, ...props }) => <MarkdownHeading level={3} {...props} />,
  a: ({ href, children, ...props }) => {
    const ref = (props as Record<string, unknown>)["data-ref"];
    return typeof ref === "string" ? (
      <RefTag refKey={ref} words>
        {children}
      </RefTag>
    ) : (
      <MarkdownLink href={href}>{children}</MarkdownLink>
    );
  },
  // A ref found in the words (refs/remark.ts).
  span: ({ node: _node, children, ...props }) => {
    const ref = (props as Record<string, unknown>)["data-ref"];
    return typeof ref === "string" ? <RefTag refKey={ref}>{children}</RefTag> : <span {...props}>{children}</span>;
  },
  img: ({ src, alt, title }) => <MarkdownImage src={src} alt={alt} title={title} />,
  // Maths in a line; any other code as it is.
  code: ({ node: _node, className, children, ...props }) =>
    /\bmath-inline\b/.test(className ?? "") ? (
      <MarkdownMaths code={String(children ?? "")} display={/\bmath-display\b/.test(className ?? "")} inline />
    ) : (
      <code className={className} {...props}>
        {children}
      </code>
    ),
  // A task list's boxes show, but can't be ticked here.
  input: ({ type, checked }) => (type === "checkbox" ? <input type="checkbox" checked={!!checked} readOnly disabled /> : null),
};
