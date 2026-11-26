import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { Link } from "react-router";
import type { TreeFile } from "../../api";
import { Markdown } from "../../markdown/LazyMarkdown";
import { Button, buttonClass } from "../../ui/Button";
import { CodeLines as UICodeLines } from "../../ui/CodeLines";
import { marksOf } from "../../ui/codeLayout";
import { Crumbs } from "../../ui/Crumbs";
import { Empty } from "../../ui/Empty";
import { Finder, FinderNone } from "../../ui/Finder";
import { fuzzy, marked } from "../../ui/fuzzy";
import { Icon } from "../../ui/Icon";
import { Keys } from "../../ui/Keys";
import { Loading } from "../../ui/Loading";
import { Segment, Segmented } from "../../ui/Segmented";
import { Tree as UITree } from "../../ui/Tree";
import { FilesBar, filesBarCrumbsClass } from "./FilesBar";
import { FilesBody } from "./FilesBody";
import { FilesCapsule } from "./FilesCapsule";
import { FilesChanged } from "./FilesChanged";
import { FilesChangeLegend } from "./FilesChangeLegend";
import { FilesCode, filesCodeLinesClass } from "./FilesCode";
import { FilesExplorer } from "./FilesExplorer";
import { FilesFile, filesFileNoteClass } from "./FilesFile";
import { FilesFileHead } from "./FilesFileHead";
import { FilesFileMeta } from "./FilesFileMeta";
import { FilesFilePath } from "./FilesFilePath";
import { FilesFinderRow } from "./FilesFinderRow";
import { FilesImage } from "./FilesImage";
import { FilesMark } from "./FilesMark";
import { FilesMarkdown } from "./FilesMarkdown";
import { FilesOverview } from "./FilesOverview";
import { FilesPanes, FilesPanesGlyph } from "./FilesPanes";
import { FilesTab, FilesTabs } from "./FilesTabs";
import { FilesTree, filesTreeClass } from "./FilesTree";
import { filesTreeLook } from "./filesTreeLook";
import { FilesTreeTools } from "./FilesTreeTools";
import { FilesView } from "./FilesView";
import { paneChoices, type Panes } from "./panes";
import { changedIn, filesTree } from "./tree";
import { Text } from "../../ui/Text";

// The file explorer, from its parts: a mission's capsule's files with one
// open (its bar, its tree, the file with its changed lines marked and a
// fold shut), and every state of a tree's row, a file's mark and a line;
// what the file's pane says with none open, or of a file that isn't
// there; a picture and a markdown file; the panes picked; and finding a
// file by a few of its letters.

const meta: Meta = { title: "Files/Browser" };
export default meta;

const none = () => {};

const tabs = (
  <FilesTabs>
    <FilesTab to="/" icon="file" label="Files" current />
    <FilesTab to="/" icon="play" label="Runs" count={2} />
    <FilesTab to="/" icon="book" label="API docs" />
  </FilesTabs>
);

const lines = [
  "// A month's invoices, summed: what was sent, what was paid, what's late.",
  "import { formatMoney } from '../invoices/money'",
  "",
  "export interface Summary {",
  "  sent: number",
  "  paid: number",
  "  late: number",
  "}",
  "",
  "export function summarise(invoices: Invoice[], today: string): Summary {",
  "  let sent = 0",
  "  let paid = 0",
  "  for (const inv of invoices) {",
  "    sent += inv.total",
  "  }",
  "  return { sent, paid, late: 0 }",
  "}",
];

function Code({ picked }: { picked?: [number, number] }) {
  const marks = marksOf([
    { kind: "added", start: 11, end: 12 },
    { kind: "modified", start: 14 },
    { kind: "deleted", start: 16 },
  ]);
  return (
    <FilesCode>
      <UICodeLines
        className={filesCodeLinesClass}
        html={lines.map((text) => text.replace(/&/g, "&amp;").replace(/</g, "&lt;"))}
        path="src/report/summary.ts"
        marks={marks}
        picked={picked && { from: picked[0], to: picked[1] }}
        folds={[
          { at: 4, to: 8 },
          { at: 10, to: 17 },
          { at: 13, to: 15 },
        ]}
        folded={new Set([4])}
      />
    </FilesCode>
  );
}

