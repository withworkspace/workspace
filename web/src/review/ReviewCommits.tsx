import css from "./ReviewCommits.module.css";
import { Text, textClass } from "../ui/Text";

// ReviewCommits are the commits a review's diff is of, folded: each its
// short hash, subject and author.
export function ReviewCommits({ commits }: { commits: { sha: string; subject: string; author: string }[] }) {
  return (
    <details className={css.commits}>
      <summary className={textClass("dim")}>Commits</summary>
      <ul>
        {commits.map((c) => (
          <li key={c.sha}>
            <Text mono dim>{c.sha.slice(0, 7)}</Text> {c.subject} <Text dim>· {c.author}</Text>
          </li>
        ))}
      </ul>
    </details>
  );
}
