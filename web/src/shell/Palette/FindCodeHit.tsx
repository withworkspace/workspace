import { Chip } from "../../ui/Chip";
import { FileIcon } from "../../ui/FileIcon";
import { useMedia } from "../../ui/useMedia";
import { type CodeHit, codeLead, codeLeadNarrow, codeRuns } from "../find";
import css from "./FindCodeHit.module.css";

// FindCodeHit is a line a file says, as a code search found it: the
// file's icon beside two lines, where it is (the path cut from its start,
// so its name and :line stay) and the line from just before its first
// match, the matches marked. repo says which repo's file it is, before
// the path; capsule, the capsule it's in, where that isn't the repo's
// default branch.
//
// It's what a row holds, in Find and in the explorer's own search: the
// row that holds it takes findCodeHitClass.
export function FindCodeHit({ h, repo, capsule }: { h: CodeHit; repo?: boolean; capsule?: boolean }) {
  // A phone's row holds about 34 characters of code: less lead-in there.
  const narrow = useMedia("(max-width: 600px)");
  const { runs, clipped } = codeRuns(h.text, h.ranges, narrow ? codeLeadNarrow : codeLead);
  return (
    <>
      <FileIcon path={h.path} className={css.icon} />
      <span className={css.where}>
        <span className={css.path}>
          <span className={css.file}>
            <bdi>
              {repo && <span className="dim">{h.repo}/</span>}
              {h.path}
            </bdi>
          </span>
          <span className={`${css.line} dim`}>:{h.line}</span>
        </span>
        {capsule && h.worktree !== ".ground" && <Chip>in {h.worktree}</Chip>}
      </span>
      <code className={css.text}>
        {(clipped || h.cut) && <span className="dim">…</span>}
        {runs.map((r, i) => (r.hit ? <mark key={i}>{r.text}</mark> : <span key={i}>{r.text}</span>))}
      </code>
    </>
  );
}

// findCodeHitClass is the look of a row that holds a FindCodeHit: two
// lines beside an icon, with the cursor on it or not (aria-selected).
export const findCodeHitClass = css.row;