// The tree as the explorer draws it (ui/Tree, with the explorer's look):
// a folder folded over three changed files, and each way a file's changed.
const treeFiles: TreeFile[] = [
  { path: "docs/guides/late.md", status: "modified" },
  { path: "docs/guides/setup.md", status: "added" },
  { path: "docs/guides/old.md", status: "deleted" },
  { path: "src/invoices/money.ts", status: "modified" },
  { path: "src/invoices/sort.ts", status: "deleted" },
  { path: "src/report/summary.ts", status: "added", uncommitted: true },
  { path: "src/styles/theme.css", status: "renamed" },
  { path: "src/App.tsx" },
  { path: "package.json" },
];
const treeNodes = filesTree(treeFiles);
const treeCounts = changedIn(treeNodes);

function Tree() {
  return (
    <UITree
      aria-label="web/late-invoices"
      className={filesTreeClass}
      nodes={treeNodes}
      look={(n, at) => filesTreeLook(n, at, treeCounts)}
      startOpen={["src", "src/invoices", "src/report"]}
      current="src/report/summary.ts"
      typeahead={false}
      marks
      mono
    />
  );
}

function Explorer({ panes = 2, file = true }: { panes?: Panes; file?: boolean }) {
  const p = paneChoices(panes, 2, false);
  return (
    <div style={{ height: 620 }}>
      <FilesExplorer>
        <FilesBar
          panes={<FilesPanes now={p.now} choices={p.choices} onPick={none} />}
          where={
            <>
              <Crumbs className={filesBarCrumbsClass} steps={[{ to: "/", label: "Show what's late on the dashboard" }]} />
              <FilesCapsule repo="web" name="late-invoices" />
            </>
          }
          tabs={tabs}
        />
        <FilesBody single={panes === 1}>
          {panes > 1 && (
            <FilesTree tools={<FilesTreeTools all={14} changed={5} changedOnly={false} canChange onShow={none} onFind={none} onSearch={none} />}>
              <Tree />
            </FilesTree>
          )}
          <FilesView solid={panes > 1}>
            {file ? (
              <FilesFile>
                <FilesFileHead
                  path={<FilesFilePath path="src/report/summary.ts" onReveal={none} />}
                  size="17 lines · 1.2 KB"
                  tools={
                    <Link to="/" className={buttonClass({ size: "small" })}>
                      <Icon name="eye" size={12} /> Changes
                    </Link>
                  }
                />
                <FilesFileMeta status="modified" base="main" uncommitted legend={<FilesChangeLegend added modified deleted />} />
                <Code picked={[11, 12]} />
              </FilesFile>
            ) : (
              <FilesOverview>
                <Empty
                  icon="file"
                  title="Pick a file"
                  action={
                    <Button onClick={none}>
                      <Icon name="search" /> Find a file <Keys k="t" />
                    </Button>
                  }
                >
                  14 files; 5 changed since the capsule left main.
                </Empty>
                <FilesChanged
                  count={5}
                  onOpen={none}
                  files={[
                    { path: "src/report/summary.ts", status: "added", uncommitted: true },
                    { path: "src/invoices/money.ts", status: "modified" },
                    { path: "src/invoices/sort.ts", status: "deleted" },
                    { path: "src/styles/theme.css", status: "renamed" },
                    { path: "src/types.ts", status: "copied" },
                  ]}
                />
              </FilesOverview>
            )}
          </FilesView>
        </FilesBody>
      </FilesExplorer>
    </div>
  );
}

export const WithAFileOpen: StoryObj = { render: () => <Explorer /> };

export const WithNoneOpen: StoryObj = { render: () => <Explorer file={false} /> };

export const TheFileAlone: StoryObj = { render: () => <Explorer panes={1} /> };

const box = { display: "grid", gap: 16, padding: 24, maxWidth: 760 } as const;
const line = { display: "flex", gap: 12, alignItems: "center", flexWrap: "wrap" } as const;

