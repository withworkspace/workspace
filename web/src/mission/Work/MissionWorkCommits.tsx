import { Copyable } from "../../ui/Copyable";
import { Time } from "../../ui/Time";
import css from "./MissionWorkCommits.module.css";
import { Text, textClass } from "../../ui/Text";

// MissionWorkCommits is a capsule's commits, newest first, a line each:
// the commit to copy, what it says, and when. more is how many there are
// besides those shown.
export function MissionWorkCommits({ commits, more }: { commits: { sha: string; subject: string; time: string }[]; more?: number }) {
  return (
    <ul className={css.commits}>
      {commits.map((m) => (
        <li key={m.sha}>
          <Copyable text={m.sha.slice(0, 7)} className={css.sha} title="Copy commit" />
          <span className={css.subject} title={m.subject}>
            {m.subject}
          </span>
          <Time iso={m.time} as="relative" className={textClass("dim")} />
        </li>
      ))}
      {!!more && more > 0 && <Text as="li" dim className={css.more}>and {more} more</Text>}
    </ul>
  );
}
