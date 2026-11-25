import type { Meta, StoryObj } from "@storybook/react-vite";
import { type CSSProperties, type ReactNode, useMemo, useState } from "react";
import { foldsOf } from "../files/folds";
import { escapeHTML, highlightLines, languageOf, useHighlighter } from "../review/highlight";
import { Button } from "./Button";
import { CodeLines } from "./CodeLines";
import { type CodeChange, type CodeRange, marksOf } from "./codeLayout";
import { NoteBody, NoteCard, NoteHead, NoteId } from "./NoteCard";
import treeSource from "./Tree.tsx?raw";

// A file's lines to read: highlighted and numbered, with lines picked by
// their numbers, changes marked, folds, something said after a line, a
// line gone to, long lines scrolled or wrapped, and a file of thousands.

const meta: Meta = { title: "Components/CodeLines" };
export default meta;

const go = `// Package nav is code navigation for reviews and the explorer: which
// words in a file are names, where a name is defined, and where a
// definition is used.
package nav

import (
	"errors"
	"fmt"
	"path/filepath"
)

// CacheMax is the most the facts kept on disk take up.
const CacheMax = 200 << 20

// Service answers questions about names, for any repo at any commit.
type Service struct {
	Runtime Runtime
	disk    disk
	facts   *lru[*Facts]   // by blob and grammar
	trees   *lru[*gitTree] // by git directory and commit
	answers *lru[*Answer]  // by question: a commit's don't change
}

// New is a service that keeps what it parses in cacheDir ("" for memory
// only), and prunes it there now and then.
func New(cacheDir string) *Service {
	s := &Service{
		Runtime: Builtin(),
		disk:    disk{dir: cacheDir},
		facts:   newLRU[*Facts](2000),
		trees:   newLRU[*gitTree](8),
		answers: newLRU[*Answer](5000),
	}
	if cacheDir != "" {
		go s.disk.prune(CacheMax)
	}
	return s
}

// Tree is a git directory's files at a commit.
func (s *Service) Tree(dir, rev string) (Tree, error) {
	if rev == "" {
		return nil, errors.New("nav: no commit to read " + filepath.Base(dir) + " at, which is what a worktree with nothing committed in it has, so there's nothing to say of its names yet")
	}
	t, err := s.tree(dir, rev)
	if err != nil {
		return nil, fmt.Errorf("nav: %w", err)
	}
	return t, nil
}
`;

const goLines = go.trimEnd().split("\n");
const tsxLines = treeSource.trimEnd().split("\n");

// The lines highlighted, once the highlighter's here: plain until then.
function useHTML(lines: string[], path: string): string[] {
  const ready = useHighlighter();
  return useMemo(() => (ready ? highlightLines(lines, languageOf(path)) : lines.map(escapeHTML)), [lines, path, ready]);
}

const page: CSSProperties = { display: "flex", gap: 24, alignItems: "start", flexWrap: "wrap", padding: 24 };
const frame: CSSProperties = { border: "1px solid var(--line)", borderRadius: 8, overflow: "hidden" };

function Panel({ title, width = 560, height, children }: { title: ReactNode; width?: number; height?: number; children: ReactNode }) {
  return (
    <figure style={{ margin: 0, width }}>
      <figcaption style={{ marginBottom: 8, color: "var(--dim)", fontSize: 12 }}>{title}</figcaption>
      <div style={{ ...frame, height, display: height ? "grid" : undefined }}>{children}</div>
    </figure>
  );
}

// Lines picked by their numbers: press one, shift-press another for the
// run between, or drag down the numbers.
function Picking() {
  const html = useHTML(goLines, "nav.go");
  const [picked, setPicked] = useState<CodeRange | undefined>({ from: 27, to: 29 });
  return (
    <Panel title={picked ? `Picked: ${picked.from === picked.to ? `line ${picked.from}` : `lines ${picked.from} to ${picked.to}`}` : "Nothing picked"}>
      <CodeLines html={html} path="internal/nav/nav.go" rev="0a96360" picked={picked} onPick={setPicked} />
    </Panel>
  );
}

function Plain() {
  return (
    <Panel title="No highlighter for it: plain" width={360}>
      <CodeLines html={["just build", "just test", "", "# a recipe file nothing parses", "try *args:", "    @./scripts/try.sh {{args}}"].map(escapeHTML)} path="notes.txt" />
    </Panel>
  );
}

export const Reading: StoryObj = {
  render: () => (
    <div style={page}>
      <Picking />
      <Plain />
    </div>
  ),
};

// How lines have changed since the file's base, beside their numbers:
// added, modified, and lines removed after one.
const changes: CodeChange[] = [
  { kind: "added", start: 6, end: 7 },
  { kind: "modified", start: 13 },
  { kind: "deleted", start: 21 },
  { kind: "added", start: 34, end: 36 },
  { kind: "modified", start: 42, end: 44 },
];

