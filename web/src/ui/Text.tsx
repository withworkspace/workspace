import { cx } from "./cx";
import css from "./Text.module.css";

// A look words can take: quieter (dim), in the code's type (mono), a line
// count's plus or minus, or a link in the accent's colour that's
// underlined under the pointer.
export type TextLook = "dim" | "mono" | "plus" | "minus" | "link";

type Tag = "span" | "p" | "div" | "td" | "th" | "dd" | "dt" | "li" | "ul" | "ol" | "small" | "strong" | "em" | "b" | "i" | "code" | "pre" | "label" | "h2" | "h3" | "h4" | "header" | "footer" | "section" | "article" | "blockquote" | "figcaption" | "kbd" | "q" | "sup" | "time";

type TextProps = { as?: Tag; dim?: boolean; mono?: boolean; plus?: boolean; minus?: boolean; ref?: React.Ref<HTMLElement> } & React.AllHTMLAttributes<HTMLElement>;

// Text is words with a look of their own among the words round them: a
// span unless it's told what it is (as="p").
export function Text({ as = "span", dim, mono, plus, minus, className, ...rest }: TextProps) {
  const Tag = as as "span";
  return <Tag className={cx(mono && css.mono, dim && css.dim, plus && css.plus, minus && css.minus, className)} {...(rest as React.HTMLAttributes<HTMLSpanElement>)} />;
}

// textClass is the looks for something that isn't Text: an Icon that's
// dim, an Input that's mono, a router Link that's a link.
export function textClass(...looks: (TextLook | false | null | undefined)[]): string {
  return cx(...looks.map((l) => l && lookClass[l]));
}

const lookClass: Record<TextLook, string> = { dim: css.dim, mono: css.mono, plus: css.plus, minus: css.minus, link: css.link };
