import { useLoaded } from "../load";
import { highlightCode, languageOf, useHighlighter } from "../review/highlight";
import { HoverCardLine } from "../ui/HoverCard";
import { excerptRoom, fileOf } from "./cards";
import { goneCapsule, GROUND, type Known, type PageRef, refHref } from "./refs";
import { RefCardHead } from "./RefCardHead";
import { RefExcerpt, RefExcerptLine } from "./RefExcerpt";
import { RefGo } from "./RefGo";
import { RefGone } from "./RefGone";

// RefFileCard is a file's card: where it is, and the lines named, with a
// line either side (or the file's start).
export function RefFileCard({ r: named, known, pane }: { r: Extract<PageRef, { kind: "file" }>; known?: Known; pane?: boolean }) {
  // A capsule that's gone is read on the default branch, where its file
  // most likely is now.
  const gone = goneCapsule(named, known);
  const r = gone ? { ...named, worktree: GROUND } : named;
  const { value: file, error } = useLoaded(() => fileOf(r.repo, r.worktree, r.path), [r.repo, r.worktree, r.path]);
  useHighlighter();
  // The lines named, with a line either side; or the file's start.
  const from = r.line ? Math.max(1, r.line - 1) : 1;
  const wanted = r.line ? (r.endLine ?? r.line) + 1 : excerptRoom;
  const to = file ? Math.min(file.lines.length, Math.max(wanted, from), from + excerptRoom - 1) : 0;
  const lines = file ? file.lines.slice(from - 1, to) : [];
  const asked = (n: number) => !!r.line && n >= r.line && n <= (r.endLine ?? r.line);
  const where = r.worktree === GROUND ? "default branch" : r.worktree;
  return (
    <>
      <RefCardHead icon="file" title={r.path} mono />
      <HoverCardLine dim>
        {r.repo}, {where}
        {r.line ? `, line${r.endLine && r.endLine > r.line ? `s ${r.line} to ${r.endLine}` : ` ${r.line}`}` : file ? `, ${file.lines.length} lines` : ""}
      </HoverCardLine>
      {gone && <RefGone r={named} there={!!file} />}
      {error && !gone && <HoverCardLine dim>It can't be read: {error}</HoverCardLine>}
      {named.commit && !gone && <HoverCardLine dim>Named at {named.commit.slice(0, 7)}.</HoverCardLine>}
      {file?.binary && <HoverCardLine dim>Not text.</HoverCardLine>}
      {file?.tooLarge && <HoverCardLine dim>Too large to show here.</HoverCardLine>}
      {r.line && file && r.line > file.lines.length && <HoverCardLine dim>The file has only {file.lines.length} lines now.</HoverCardLine>}
      {lines.length > 0 && (
        <RefExcerpt raised={pane} more={!!file && to < file.lines.length}>
          {lines.map((l, i) => (
            <RefExcerptLine key={i} n={from + i} named={asked(from + i)} html={highlightCode(l, languageOf(r.path)) || " "} />
          ))}
        </RefExcerpt>
      )}
      <RefGo to={refHref(r, known)}>Open it in Files</RefGo>
    </>
  );
}
