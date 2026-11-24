import type { CSSProperties, ReactNode } from "react";
import { Button as AriaButton } from "react-aria-components";
import { cx } from "./cx";
import { Icon } from "./Icon";
import css from "./TreeRow.module.css";

// TreeLook is how a row of a Tree is drawn, beside its name: all of it the
// page's to say, none of it the tree's to know.
//
//   mark   before its icon, in a column of its own so names line up: how a
//          file's changed (a TreeMark)
//   icon   before its name: a FileIcon, an Icon
//   end    its treats, at the row's end: a count, a dot, a tick
//   tone   gone (struck: a file deleted) or quiet (dimmed: one ignored)
//   title  what it says on hover
export interface TreeLook {
  mark?: ReactNode;
  icon?: ReactNode;
  end?: ReactNode;
  tone?: "gone" | "quiet";
  title?: string;
}

// TreeRow is one row of a Tree, drawn: so far in as its level, a folder's
// twist (which folds it: react-aria's chevron), then its mark, its icon,
// its name and its treats. A folder named for several ("internal/web",
// one row for a folder that holds only a folder) has its slashes quiet.
// It's drawn inside a tree's item, which is what's focused and pressed.
export function TreeRow({
  name,
  level,
  folder,
  open,
  current,
  hovered,
  focused,
  look,
  marks,
  guides,
  mono,
}: {
  name: string;
  level: number; // 1 at the top
  folder: boolean;
  open: boolean;
  current: boolean; // it's the one that's open in the page
  hovered: boolean;
  focused: boolean; // the keys are on it
  look: TreeLook;
  marks: boolean; // keep the marks' column on what isn't a folder, with or without one
  guides: boolean;
  mono: boolean;
}) {
  const pieces = folder ? name.split("/") : [name];
  return (
    <div
      className={cx(css.row, hovered && css.hovered, focused && css.focused, current && css.current, look.tone === "gone" && css.gone, look.tone === "quiet" && css.quiet)}
      style={{ "--tree-level": level - 1 } as CSSProperties}
      title={look.title}
    >
      {level > 1 && <span className={cx(css.indent, guides && css.guides)} aria-hidden />}
      {folder ? (
        <AriaButton slot="chevron" data-ui="" className={css.twist}>
          <Icon name={open ? "chevron-down" : "chevron-right"} size={12} />
        </AriaButton>
      ) : (
        <span className={css.leaf} aria-hidden />
      )}
      {((marks && !folder) || look.mark) && <span className={css.mark}>{look.mark}</span>}
      {look.icon && <span className={css.icon}>{look.icon}</span>}
      <span className={cx(css.name, mono && !folder && css.mono)}>
        {pieces.map((p, i) => (
          <span key={i}>
            {i > 0 && <span className={css.slash}>/</span>}
            {p}
          </span>
        ))}
      </span>
      {look.end && <span className={css.end}>{look.end}</span>}
    </div>
  );
}
