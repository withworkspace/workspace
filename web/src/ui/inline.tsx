import { Fragment, type ReactNode, useContext } from "react";
import { useRefs, useWanted } from "../refs/known";
import { RefTag } from "../refs/RefTag";
import { findRefs, type Known, refAt, refKey } from "../refs/refs";
import { linkRefs, type Refs, RefContext } from "./hover";

// Inline is agent-written markdown in a place with room for a line or two
// (a log line, a summary, a list row): `code`, **bold**, *emphasis* and
// [links](…) render; block syntax (fences, headings, bullets, quotes)
// folds into the line, so no `**` or ``` ever shows. Links render as their
// text: these lines often sit inside a link already.

// flatten folds markdown's block syntax into one paragraph.
export function flatten(md: string): string {
  const out: string[] = [];
  let fenced = false;
  for (const raw of md.split("\n")) {
    const line = raw.trim();
    if (/^(```|~~~)/.test(line)) {
      fenced = !fenced;
      continue;
    }
    if (!line) continue;
    if (fenced) {
      out.push("`" + line.replace(/`/g, "") + "`");
      continue;
    }
    if (/^\[\^[^\]]+\]:/.test(line)) continue; // a footnote's definition
    if (/^\|?\s*:?-{3,}/.test(line)) continue; // a table's rule
    const bullet = /^([-*+]|\d+[.)])\s+/.test(line);
    const text = line
      .replace(/^#{1,6}\s+/, "")
      .replace(/^>\s?/, "")
      .replace(/^([-*+]|\d+[.)])\s+(\[[ xX]\]\s+)?/, "");
    out.push(bullet ? `• ${text}` : text);
  }
  return out
    .join(" ")
    .replace(/```[a-z]*\s*([^`]+?)\s*```/g, "`$1`") // a fence written inline
    .replace(/\[\^[^\]]+\]:\s*/g, "") // a footnote's definition, inline
    .replace(/``\s*([^`]+?)\s*``/g, "`$1`");
}

const token = /(`[^`]+`)|(\*\*[^*]+\*\*|__[^_]+__)|(\*[^*\s][^*]*\*)|(\[\^[^\]]+\])|(\[[^\]]+\]\([^)\s]+\))/g;

// named draws the refs the web UI knows (refs/) in words a mission's own
// refs (c1, d2) have left as words.
function named(nodes: ReactNode[], known: Known | undefined, at: number): ReactNode[] {
  if (!known) return nodes;
  return nodes.flatMap((n, i) => {
    if (typeof n !== "string") return [n];
    const out: ReactNode[] = [];
    let last = 0;
    for (const f of findRefs(n, known)) {
      if (f.from > last) out.push(n.slice(last, f.from));
      out.push(
        <RefTag key={`r${at}.${i}.${f.from}`} refKey={refKey(f.ref)} inline>
          {n.slice(f.from, f.to)}
        </RefTag>,
      );
      last = f.to;
    }
    if (last < n.length) out.push(n.slice(last));
    return out;
  });
}

function parse(text: string, refs?: Refs, known?: Known): ReactNode[] {
  const out: ReactNode[] = [];
  let last = 0;
  // Plain words: the refs in them drawn as refs.
  const words = (s: string, at: number) => out.push(<Fragment key={`w${at}`}>{named(linkRefs(s, refs, at), known, at)}</Fragment>);
  for (const m of text.matchAll(token)) {
    const at = m.index ?? 0;
    if (at > last) words(text.slice(last, at), last);
    const [whole, code, bold, em, footnote, link] = m;
    if (code) {
      // A name in backticks is drawn as what it names.
      const ref = known && refAt(code.slice(1, -1), known);
      out.push(
        ref ? (
          <RefTag key={at} refKey={refKey(ref)} inline>
            <code>{code.slice(1, -1)}</code>
          </RefTag>
        ) : (
          <code key={at}>{code.slice(1, -1)}</code>
        ),
      );
    } else if (bold) out.push(<strong key={at}>{parse(bold.slice(2, -2), refs, known)}</strong>);
    else if (em) out.push(<em key={at}>{parse(em.slice(1, -1), refs, known)}</em>);
    else if (footnote) {
      // a footnote's marker: left out
    } else if (link) {
      // Its words; with the kind's icon and card when its address names
      // something (an agent's link to a file on this disk).
      const [, words, url] = /^\[([^\]]+)\]\(([^)\s]+)\)$/.exec(link)!;
      const ref = known && refAt(url, known);
      out.push(
        ref ? (
          <RefTag key={at} refKey={refKey(ref)} words inline>
            {parse(words, refs)}
          </RefTag>
        ) : (
          <Fragment key={at}>{parse(words, refs, known)}</Fragment>
        ),
      );
    }
    last = at + whole.length;
  }
  if (last < text.length) words(text.slice(last), last);
  return out;
}

// Inline renders text so: lines (if given) clamps it to that many lines.
export function Inline({ text, lines, className }: { text: string; lines?: number; className?: string }) {
  const refs = useContext(RefContext);
  // Inside a card the mission's refs are words (RefContext is cleared), and
  // so are the web UI's (NoRefs).
  const known = useRefs()?.known;
  useWanted(text);
  return (
    <span className={`inline-md${lines ? " clamp" : ""}${className ? ` ${className}` : ""}`} style={lines ? ({ "--lines": lines } as React.CSSProperties) : undefined}>
      {parse(flatten(text), refs, known)}
    </span>
  );
}

// plain is the same text with no markup at all, for a title attribute or
// a page's title.
export const plain = (md: string) =>
  flatten(md)
    .replace(/`([^`]+)`/g, "$1")
    .replace(/\*\*([^*]+)\*\*|__([^_]+)__/g, "$1$2")
    .replace(/\*([^*\s][^*]*)\*/g, "$1")
    .replace(/\[\^[^\]]+\]/g, "")
    .replace(/\[([^\]]+)\]\([^)\s]+\)/g, "$1");
