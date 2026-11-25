import type { Meta, StoryObj } from "@storybook/react-vite";
import { type CSSProperties, type ReactNode, useMemo, useState } from "react";
import type { DiffFile } from "../api";
import { escapeHTML, highlightLines, languageOf, useHighlighter } from "../review/highlight";
import { type Expansion, fileRows, type Row } from "../review/model";
import { diffOf, goBefore, goNow } from "../stories/code";
import { Button } from "./Button";
import { DiffLines } from "./DiffLines";
import type { DiffCell, DiffPick } from "./diffLayout";
import { NoteBody, NoteCard, NoteHead, NoteId } from "./NoteCard";
import { Segment, Segmented } from "./Segmented";

// A file's diff to read: in one column or side by side, with lines picked
// by their numbers, gaps that open, a comment among the lines, the whole
// file with the change in place, what's said of a file with no lines to
// show, and a long one. The rows are review/model's, as a review's are.

const meta: Meta = { title: "Components/DiffLines" };
export default meta;

const path = "internal/nav/nav.go";
const at = { old: { rev: "0a96360", path }, new: { rev: "88cf294", path } };
const changed = diffOf(goBefore, goNow, path);

// Each side's lines highlighted, once the highlighter's here; a cell's
// line is its side's, by its number there.
function useHTML(before: string[], after: string[], file: string) {
  const ready = useHighlighter();
  return useMemo(() => {
    const lang = ready ? languageOf(file) : undefined;
    const old = lang ? highlightLines(before, lang) : before.map(escapeHTML);
    const now = lang ? highlightLines(after, lang) : after.map(escapeHTML);
    return (c: DiffCell, side: "old" | "new") => (side === "old" ? old : now)[c.num - 1] ?? escapeHTML(c.text);
  }, [before, after, file, ready]);
}

const page: CSSProperties = { display: "flex", gap: 24, alignItems: "start", flexWrap: "wrap", padding: 24 };
const frame: CSSProperties = { border: "1px solid var(--line)", borderRadius: 8, overflow: "hidden" };

function Panel({ title, width = 720, height, children }: { title: ReactNode; width?: number; height?: number; children: ReactNode }) {
  return (
    <figure style={{ margin: 0, width }}>
      <figcaption style={{ display: "flex", gap: 8, alignItems: "center", minHeight: 28, marginBottom: 8, color: "var(--dim)", fontSize: 12 }}>{title}</figcaption>
      <div style={{ ...frame, height, overflow: height ? "auto" : "hidden" }}>{children}</div>
    </figure>
  );
}

// The file's rows, without the header a review gives each file.
const rowsOf = (f: DiffFile, split: boolean, expanded: Record<number, Expansion> = {}, blob: string[] = goNow) =>
  fileRows(0, f, { viewed: false, expanded, blob }, split).filter((r) => r.type !== "file");

const ALL = { top: 1e6, bottom: 0 };

// One column or side by side. Press a number to pick its line, shift-press
// for the run, or drag down the numbers (side by side, in one column of
// them). Press a gap to open it.
function Reading() {
  const html = useHTML(goBefore, goNow, path);
  const [split, setSplit] = useState(false);
  const [picked, setPicked] = useState<DiffPick>();
  const [cursor, setCursor] = useState<string>();
  const [expanded, setExpanded] = useState<Record<number, Expansion>>({});
  const rows = useMemo(() => rowsOf(changed, split, expanded), [split, expanded]);
  return (
    <Panel
      width={split ? 1040 : 720}
      title={
        <>
          <Segmented value={split ? "split" : "unified"} onChange={(v) => (setSplit(v === "split"), setPicked(undefined))} size="small" aria-label="Layout">
            <Segment value="unified">Unified</Segment>
            <Segment value="split">Side by side</Segment>
          </Segmented>
          <span>{picked ? `Picked ${picked.from}${picked.to === picked.from ? "" : ` to ${picked.to}`}${picked.side ? `, ${picked.side} side` : ""}` : "Nothing picked"}</span>
        </>
      }
    >
      <DiffLines
        aria-label={`Changes to ${path}`}
        rows={rows}
        split={split}
        html={html}
        at={at}
        picked={picked}
        onPick={setPicked}
        cursor={cursor}
        onRow={setCursor}
        onGap={(r) => r.type === "gap" && setExpanded((e) => ({ ...e, [r.gap]: ALL }))}
      />
    </Panel>
  );
}

export const Changes: StoryObj = {
  render: () => (
    <div style={page}>
      <Reading />
    </div>
  ),
};

// A row of the page's own among the lines: a comment on the line above
// it, level with the code in either layout.
type Said = { type: "said"; key: string; file: number };

