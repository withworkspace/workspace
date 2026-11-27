import { Markdown } from "../markdown/LazyMarkdown";
import type { Pinned } from "../refs/refs";
import { cx } from "./cx";
import css from "./RichText.module.css";

// RichText is what someone said, as it reads: a turn of yours, a comment,
// a reply. It's what a RichTextArea writes, drawn: markdown, with the
// names it says (a file, a pull request, a mission) drawn as what they
// are, a card on hover (refs/). It's markdown as a comment is on GitHub,
// not as a document is: a line break is one (what was written before the
// box wrote markdown keeps its lines), and it's the size of where it is,
// a heading only a little larger than its words, with no room after it.
// pinned are the names as they were when it was said (a turn's).
export function RichText({ text, pinned, className }: { text: string; pinned?: Pinned[]; className?: string }) {
  return (
    <div className={cx(css.text, className)}>
      <Markdown text={text} pinned={pinned} breaks />
    </div>
  );
}
