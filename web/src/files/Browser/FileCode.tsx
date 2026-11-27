import { useEffect, useMemo, useRef, useState } from "react";
import { useLocation, useNavigate } from "react-router";
import type { FileView } from "../../api";
import { useCommands } from "../../keymap/useCommands";
import { reviewNavKeys } from "../../review/keys";
import { useCodeNav } from "../../review/CodeNav";
import { escapeHTML, highlightLines, languageOf } from "../../review/highlight";
import { getNames, type NavNames } from "../../review/nav";
import { CodeLines } from "../../ui/CodeLines";
import { type CodeRange, marksOf } from "../../ui/codeLayout";
import { foldsOf } from "../folds";
import { linesAsked } from "../lines";
import { FilesCode, filesCodeLinesClass } from "./FilesCode";

// FileCode is a file's lines (ui/CodeLines draws them): highlighted,
// numbered, the changed ones marked. A line's number links to the line
// (#L12; shift-press or drag for a run). With a commit to read them at
// (rev), its names are links, as in a review: hold ⌘ over one, or d on
// the picked line. A declaration, a scope or a run of lines further in
// folds away at the twist beside its first line (../folds), and opens
// again when a line in it is gone to.
export function FileCode({
  file,
  path,
  ready,
  api,
  rev,
  onDefinition,
  onBackFromDefinition,
}: {
  file: FileView;
  path: string;
  ready: boolean;
  api: string;
  rev?: string;
  onDefinition: (path: string, line: number) => void;
  onBackFromDefinition: () => boolean;
}) {
  const location = useLocation();
  const navigate = useNavigate();
  const lang = languageOf(path);
  // Plain until the highlighter's loaded (ready), then drawn again.
  const html = useMemo(() => (ready && file.lines.length <= 20000 ? highlightLines(file.lines, lang) : file.lines.map(escapeHTML)), [file, lang, ready]);
  const marks = useMemo(() => marksOf(file.changes), [file]);
  const sel = linesAsked(location.hash);
  const from = sel?.from ?? 0;
  const picked = useMemo(() => (sel ? { from: sel.from, to: sel.to } : undefined), [sel?.from, sel?.to]);
  const body = useRef<HTMLDivElement>(null);
  // What folds: by the file's names where they're read, by indentation
  // until then and where there are none.
  const namesKey = rev ? `${api}\u0000${rev}\u0000${path}` : "";
  const [names, setNames] = useState<{ key: string; names: NavNames }>();
  useEffect(() => {
    if (!rev) return;
    let live = true;
    getNames(api, rev, path).then(
      (n) => live && setNames({ key: namesKey, names: n }),
      () => {},
    );
    return () => {
      live = false;
    };
  }, [api, rev, path, namesKey]);
  const found = names?.key === namesKey ? names.names.folds : undefined;
  const folds = useMemo(() => foldsOf(file.lines, found), [file, found]);
  // A line asked for (a link to it) is gone to: brought into view, a fold
  // over it opened. Not one you've just pressed the number of.
  const clicked = (location.state as { clicked?: boolean } | null)?.clicked;
  const pick = (lines: CodeRange) => {
    const hash = lines.from === lines.to ? `#L${lines.from}` : `#L${lines.from}-L${lines.to}`;
    if (hash !== location.hash) navigate({ search: location.search, hash }, { replace: true, state: { clicked: true } });
  };
  const codeNav = useCodeNav({
    api,
    diff: rev ? { base: rev, head: rev } : undefined,
    files: [],
    rows: [],
    states: [],
    pane: body,
    expand: async () => {},
    goToRow: () => {},
    open: onDefinition,
    back: onBackFromDefinition,
    line: from && file.lines[from - 1] !== undefined ? { path, line: from, text: file.lines[from - 1] } : undefined,
    sideWords: () => "current files",
  });
  useCommands(reviewNavKeys, rev ? codeNav.keys : {});
  return (
    <FilesCode ref={body}>
      {codeNav.card}
      {codeNav.picker}
      {codeNav.panel}
      {/* What's folded is the lines' own, of this file: another's start with none. */}
      <CodeLines key={path} className={filesCodeLinesClass} html={html} path={path} rev={rev} marks={marks} picked={picked} onPick={pick} folds={folds} reveal={from && !clicked ? from : undefined} />
    </FilesCode>
  );
}
