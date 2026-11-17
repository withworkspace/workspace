import { useMedia } from "../ui/kit";
import { type CodeHit, codeLead, codeLeadNarrow, codeRuns } from "./find";

// How a code search hit's row says it, in Find and in the explorer's own
// search: where (the path cut from its start, so its name and :line stay),
// then the line from just before its first match, the matches marked.

export function CodeWhere({ h, repo, capsule }: { h: CodeHit; repo?: boolean; capsule?: boolean }) {
  return (
    <span className="find-code-where">
      <span className="find-code-path">
        <span className="find-code-file">
          <bdi>
            {repo && <span className="dim">{h.repo}/</span>}
            {h.path}
          </bdi>
        </span>
        <span className="find-code-line dim">:{h.line}</span>
      </span>
      {capsule && h.worktree !== ".ground" && <span className="chip">in {h.worktree}</span>}
    </span>
  );
}

export function CodeText({ h }: { h: CodeHit }) {
  // A phone's row holds about 34 characters of code: less lead-in there.
  const narrow = useMedia("(max-width: 600px)");
  const { runs, clipped } = codeRuns(h.text, h.ranges, narrow ? codeLeadNarrow : codeLead);
  return (
    <code className="find-code-text">
      {(clipped || h.cut) && <span className="dim">…</span>}
      {runs.map((r, i) => (r.hit ? <mark key={i}>{r.text}</mark> : <span key={i}>{r.text}</span>))}
    </code>
  );
}