// The tree's head: what it shows (changed, or all), a ground's count, and
// one with no branch point to compare with yet.
export const TreeTools: StoryObj = {
  render: () => (
    <div style={{ ...box, maxWidth: 300 }}>
      <FilesTreeTools all={14} changed={5} changedOnly={false} canChange onShow={none} onFind={none} onSearch={none} />
      <FilesTreeTools all={14} changed={5} changedOnly canChange onShow={none} onFind={none} onSearch={none} />
      <FilesTreeTools all={14} changed={0} changedOnly={false} canChange={false} onShow={none} onFind={none} />
      <FilesTreeTools all={1} changed={0} counted changedOnly={false} canChange={false} onShow={none} onFind={none} />
      <FilesTreeTools all={204} changed={0} counted changedOnly={false} canChange={false} onShow={none} onFind={none} />
    </div>
  ),
};

// Every way a file's changed, and a tree read so far (loading), empty,
// and cut short.
export const TreeStates: StoryObj = {
  render: () => (
    <div style={{ display: "flex", gap: 24, height: 420, padding: 24 }}>
      {[
        <Tree key="t" />,
        <Loading key="l" what="Reading the files…" />,
        <Empty key="e" icon="check-circle" title="Nothing's changed yet">
          Nothing here differs from main, committed or not.
        </Empty>,
      ].map((rows, i) => (
        <div key={i} style={{ width: 300, display: "flex" }}>
          <FilesTree panel={i === 0} tools={<FilesTreeTools all={14} changed={5} changedOnly={i === 2} canChange onShow={none} onFind={none} />} note={i === 0 ? "Only the first 10 files are listed." : undefined}>
            {rows}
          </FilesTree>
        </div>
      ))}
    </div>
  ),
};

export const Marks: StoryObj = {
  render: () => (
    <div style={box}>
      {(["added", "modified", "deleted", "renamed", "copied", "type", undefined] as const).map((s) => (
        <div key={s ?? "none"} style={line}>
          <FilesMark status={s} uncommitted={s === "added"} /> {s ?? "unchanged: a blank, so names line up"}
        </div>
      ))}
      <FilesChangeLegend added modified deleted />
      <FilesChangeLegend added />
    </div>
  ),
};

// Code: changed lines marked, a range picked, a fold shut (press its
// mark to open it), and the gutter as wide as the last line's number.
export const CodeLines: StoryObj = {
  render: () => (
    <div style={{ padding: 24 }}>
      <Code picked={[10, 12]} />
    </div>
  ),
};

// What's said of the open file: changed since main, not committed, renamed
// from where it was, unchanged, and a ground's (nothing to compare with).
export const FileMeta: StoryObj = {
  render: () => (
    <div style={{ display: "grid", gap: 8 }}>
      <FilesFileMeta status="added" base="main" uncommitted legend={<FilesChangeLegend added />} />
      <FilesFileMeta status="modified" base="main" legend={<FilesChangeLegend added modified deleted />} />
      <FilesFileMeta status="deleted" base="main" />
      <FilesFileMeta status="renamed" base="main" oldPath="src/theme.css" />
      <FilesFileMeta base="main" />
      <FilesFileMeta base="" />
    </div>
  ),
};

const logo = `data:image/svg+xml,${encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" width="120" height="60"><rect width="120" height="60" rx="10" fill="#3b6fd4"/><circle cx="30" cy="30" r="14" fill="#fff"/></svg>')}`;

