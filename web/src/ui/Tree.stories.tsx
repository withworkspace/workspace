import type { Meta, StoryObj } from "@storybook/react-vite";
import { type CSSProperties, type ReactNode, useMemo, useState } from "react";
import { Button } from "./Button";
import { Count } from "./Count";
import { FileIcon } from "./FileIcon";
import { Icon, type IconName } from "./Icon";
import { Input } from "./Input";
import { Tree } from "./Tree";
import { TreeDot } from "./TreeDot";
import { TreeMark } from "./TreeMark";
import type { TreeLook } from "./TreeRow";
import { compact, filterTree, foldersOf, leavesOf, pathTo, type TreeNode, treeOf } from "./treeNodes";

// A tree of rows in folders that fold: a repo's files, with each thing a
// row can carry beside its name, then folded and found in from outside,
// one of thousands, and one that isn't files at all. Click in one and use
// the arrows: → opens, ← folds, typing a name goes to it.

const meta: Meta = { title: "Components/Tree" };
export default meta;

// A file of the repo: how it's changed since its base, whether that's
// committed, what's been said on it, and whether you've read it.
interface File {
  path: string;
  status?: "added" | "modified" | "deleted" | "renamed";
  uncommitted?: boolean;
  comments?: number;
  viewed?: boolean;
  ignored?: boolean;
}

const files: File[] = [
  { path: ".gitignore" },
  { path: "README.md", status: "modified", viewed: true },
  { path: "go.mod" },
  { path: "go.sum" },
  { path: "justfile" },
  { path: "cmd/workspace/main.go" },
  { path: "internal/nav/nav.go", status: "modified", comments: 2 },
  { path: "internal/nav/refs.go", status: "modified", viewed: true },
  { path: "internal/nav/tree.go", status: "added", uncommitted: true },
  { path: "internal/nav/walk.go" },
  { path: "internal/nav/queries/go.scm" },
  { path: "internal/nav/queries/typescript.scm", status: "added" },
  { path: "internal/review/diff.go" },
  { path: "internal/review/order.go", status: "renamed" },
  { path: "internal/review/litter.go", status: "deleted" },
  { path: "internal/web/api.go", status: "modified", comments: 1 },
  { path: "internal/web/files.go" },
  { path: "internal/web/dist/index.html", ignored: true },
  { path: "internal/web/dist/assets/index.js", ignored: true },
  { path: "scripts/try.sh" },
  { path: "web/package.json" },
  { path: "web/pnpm-lock.yaml", status: "modified" },
  { path: "web/vite.config.ts" },
  { path: "web/src/App.tsx" },
  { path: "web/src/styles.css" },
  { path: "web/src/files/Explorer.tsx", status: "modified", comments: 4 },
  { path: "web/src/files/lines.ts" },
  { path: "web/src/files/lines.test.ts" },
  { path: "web/src/ui/Tree.tsx", status: "added" },
  { path: "web/src/ui/Tree.module.css", status: "added" },
  { path: "web/src/ui/file2.ts" },
  { path: "web/src/ui/file10.ts" },
  { path: "docs/assets/shots/files.png" },
];

const nodes = compact(treeOf(files, (f) => f.path));

const box: CSSProperties = { width: 300, height: 460, border: "1px solid var(--line)", borderRadius: 8, padding: 6, boxSizing: "border-box" };
const page: CSSProperties = { display: "flex", gap: 24, alignItems: "start", flexWrap: "wrap", padding: 24 };
const fill: CSSProperties = { height: "100%" };

function Panel({ title, children, style }: { title: string; children: ReactNode; style?: CSSProperties }) {
  return (
    <figure style={{ margin: 0 }}>
      <figcaption style={{ marginBottom: 8, color: "var(--dim)", fontSize: 12 }}>{title}</figcaption>
      <div style={{ ...box, ...style }}>{children}</div>
    </figure>
  );
}

// A file's icon by its name; a folder has its twist alone.
const icons = (n: TreeNode<File>): TreeLook => (n.children ? {} : { icon: <FileIcon path={n.id} /> });

// Folders, files' icons, lines down from open folders, and the one that's
// open in the page. A folder that holds only a folder is one row
// ("cmd/workspace"). Beside it, the same with no lines and names in the
// UI's face, and one with nothing in it.
function Plain({ guides, mono }: { guides?: boolean; mono?: boolean }) {
  const [current, setCurrent] = useState("internal/nav/nav.go");
  return (
    <div style={fill}>
      <Tree aria-label="Files" nodes={nodes} look={icons} startOpen={["internal", "internal/nav"]} current={current} onPick={(n) => setCurrent(n.id)} guides={guides} mono={mono} />
    </div>
  );
}

