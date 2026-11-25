import { highlightCode } from "../review/highlight";
import { CodeBlock } from "./CodeBlock";
import css from "./Markdown.module.css";

// MarkdownFrontMatter is a document's front matter, folded away at its
// top: its keys say what's there, and it opens as YAML. It isn't the
// document's text.
export function MarkdownFrontMatter({ text, data }: { text: string; data: Record<string, unknown> }) {
  const keys = [...text.matchAll(/^([A-Za-z_][\w-]*):/gm)].map((m) => m[1]);
  return (
    <details className={css.frontMatter} {...data} data-skip="">
      <summary>
        Front matter{keys.length > 0 && <span className="dim"> · {keys.join(", ")}</span>}
      </summary>
      <CodeBlock html={highlightCode(text, "yaml")} className={css.code} />
    </details>
  );
}
