import { type ComponentProps, type CSSProperties, type ReactNode, type Ref, useEffect, useImperativeHandle, useMemo, useRef, useState } from "react";
import { Tree as AriaTree, Collection, ListLayout, TreeItem, TreeItemContent, Virtualizer } from "react-aria-components";
import { cx } from "./cx";
import css from "./Tree.module.css";
import { type TreeLook, TreeRow } from "./TreeRow";
import { foldersOf, shownIds, type TreeNode } from "./treeNodes";

// A row's height, in px: the same for every row, which is what lets a
// tree that draws only the rows in view say where each one is.
const ROW = 22;

// TreeControl is the tree's keys in a page's hands, for a page whose own
// keys do what the tree's do (j for ↓, o for ⏎) from wherever the focus is.
//
//   press  one of the tree's keys (ArrowDown, ArrowLeft, Enter, Home),
//          pressed where its keys are. With the focus outside the tree
//          they come in first: to the row last gone to, or the one that's
//          open in the page, or where they were
//   go     a row to bring into view, once the folders it's in are open,
//          and where the keys come in next
export interface TreeControl {
  press(key: string): void;
  go(id: string): void;
}

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
//
// A page with keys of its own (the explorer: j and k, t for its finder)
// says typeahead={false}, so a letter typed with the keys in the tree is
// the page's and not a name gone to, and presses the tree's keys for its
// own through control (TreeControl).
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
  typeahead = true,
  control,
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
  typeahead?: boolean; // typing a name goes to it
  control?: Ref<TreeControl>;
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
  // bring says whether the row's in view now: a tree that's only just
  // drawn has no height to scroll yet, so it's asked again a frame on.
  const bring = (i: number): boolean => {
    const el = box.current;
    if (!el || i < 0) return true;
    // Under whatever room the page's class puts over the rows.
    const top = i * ROW + (parseFloat(getComputedStyle(el).paddingTop) || 0);
    if (top < el.scrollTop) el.scrollTop = top;
    else if (top + ROW > el.scrollTop + el.clientHeight) el.scrollTop = top + ROW - el.clientHeight;
    return top >= el.scrollTop && top + ROW <= el.scrollTop + el.clientHeight;
  };
  const bringSoon = (i: number) => {
    let frame = 0;
    let left = 20;
    const go = () => {
      if (!bring(i) && left-- > 0) frame = requestAnimationFrame(go);
    };
    go();
    return () => cancelAnimationFrame(frame);
  };
  const at = current === undefined ? -1 : shown.indexOf(current);
  useEffect(() => bringSoon(at), [at, current]);

  // The page's hands on the keys. Where they come in from outside is the
  // row gone to, or the one open in the page since it last changed; once
  // in, where they are is react-aria's to keep.
  const where = useRef(current);
  const was = useRef(current);
  if (was.current !== current) {
    was.current = current;
    where.current = current;
  }
  const rowOf = (id: string) => box.current?.querySelector<HTMLElement>(`[data-key="${CSS.escape(id)}"]`) ?? null;
  const goal = useRef<string>(undefined);
  const [asked, setAsked] = useState(0);
  useEffect(() => {
    const id = goal.current;
    const i = id === undefined ? -1 : shown.indexOf(id);
    if (id === undefined || i < 0) return;
    goal.current = undefined;
    if (box.current?.contains(document.activeElement)) rowOf(id)?.focus();
    return bringSoon(i);
  }, [asked, shown]);
  // A key the tree doesn't take (↓ on its last row) comes back to the
  // page, whose own key for it would press it here again: once is enough.
  const pressing = useRef(false);
  useImperativeHandle(
    control,
    () => ({
      press(key) {
        const el = box.current;
        if (!el || pressing.current) return;
        if (!el.contains(document.activeElement)) ((where.current === undefined ? null : rowOf(where.current)) ?? el).focus();
        where.current = undefined;
        const on = el.contains(document.activeElement) ? document.activeElement! : el;
        pressing.current = true;
        try {
          on.dispatchEvent(new KeyboardEvent("keydown", { key, bubbles: true, cancelable: true }));
          on.dispatchEvent(new KeyboardEvent("keyup", { key, bubbles: true, cancelable: true }));
        } finally {
          pressing.current = false;
        }
      },
      go(id) {
        where.current = id;
        goal.current = id;
        setAsked((n) => n + 1);
      },
    }),
    [],
  );

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
      // react-aria's tree takes this as its list does, though it doesn't say so.
      {...(typeahead ? {} : ({ disallowTypeAhead: true } as object))}
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