export const Files: StoryObj = {
  render: () => (
    <div style={page}>
      <Panel title="Files: icons, guides, the open one">
        <Plain mono />
      </Panel>
      <Panel title="No guides, names in the UI's face">
        <Plain guides={false} />
      </Panel>
      <Panel title="Nothing in it" style={{ height: 120 }}>
        <Tree aria-label="Files" nodes={[]} empty="The capsule is empty." />
      </Panel>
    </div>
  ),
};

// What a row carries beside its name. Before its icon, its mark: how the
// file's changed. At its end, its treats: what's said on it, that it's
// not committed, that you've read it. A folder says what's in it while
// it's folded: how many files changed, and whether anything's said. A
// deleted file is struck, an ignored one quiet.
const marks: Record<NonNullable<File["status"]>, ReactNode> = {
  added: (
    <TreeMark tone="done" title="Added">
      A
    </TreeMark>
  ),
  modified: (
    <TreeMark tone="attention" title="Modified">
      M
    </TreeMark>
  ),
  deleted: (
    <TreeMark tone="danger" title="Deleted">
      D
    </TreeMark>
  ),
  renamed: (
    <TreeMark tone="accent" title="Renamed">
      <Icon name="arrow-right" size={10} />
    </TreeMark>
  ),
};

function treats(n: TreeNode<File>, at: { open: boolean }): TreeLook {
  if (n.children) {
    const inside = leavesOf(n.children).map((l) => l.value!);
    const changed = inside.filter((f) => f.status).length;
    const said = inside.reduce((s, f) => s + (f.comments ?? 0), 0);
    return {
      tone: inside.every((f) => f.ignored) ? "quiet" : undefined,
      end: !at.open && (changed > 0 || said > 0) && (
        <>
          {said > 0 && <TreeDot tone="accent" title={`${said} comments in it`} />}
          {changed > 0 && <Count title={`${changed} changed in it`}>{changed}</Count>}
        </>
      ),
    };
  }
  const f = n.value!;
  return {
    mark: f.status && marks[f.status],
    icon: <FileIcon path={n.id} />,
    tone: f.status === "deleted" ? "gone" : f.ignored ? "quiet" : undefined,
    title: f.status ? `${n.id}: ${f.status}${f.uncommitted ? ", not committed" : ""}` : n.id,
    end: (f.comments || f.uncommitted || f.viewed) && (
      <>
        {f.uncommitted && <TreeDot tone="attention" title="Not committed" />}
        {f.comments ? <Count tone="attention">{f.comments}</Count> : null}
        {f.viewed && (
          <TreeMark tone="done" title="Viewed">
            <Icon name="check" size={11} />
          </TreeMark>
        )}
      </>
    ),
  };
}

export const Treats: StoryObj = {
  render: () => (
    <div style={page}>
      <Panel title="Marks, counts, dots and ticks; folders say what's in them">
        <div style={fill}>
          <Tree aria-label="Files" nodes={nodes} look={treats} marks mono startOpen={["internal", "internal/nav", "internal/review"]} current="internal/nav/nav.go" />
        </div>
      </Panel>
      <Panel title="Only what's changed, all open">
        <ChangedOnly />
      </Panel>
    </div>
  ),
};

function ChangedOnly() {
  const changed = useMemo(() => compact(treeOf(files.filter((f) => f.status), (f) => f.path)), []);
  return (
    <div style={fill}>
      <Tree aria-label="Changed files" nodes={changed} look={treats} marks mono startOpen="all" />
    </div>
  );
}

