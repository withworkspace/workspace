import { cx } from "./cx";
import css from "./Prose.module.css";

type ProseProps = { as?: "div" | "article" | "section"; ref?: React.Ref<HTMLElement> } & React.HTMLAttributes<HTMLElement>;

// Prose holds something written to be read (a document, a brief, a
// proposal's text): its markdown larger, at a reading measure, its links
// in the accent.
export function Prose({ as = "div", className, ...rest }: ProseProps) {
  const Tag = as as "div";
  return <Tag className={proseClass(className)} {...(rest as React.HTMLAttributes<HTMLDivElement>)} />;
}

// proseClass is prose's look for what isn't a Prose: a pre that's its
// text as written. It says "prose" too, a class with no rules of its own:
// what's drawn inside prose (markdown/Markdown.module.css), or a place
// with prose in it (a brief's documents, a review's), finds it by that.
export function proseClass(className?: string): string {
  return cx(css.prose, "prose", className);
}
