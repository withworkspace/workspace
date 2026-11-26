import { HoverCard, HoverCardHead, HoverCardStrong } from "../ui/HoverCard";
import { Icon } from "../ui/Icon";
import { Text } from "../ui/Text";
import css from "./ReviewCommitCount.module.css";

// ReviewCommitCount is how many commits a review's diff is of, in its bar;
// pointed at, a card lists them: short hash, subject, author. since: the
// diff is what's changed since your last review, which it says.
export function ReviewCommitCount({ commits, since }: { commits: { sha: string; subject: string; author: string }[]; since?: boolean }) {
  const n = `${commits.length} ${commits.length === 1 ? "commit" : "commits"}`;
  return (
    <HoverCard
      className={css.count}
      trigger={
        <span className={css.trigger}>
          <Icon name="commit" size={13} />
          <span className={css.words}>{n}</span>
        </span>
      }
    >
      <span className={css.card}>
        <HoverCardHead>
          <HoverCardStrong>{n}</HoverCardStrong>
          {since && <Text dim>since your review</Text>}
        </HoverCardHead>
        <ol className={css.list}>
          {commits.map((c) => (
            <li key={c.sha} className={css.commit}>
              <Text mono dim className={css.sha}>
                {c.sha.slice(0, 7)}
              </Text>
              <span className={css.subject}>{c.subject}</span>
              <Text dim className={css.author}>
                {c.author}
              </Text>
            </li>
          ))}
        </ol>
      </span>
    </HoverCard>
  );
}
