import css from "./ReviewCommits.module.css";

// ReviewCommits are the commits a review's diff is of, folded: each its
// short hash, subject and author.
export function ReviewCommits({ commits }: { commits: { sha: string; subject: string; author: string }[] }) {
  return (
    <details className={css.commits}>
      <summary className="dim">Commits</summary>
      <ul>
        {commits.map((c) => (
          <li key={c.sha}>
            <span className="mono dim">{c.sha.slice(0, 7)}</span> {c.subject} <span className="dim">· {c.author}</span>
          </li>
        ))}
      </ul>
    </details>
  );
}
