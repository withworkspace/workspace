import { Link } from "react-router";
import { cx } from "../ui/cx";
import css from "./TalkProposalWhere.module.css";

const tones: Record<string, string> = { blocking: css.blocking, suggestion: css.suggestion, question: css.question, ask: css.question, nit: css.nit, note: css.nit, change: css.change };

// TalkProposalWhere is where a review comment an agent proposed goes: its
// kind (blocking, suggestion, question, nit), the pull request (leading
// to it), the file and its lines (removed: on the old side).
export function TalkProposalWhere({ kind, pr, href, path, lines, removed }: { kind?: string; pr?: string; href: string; path?: string; lines: string; removed?: boolean }) {
  return (
    <div className={css.where}>
      <span className={cx(css.kind, kind && tones[kind])}>{kind}</span>
      <Link to={href} className="link mono">
        {pr}
      </Link>
      <span className="mono dim">
        {path}:{lines}
        {removed ? " (removed)" : ""}
      </span>
    </div>
  );
}