// What's open is the page's here: it folds all of it, opens all of it,
// and reveals a file deep in it (opening the folders it's in; the tree
// scrolls to it). Typing in the box cuts the tree to what matches, all of
// it open while it's cut.
function Folded() {
  const all = useMemo(() => new Set(foldersOf(nodes)), []);
  const [open, setOpen] = useState<ReadonlySet<string>>(new Set(["internal"]));
  const [current, setCurrent] = useState<string>();
  const [find, setFind] = useState("");
  const found = useMemo(() => (find ? filterTree(nodes, (n) => !n.children && n.id.toLowerCase().includes(find.toLowerCase())) : nodes), [find]);
  const reveal = (id: string) => {
    setFind("");
    setOpen(new Set([...open, ...(pathTo(nodes, id) ?? [])]));
    setCurrent(id);
  };
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 6, height: "100%" }}>
      <div style={{ display: "flex", gap: 6 }}>
        <Button size="small" onClick={() => setOpen(new Set())}>
          Fold all
        </Button>
        <Button size="small" onClick={() => setOpen(all)}>
          Open all
        </Button>
        <Button size="small" onClick={() => reveal("web/src/ui/Tree.module.css")}>
          Reveal Tree.module.css
        </Button>
      </div>
      <Input value={find} onChange={(e) => setFind(e.target.value)} placeholder="Filter files" aria-label="Filter files" />
      <div style={{ flex: 1, minHeight: 0 }}>
        <div style={fill}>
          <Tree
            aria-label="Files"
            nodes={found}
            look={treats}
            marks
            mono
            open={find ? new Set(foldersOf(found)) : open}
            onOpen={find ? undefined : setOpen}
            current={current}
            onPick={(n) => setCurrent(n.id)}
            empty={`No file matches “${find}”.`}
           
          />
        </div>
      </div>
      <div style={{ color: "var(--dim)", fontSize: 12 }}>{current ? `Open: ${current}` : "Nothing open"}</div>
    </div>
  );
}

export const Folding: StoryObj = {
  render: () => (
    <div style={page}>
      <Panel title="Folded, opened, revealed and filtered from outside" style={{ width: 380, height: 520 }}>
        <Folded />
      </Panel>
    </div>
  ),
};

// Twenty thousand files, only the rows in view drawn (virtual): it opens
// and scrolls as a small one does.
const many: File[] = Array.from({ length: 20000 }, (_, i) => {
  const pkg = `pkg${String(Math.floor(i / 400)).padStart(2, "0")}`;
  const sub = `mod${String(Math.floor((i % 400) / 40)).padStart(2, "0")}`;
  const ext = ["go", "ts", "tsx", "css", "md"][i % 5];
  return { path: `packages/${pkg}/${sub}/file${i % 40}.${ext}`, status: i % 97 === 0 ? "modified" : undefined };
});

function ManyFiles() {
  const big = useMemo(() => compact(treeOf(many, (f) => f.path)), []);
  const [current, setCurrent] = useState<string>();
  return (
    <div style={fill}>
      <Tree aria-label="Files" nodes={big} look={treats} marks mono virtual startOpen="all" current={current} onPick={(n) => setCurrent(n.id)} />
    </div>
  );
}

export const Many: StoryObj = {
  render: () => (
    <div style={page}>
      <Panel title="20,000 files, all open: only the rows in view are drawn">
        <ManyFiles />
      </Panel>
    </div>
  ),
};

// Not files: what's defined in a file, each with an icon for its kind and
// the line it's at as its treat.
interface Symbol {
  kind: "type" | "func" | "field";
  line: number;
}

const kindIcon: Record<Symbol["kind"], IconName> = { type: "box", func: "code", field: "hash" };

const outline: TreeNode<Symbol>[] = [
  {
    id: "Service",
    name: "Service",
    value: { kind: "type", line: 18 },
    children: [
      { id: "Service.Runtime", name: "Runtime", value: { kind: "field", line: 19 } },
      { id: "Service.facts", name: "facts", value: { kind: "field", line: 21 } },
      { id: "Service.Tree", name: "Tree", value: { kind: "func", line: 54 } },
      { id: "Service.Names", name: "Names", value: { kind: "func", line: 88 } },
    ],
  },
  { id: "New", name: "New", value: { kind: "func", line: 36 } },
  { id: "CacheMax", name: "CacheMax", value: { kind: "field", line: 15 } },
];

export const Outline: StoryObj = {
  render: () => (
    <div style={page}>
      <Panel title="A file's outline" style={{ width: 240, height: 220 }}>
        <Tree
          aria-label="Outline of nav.go"
          nodes={outline}
          startOpen="all"
          guides={false}
          look={(n) => ({
            icon: <Icon name={kindIcon[n.value!.kind]} size={13} />,
            end: <span style={{ color: "var(--faint)", fontSize: 11, fontVariantNumeric: "tabular-nums" }}>{n.value!.line}</span>,
            title: `${n.name}, line ${n.value!.line}`,
          })}
        />
      </Panel>
    </div>
  ),
};
