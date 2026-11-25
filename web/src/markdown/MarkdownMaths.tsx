import { useEffect, useState } from "react";
import { cx } from "../ui/cx";
import { codeBlockClass } from "./CodeBlock";
import css from "./Markdown.module.css";

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

// MarkdownMaths is maths drawn: in a line (inline), or on lines of its
// own, and set out large (display) or to fit the line. As written until
// KaTeX has loaded. It isn't the document's words (a comment is kept by
// what it quotes, and this reads differently drawn).
export function MarkdownMaths({ code, display = false, inline = false, data = {} }: { code: string; display?: boolean; inline?: boolean; data?: Record<string, unknown> }) {
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
      <pre className={cx(codeBlockClass, css.code)} {...data} data-skip="">
        {code}
      </pre>
    );
  }
  return inline ? <span data-skip="" dangerouslySetInnerHTML={{ __html: html }} /> : <div className={css.maths} {...data} data-skip="" dangerouslySetInnerHTML={{ __html: html }} />;
}
