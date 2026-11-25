import { type MouseEvent, type ReactNode, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { useLocation, useNavigate } from "react-router";
import type { FileView } from "../../api";
import { useKeys } from "../../keys";
import { useCodeNav } from "../../review/CodeNav";
import { escapeHTML, highlightLines, languageOf } from "../../review/highlight";
import { getNames, type NavNames } from "../../review/nav";
import { foldsOf, unfolded } from "../folds";
import { linesAsked } from "../lines";
import { FilesCode, FilesCodeLine, type LineMark } from "./FilesCode";

// FileCode is a file's lines, highlighted, numbered, the changed ones
// marked in the gutter. A line number links to the line (#L12; shift-click
// for a range). With a commit to read them at (rev), its names are links,
// as in a review: hold ⌘ over one, or d on the picked line. A declaration,
// a scope or a run of lines further in folds away at the mark beside its
// first line (../folds), and opens again when a line in it is gone to.
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
  const marks = useMemo(() => {
    const m = new Map<number, LineMark>();
    for (const c of file.changes) {
      if (c.kind === "deleted") {
        m.set(c.start, m.get(c.start) ? m.get(c.start)! : "deleted-after");
        continue;
      }
      for (let n = c.start; n <= (c.end ?? c.start); n++) m.set(n, c.kind);
    }
    return m;
  }, [file]);
  const sel = linesAsked(location.hash);
  const from = sel?.from ?? 0;
  const to = sel?.to ?? 0;
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
  // What's folded, by the line it starts at: of this file, and opened
  // where a line in it is asked for.
  const asked = `${path}#${from}-${to}`;
  const [fold, setFold] = useState<{ path: string; asked: string; at: ReadonlySet<number> }>({ path, asked, at: new Set() });
  let folded = fold.path === path ? fold.at : new Set<number>();
  if (fold.asked !== asked || fold.path !== path) {
    folded = unfolded(folds, folded, [from, to]);
    setFold({ path, asked, at: folded });
  }
  const toggle = (at: number) => {
    const next = new Set(folded);
    if (!next.delete(at)) next.add(at);
    setFold({ path, asked, at: next });
  };
  const foldAt = useMemo(() => new Map(folds.map((f) => [f.at, f])), [folds]);
  // To the line asked for (a link to it), not each time you click one.
  const clicked = (location.state as { clicked?: boolean } | null)?.clicked;
  useLayoutEffect(() => {
    if (from && !clicked) body.current?.querySelector(`[data-line="${from}"]`)?.scrollIntoView({ block: "center" });
  }, [file.path, from, clicked]);
  const pick = (n: number, e: MouseEvent) => {
    const hash = e.shiftKey && from ? `#L${Math.min(from, n)}-L${Math.max(from, n)}` : `#L${n}`;
    navigate({ search: location.search, hash }, { replace: true, state: { clicked: true } });
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
    sideWords: () => "as the files are now",
  });
  useKeys(rev ? codeNav.keys : {});
  const lines: ReactNode[] = [];
  let hiddenTo = 0;
  html.forEach((h, i) => {
    const n = i + 1;
    if (n <= hiddenTo) return;
    const f = foldAt.get(n);
    const shut = !!f && folded.has(n);
    if (shut) hiddenTo = f.to;
    lines.push(
      <FilesCodeLine
        key={n}
        n={n}
        html={h}
        mark={marks.get(n)}
        picked={n >= from && n <= to}
        fold={f && { to: f.to, shut }}
        rev={rev}
        path={path}
        onPick={(e) => pick(n, e)}
        onFold={() => toggle(n)}
      />,
    );
  });
  return (
    <FilesCode ref={body} digits={Math.max(2, String(html.length).length)}>
      {codeNav.card}
      {codeNav.picker}
      {codeNav.panel}
      {lines}
    </FilesCode>
  );
}
