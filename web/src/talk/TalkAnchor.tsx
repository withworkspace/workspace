import { cx } from "../ui/cx";
import { Excerpt } from "../ui/Excerpt";
import css from "./TalkAnchor.module.css";

// TalkAnchor is what a conversation was asked of, over its turns: the
// lines of a file (excerpt, from its first line's number), or else the
// passage of a document, quoted.
export function TalkAnchor({ excerpt, quote }: { excerpt?: { lines: string[]; from: number; title: string }; quote?: string }) {
  if (excerpt) return <Excerpt className={css.anchor} lines={excerpt.lines} from={excerpt.from} title={excerpt.title} />;
  if (!quote) return null;
  return (
    <blockquote className={cx(css.anchor, css.quote)} title="The passage it was asked of">
      {quote}
    </blockquote>
  );
}
