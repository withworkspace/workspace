import { type ComponentProps, type CSSProperties, type ReactNode, useEffect, useMemo, useRef, useState } from "react";
import { Tree as AriaTree, Collection, ListLayout, TreeItem, TreeItemContent, Virtualizer } from "react-aria-components";
import { cx } from "./cx";
import css from "./Tree.module.css";
import { type TreeLook, TreeRow } from "./TreeRow";
import { foldersOf, shownIds, type TreeNode } from "./treeNodes";

// A row's height, in px: the same for every row, which is what lets a
// tree that draws only the rows in view say where each one is.
const ROW = 22;

// Tree is rows in folders that fold: a repo's files, a file's outline. It
// draws nodes (treeNodes.ts makes them: treeOf from paths, compact,
// filterTree), and how a row looks beside its name is the page's to say,
// row by row (look: its icon, its mark, its treats, its tone).
//
// It's react-aria's tree, so the keys are its: it's one stop for Tab, ↑
// and ↓ move along it, → opens a folder (or goes into it) and ← folds it
// (or goes out to the one it's in), Home and End are its ends, typing a
// name goes to it, and ⏎ or a press is what a click is: a folder folds or
// opens, and anything else is picked (onPick). A screen reader hears a
// tree, each row with its level and whether it's open.
//
// What's open is the page's (open and onOpen: a set of folders' ids), or
// the tree's own from where it starts (startOpen: some ids, or "all").
// current is the row that's open in the page, marked, and scrolled to
// when it changes: the page opens the folders it's in (pathTo).
//
// virtual draws only the rows in view, for a tree of thousands: it then
// needs a height to scroll in, from the class the page gives it.
export function Tree<T>({
  nodes,
  look,
  open,
  onOpen,
  startOpen,
  current,
  onPick,
  marks = false,
  guides = true,
  mono = false,
  virtual = false,
  empty,
  className,
  "aria-label": label,
}: {
  nodes: TreeNode<T>[];
  look?: (node: TreeNode<T>, at: { open: boolean; level: number }) => TreeLook;
  open?: ReadonlySet<string>;
  onOpen?: (open: Set<string>) => void;
  startOpen?: Iterable<string> | "all";
  current?: string;
  onPick?: (node: TreeNode<T>) => void;
  marks?: boolean; // keep a column for marks, so names line up
  guides?: boolean; // a line down from each open folder
  mono?: boolean; // names that aren't folders' in the code's face
  virtual?: boolean;
  empty?: ReactNode;
  className?: string;
  "aria-label": string;
}) {
  const [own, setOwn] = useState<ReadonlySet<string>>(() => new Set(startOpen === "all" ? foldersOf(nodes) : (startOpen ?? [])));
  const opened = open ?? own;
  const setOpened = (next: Set<string>) => {
    if (open === undefined) setOwn(next);
    onOpen?.(next);
  };

  // To the current row when it changes, or when what's open over it does
  // (it's been revealed): only as far as brings it into view.
  const box = useRef<HTMLDivElement>(null);
  const shown = useMemo(() => shownIds(nodes, opened), [nodes, opened]);
  const at = current === undefined ? -1 : shown.indexOf(current);
  useEffect(() => {
    const el = box.current;
    if (!el || at < 0) return;
    const top = at * ROW;
    if (top < el.scrollTop) el.scrollTop = top;
    else if (top + ROW > el.scrollTop + el.clientHeight) el.scrollTop = top + ROW - el.clientHeight;
  }, [at, current]);

  // A row is drawn again when what it's drawn from changes: react-aria
  // keeps what it drew for a node otherwise.
  const deps = [look, current, marks, guides, mono];
  const item = (node: TreeNode<T>) => (
    <TreeItem
      id={node.id}
      textValue={node.name}
      hasChildItems={!!node.children}
      onAction={node.children ? undefined : () => onPick?.(node)}
      className={css.item}
      // react-aria's row says nothing of which one the page has open.
      render={(props) => <div {...(props as ComponentProps<"div">)} aria-current={node.id === current ? "true" : undefined} />}
    >
      <TreeItemContent>
        {(s) => {
          const l = look?.(node, { open: s.isExpanded, level: s.level }) ?? {};
          return (
            <TreeRow
              name={node.name}
              level={s.level}
              folder={!!node.children}
              open={s.isExpanded}
              current={node.id === current}
              hovered={s.isHovered}
              focused={s.isFocusVisible}
              look={{ ...l, title: l.title ?? node.id }}
              marks={marks}
              guides={guides}
              mono={mono}
            />
          );
        }}
      </TreeItemContent>
      {node.children && (
        <Collection items={node.children} dependencies={deps}>
          {item}
        </Collection>
      )}
    </TreeItem>
  );

  const tree = (
    <AriaTree
      ref={box}
      aria-label={label}
      className={cx(css.tree, className)}
      style={{ "--tree-row": `${ROW}px` } as CSSProperties}
      items={nodes}
      dependencies={deps}
      selectionMode="none"
      expandedKeys={opened}
      onExpandedChange={(keys) => setOpened(new Set([...keys].map(String)))}
      renderEmptyState={empty === undefined ? undefined : () => <div className={css.empty}>{empty}</div>}
    >
      {item}
    </AriaTree>
  );
  if (!virtual) return tree;
  return (
    <Virtualizer layout={ListLayout} layoutOptions={{ rowHeight: ROW }}>
      {tree}
    </Virtualizer>
  );
}
