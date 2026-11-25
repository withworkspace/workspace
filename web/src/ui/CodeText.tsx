import css from "./CodeText.module.css";
import { cx } from "./cx";

// CodeText is a line's code: what a file's lines and a diff's are both
// drawn with. Its html is the line highlighted (review/highlight's
// highlightLines) or escaped, never what someone typed as it was; an
// empty line keeps its height. It says where the line is (data-rev,
// data-path, data-line, data-side), which is what following a name in it
// reads (review/CodeNav). noeol marks a file's last line that doesn't end
// in a newline. How it's spaced and what's behind it is the row's, by the
// class it gives.
export function CodeText({ html, rev, path, line, side, noeol, className }: { html: string; rev?: string; path?: string; line: number; side: "old" | "new"; noeol?: boolean; className?: string }) {
  const mark = noeol ? `<span class="${css.noeol}" title="No newline at the end of the file">⊘</span>` : "";
  return <code className={cx(css.text, className)} data-rev={rev} data-path={path} data-line={line} data-side={side} dangerouslySetInnerHTML={{ __html: (html || " ") + mark }} />;
}