// A file's head and what's in place of its text: narrow, with the way
// back; markdown, rendered or as text, to edit as a page; a picture; and
// a file deleted, binary, too large, empty, or that couldn't be opened.
export const FilePanes: StoryObj = {
  render: () => {
    const [md, setMd] = useState("rendered");
    return (
      <div style={{ display: "grid", gap: 24 }}>
        <FilesFile>
          <FilesFileHead
            onBack={none}
            path={<FilesFilePath path="docs/guide.md" onReveal={none} />}
            size="31 lines · 712 B"
            tools={
              <>
                <Segmented value={md} onChange={setMd} aria-label="Markdown">
                  <Segment value="rendered">Rendered</Segment>
                  <Segment value="text">Text</Segment>
                </Segmented>
                <Link to="/" className={buttonClass({ size: "small" })}>
                  <Icon name="pencil" size={12} /> Edit as a page <Keys k="⏎" />
                </Link>
              </>
            }
          />
          <FilesFileMeta base="main" />
          <FilesMarkdown>
            <Markdown text={"# The dashboard guide\n\nHow the dashboard is put together, and where to change it.\n\n## Invoices\n\nThe list of invoices is in `src/invoices`."} />
          </FilesMarkdown>
        </FilesFile>
        <FilesFile>
          <FilesFileHead path={<FilesFilePath path="img/logo.svg" onReveal={none} />} size="412 B" />
          <FilesFileMeta status="added" base="main" />
          <FilesImage src={logo} path="img/logo.svg" />
        </FilesFile>
        <FilesFile>
          <FilesFileHead path={<FilesFilePath path="src/invoices/sort.ts" onReveal={none} />} />
          <FilesFileMeta status="deleted" base="main" />
          <Empty className={filesFileNoteClass} icon="file" title="A binary file">
            2.4 MB: there's no text to show.
          </Empty>
          <Empty className={filesFileNoteClass} icon="file" title="An empty file" />
          <Loading className={filesFileNoteClass} what="Opening it…" />
        </FilesFile>
      </div>
    );
  },
};

// A file asked for that isn't in this worktree.
export const NotHere: StoryObj = {
  render: () => (
    <FilesOverview>
      <Empty icon="file" title="Not here" action={<Button onClick={none}>Pick another file</Button>}>
        There's no <Text mono>src/old.ts</Text> in web/late-invoices.
      </Empty>
    </FilesOverview>
  ),
};

// The panes: the button on each, and what each choice is drawn as. The
// menu opens from the button.
export const PanesPicked: StoryObj = {
  render: () => (
    <div style={box}>
      {([3, 2, 1] as Panes[]).map((n) => {
        const p = paneChoices(n, 3, false);
        return (
          <div key={n} style={line}>
            <FilesPanes now={p.now} choices={p.choices} onPick={none} /> {p.now.label}: {p.now.sub}
          </div>
        );
      })}
      <div style={line}>
        {(() => {
          const p = paneChoices(3, 3, true);
          return <FilesPanes now={p.now} choices={p.choices} onPick={none} />;
        })()}{" "}
        On another tab: whether the repos are beside it
      </div>
      <div style={line}>
        <FilesPanesGlyph n={3} />
        <FilesPanesGlyph n={2} />
        <FilesPanesGlyph n={1} />
      </div>
    </div>
  ),
};

const paths = ["src/report/summary.ts", "src/invoices/money.ts", "src/invoices/InvoiceList.tsx", "src/api.ts", "src/App.tsx", "docs/guide.md", "docs/guides/setup.md", "README.md", "package.json"];
const status: Record<string, "added" | "modified"> = { "src/report/summary.ts": "added", "src/invoices/money.ts": "modified" };

function Finding({ typed }: { typed: string }) {
  const [q, setQ] = useState(typed);
  const found = q ? paths.flatMap((p) => (fuzzy(p, q) ? [{ p, at: fuzzy(p, q)!.at }] : [])) : paths.map((p) => ({ p, at: [] as number[] }));
  return (
    <Finder aria-label="Find a file" placeholder="Find a file by a few of its letters…" value={q} onChange={setQ} count={found.length} onPick={none} onClose={none}>
      {found.map((f, i) => (
        <FilesFinderRow key={f.p} index={i} path={f.p} runs={marked(f.p, f.at)} status={status[f.p]} uncommitted={f.p === "src/report/summary.ts"} />
      ))}
      {!found.length && <FinderNone>No file matches “{q}”.</FinderNone>}
    </Finder>
  );
}

// Finding a file: before typing (what's changed first), and with letters
// typed, marked in each path.
export const FindAFile: StoryObj = { render: () => <Finding typed="" /> };
export const FindAFileTyped: StoryObj = { render: () => <Finding typed="sum" /> };