function WithComment({ split }: { split?: boolean }) {
  const html = useHTML(goBefore, goNow, path);
  const rows = useMemo(() => {
    const out: (Row | Said)[] = [];
    for (const r of rowsOf(changed, !!split)) {
      out.push(r);
      if (r.type === "line" && r.right?.num === 21) out.push({ type: "said", key: "said", file: 0 });
    }
    return out;
  }, [split]);
  return (
    <Panel title={split ? "Side by side" : "A comment on line 21"} width={split ? 900 : 620} height={split ? 420 : 400}>
      <DiffLines
        aria-label={`Changes to ${path}`}
        rows={rows}
        split={split}
        html={html}
        picked={{ from: "0:n21", to: "0:n21", side: split ? "new" : undefined }}
        draw={(r) =>
          r.type === "said" && (
            <div style={{ padding: "8px 12px 8px 0" }}>
              <NoteCard>
                <NoteHead end={<NoteId>n1</NoteId>}>
                  <strong>You</strong>
                </NoteHead>
                <NoteBody>Is this bounded now? The map never was.</NoteBody>
              </NoteCard>
            </div>
          )
        }
      />
    </Panel>
  );
}

export const Comments: StoryObj = {
  render: () => (
    <div style={page}>
      <WithComment />
      <WithComment split />
    </div>
  ),
};

// Every gap open: the whole file, with the change in place. The lines
// opened up are quieter than the diff's own.
function Whole() {
  const html = useHTML(goBefore, goNow, path);
  const [open, setOpen] = useState(true);
  const rows = useMemo(() => rowsOf(changed, false, open ? Object.fromEntries(changed.hunks.map((_, i) => [i, ALL]).concat([[changed.hunks.length, ALL]])) : {}), [open]);
  return (
    <Panel
      height={520}
      title={
        <>
          {open ? "The whole file, the change in place" : "Only the change"}
          <Button size="small" onClick={() => setOpen((o) => !o)}>
            {open ? "Close the gaps" : "Open every gap"}
          </Button>
        </>
      }
    >
      <DiffLines aria-label={`Changes to ${path}`} rows={rows} html={html} cursor="0:n13" />
    </Panel>
  );
}

export const WholeFile: StoryObj = {
  render: () => (
    <div style={page}>
      <Whole />
    </div>
  ),
};

// What a diff is when it isn't a few changed lines: a file added, one
// deleted, a last line with no newline, and files with no lines to show
// (one that's pressed for them).
const added = diffOf([], goNow.slice(0, 8), "internal/nav/doc.go");
const deleted = diffOf(goNow.slice(11, 22), [], "internal/nav/old.go");
const noNewline: DiffFile = (() => {
  const f = diffOf(['{', '  "name": "ws-web"', "}"], ['{', '  "name": "ws-web",', '  "private": true', "}"], "package.json");
  const last = f.hunks[0]!.lines.at(-1)!;
  last.noeol = true;
  return f;
})();
const none = (over: Partial<DiffFile>): DiffFile => ({ path: "x", status: "modified", additions: 0, deletions: 0, hunks: [], ...over });

function Shape({ title, file, before = [], after = [], split, width = 440 }: { title: string; file: DiffFile; before?: string[]; after?: string[]; split?: boolean; width?: number }) {
  const html = useHTML(before, after, file.path);
  const [shown, setShown] = useState<string>();
  const rows = useMemo(() => rowsOf(file, !!split, {}, after), [file, split, after]);
  return (
    <Panel title={shown ? `${title}: asked for` : title} width={width}>
      <DiffLines aria-label={`Changes to ${file.path}`} rows={rows} split={split} html={html} onNote={(r) => setShown(r.key)} />
    </Panel>
  );
}

export const Shapes: StoryObj = {
  render: () => (
    <div style={page}>
      <Shape title="Added" file={added} after={goNow.slice(0, 8)} width={560} />
      <Shape title="Deleted" file={deleted} before={goNow.slice(11, 22)} width={560} />
      <Shape title="Added, side by side" file={added} after={goNow.slice(0, 8)} split width={760} />
      <Shape title="No newline at the end" file={noNewline} before={['{', '  "name": "ws-web"', "}"]} after={['{', '  "name": "ws-web",', '  "private": true', "}"]} />
      <Shape title="Binary" file={none({ path: "logo.png", binary: true })} />
      <Shape title="Renamed, unchanged" file={none({ path: "new.go", oldPath: "internal/nav/old.go", status: "renamed" })} />
      <Shape title="Too large to show until asked" file={none({ path: "pnpm-lock.yaml", large: true, additions: 4210, deletions: 3977 })} />
    </div>
  ),
};

// A long diff: six thousand rows, laid out only near what's in view.
const longBefore = Array.from({ length: 3000 }, (_, i) => goNow[i % goNow.length]!);
const longNow = longBefore.map((l, i) => (i % 9 === 4 ? `${l} // touched` : l)).filter((_, i) => i % 31 !== 7);
const long = diffOf(longBefore, longNow, path, 3);

function LongDiff() {
  const html = useHTML(longBefore, longNow, path);
  const rows = useMemo(() => rowsOf(long, false, {}, longNow), []);
  const [picked, setPicked] = useState<DiffPick>();
  return (
    <Panel title={`${rows.length.toLocaleString()} rows`} height={460}>
      <DiffLines aria-label={`Changes to ${path}`} rows={rows} html={html} picked={picked} onPick={setPicked} />
    </Panel>
  );
}

export const Long: StoryObj = {
  render: () => (
    <div style={page}>
      <LongDiff />
    </div>
  ),
};