function Marked() {
  const html = useHTML(goLines, "nav.go");
  const marks = useMemo(() => marksOf(changes), []);
  return (
    <Panel title="Added, modified, and removed after line 21">
      <CodeLines html={html} path="internal/nav/nav.go" marks={marks} />
    </Panel>
  );
}

export const Changes: StoryObj = {
  render: () => (
    <div style={page}>
      <Marked />
    </div>
  ),
};

// What folds, by indentation here (files/folds): point at the code for
// the twists. One's folded to begin with; the page keeps what's folded
// in the second, and folds or opens all of it.
function Folds() {
  const html = useHTML(goLines, "nav.go");
  const folds = useMemo(() => foldsOf(goLines), []);
  const [folded, setFolded] = useState<ReadonlySet<number>>(new Set([16, 26]));
  return (
    <>
      <Panel title="Folds of its own: point at the code, press a twist">
        <CodeLines html={html} path="internal/nav/nav.go" folds={folds} />
      </Panel>
      <Panel
        title={
          <span style={{ display: "flex", gap: 6, alignItems: "center" }}>
            The page's: {folded.size} folded
            <Button size="small" onClick={() => setFolded(new Set(folds.map((f) => f.at)))}>
              Fold all
            </Button>
            <Button size="small" onClick={() => setFolded(new Set())}>
              Open all
            </Button>
          </span>
        }
      >
        <CodeLines html={html} path="internal/nav/nav.go" folds={folds} folded={folded} onFold={setFolded} />
      </Panel>
    </>
  );
}

export const Folding: StoryObj = {
  render: () => (
    <div style={page}>
      <Folds />
    </div>
  ),
};

// What the page puts after a line: a comment on it, here. It's as wide
// as the code's box and stays put while the long line above it scrolls.
function Said() {
  const html = useHTML(goLines, "nav.go");
  const marks = useMemo(() => marksOf([{ kind: "modified", start: 43 }]), []);
  return (
    <Panel title="A comment after line 43, which scrolls sideways over it">
      <CodeLines
        html={html}
        path="internal/nav/nav.go"
        marks={marks}
        picked={{ from: 43, to: 43 }}
        after={(n) =>
          n === 43 && (
            <div style={{ padding: "8px 12px" }}>
              <NoteCard>
                <NoteHead end={<NoteId>n2</NoteId>}>
                  <strong>You</strong>
                </NoteHead>
                <NoteBody>This error's a paragraph. Say there's no commit, and leave the rest to the caller.</NoteBody>
              </NoteCard>
            </div>
          )
        }
      />
    </Panel>
  );
}

export const After: StoryObj = {
  render: () => (
    <div style={page}>
      <Said />
    </div>
  ),
};

// A line gone to: it's brought to the middle of the box and flashed. A
// fold of the code's own that hides it opens.
function GoneTo() {
  const html = useHTML(tsxLines, "Tree.tsx");
  const folds = useMemo(() => foldsOf(tsxLines), []);
  const [line, setLine] = useState<number>();
  return (
    <Panel
      height={360}
      title={
        <span style={{ display: "flex", gap: 6, alignItems: "center" }}>
          {line ? `At line ${line}` : "Go to a line"}
          {[12, 70, 140].map((n) => (
            <Button key={n} size="small" onClick={() => setLine(n)}>
              Line {n}
            </Button>
          ))}
        </span>
      }
    >
      <CodeLines html={html} path="web/src/ui/Tree.tsx" folds={folds} reveal={line} picked={line ? { from: line, to: line } : undefined} />
    </Panel>
  );
}

export const Arriving: StoryObj = {
  render: () => (
    <div style={page}>
      <GoneTo />
    </div>
  ),
};

// Long lines scroll sideways, the numbers staying at the edge; or wrap.
function Long({ wrap }: { wrap?: boolean }) {
  const html = useHTML(goLines, "nav.go");
  return (
    <Panel title={wrap ? "Wrapped" : "Scrolled sideways: the numbers stay"} width={380} height={260}>
      <CodeLines html={html} path="internal/nav/nav.go" wrap={wrap} reveal={45} />
    </Panel>
  );
}

export const LongLines: StoryObj = {
  render: () => (
    <div style={page}>
      <Long />
      <Long wrap />
    </div>
  ),
};

// Twenty thousand lines in a box: only those near what's in view are
// laid out, so it opens and scrolls as a short file does.
const manyLines = Array.from({ length: 20000 }, (_, i) => goLines[i % goLines.length]!);

function ManyLines() {
  const html = useHTML(manyLines, "nav.go");
  const [picked, setPicked] = useState<CodeRange>();
  return (
    <Panel title="20,000 lines" height={420}>
      <CodeLines html={html} path="internal/nav/nav.go" picked={picked} onPick={setPicked} />
    </Panel>
  );
}

export const Many: StoryObj = {
  render: () => (
    <div style={page}>
      <ManyLines />
    </div>
  ),
};
