import type { Posting } from "../api";
import { Icon } from "../ui/Icon";
import css from "./PullRequestPosting.module.css";

// PullRequestPosting is exactly what will be posted on a pull request,
// and where: its verdict and commit, its body, each comment on its lines,
// and those whose lines GitHub hasn't, which go in the body.
export function PullRequestPosting({ p }: { p: Posting }) {
  return (
    <div className={css.posting}>
      <div className={`dim ${css.where}`}>
        Goes to{" "}
        <a href={p.url} target="_blank" rel="noreferrer" className="link">
          {p.url.replace(/^https:\/\//, "")}
        </a>{" "}
        as “{p.event === "APPROVE" ? "Approve" : p.event === "REQUEST_CHANGES" ? "Request changes" : "Comment"}”, at commit <span className="mono">{p.commit.slice(0, 7)}</span>.
      </div>
      {p.body && <pre>{p.body}</pre>}
      {p.comments.map((c) => (
        <div key={c.id}>
          <div className="mono dim">
            {c.path}:{c.startLine ? `${c.startLine}–${c.line}` : c.line}
            {c.side === "LEFT" ? " (removed lines)" : ""}
          </div>
          <pre>{c.body}</pre>
        </div>
      ))}
      {!p.body && !p.comments.length && <div className="dim">No words: just the verdict.</div>}
      {!!p.loose && (
        <div className={`dim ${css.loose}`}>
          <Icon name="info" size={12} /> {p.loose === 1 ? "One comment is" : `${p.loose} comments are`} on lines outside the pull request's diff on GitHub, so {p.loose === 1 ? "it goes" : "they go"} in the review's body instead.
        </div>
      )}
    </div>
  );
}
