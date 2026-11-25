import type { ReactElement, ReactNode } from "react";
import { highlightCode } from "../review/highlight";
import { CodeBlock } from "./CodeBlock";
import css from "./Markdown.module.css";
import { MarkdownFrontMatter } from "./MarkdownFrontMatter";
import { MarkdownMaths } from "./MarkdownMaths";
import { Mermaid } from "./Mermaid";

// MarkdownBlock is a fenced code block: a diagram for mermaid, maths for
// math ($$…$$ too), front matter folded, else code. data is the block's
// data- attributes, kept on what draws it.
export function MarkdownBlock({ children, data }: { children?: ReactNode; data: Record<string, unknown> }) {
  const code = children as ReactElement<{ className?: string; children?: ReactNode }> | undefined;
  const lang = /language-([\w+-]+)/.exec(code?.props?.className ?? "")?.[1];
  const text = String(code?.props?.children ?? "").replace(/\n$/, "");
  if (lang === "mermaid") return <Mermaid code={text} data={data} drawnClass={css.diagram} codeClass={css.code} />;
  if (lang === "math") return <MarkdownMaths code={text} display data={data} />;
  if (lang === "front-matter") return <MarkdownFrontMatter text={text} data={data} />;
  return <CodeBlock lang={lang} html={highlightCode(text, lang)} className={css.code} data={data} />;
}
