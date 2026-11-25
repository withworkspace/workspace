import { highlightCode } from "../review/highlight";
import { CodeBlock } from "./CodeBlock";
import css from "./Markdown.module.css";
import { Text } from "../ui/Text";

// MarkdownFrontMatter is a document's front matter, folded away at its
// top: its keys say what's there, and it opens as YAML. It isn't the
// document's text.
export function MarkdownFrontMatter({ text, data }: { text: string; data: Record<string, unknown> }) {
  const keys = [...text.matchAll(/^([A-Za-z_][\w-]*):/gm)].map((m) => m[1]);
  return (
    <details className={css.frontMatter} {...data} data-skip="">
      <summary>
        Front matter{keys.length > 0 && <Text dim> · {keys.join(", ")}</Text>}
      </summary>
      <CodeBlock html={highlightCode(text, "yaml")} className={css.code} />
    </details>
  );
}
