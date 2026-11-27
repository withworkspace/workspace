import { teamKeys } from "./keys";
import type { TeamTopic, TeamView } from "../api";
import { Button } from "../ui/Button";
import { GroupHead } from "../ui/GroupHead";
import { Icon } from "../ui/Icon";
import { Inline } from "../ui/Inline";
import { Spinner } from "../ui/Spinner";
import { humanError } from "../ui/strings";
import { Time } from "../ui/Time";
import { cx } from "../ui/cx";
import css from "./TeamSummary.module.css";

export type TeamSummaryData = NonNullable<TeamView["summary"]>;

type TeamSummaryProps = {
  summary: TeamSummaryData;
  // folded leaves only its heading; onFold folds or unfolds it.
  folded?: boolean;
  onFold: () => void;
  // on is the name of the topic whose changes alone are showing, if any;
  // onTopic is a topic's name pressed, to show only its changes or all.
  on?: string;
  onTopic: (t: TeamTopic) => void;
  // names is what the people with these keys are called.
  names: (keys: string[]) => string[];
  // onWrite has it written again.
  onWrite: () => void;
};

// TeamSummary is the Team page's account of a period: what the team's
// been working on, as a few topics an agent wrote, each with who's on it
// and how many changes are part of it. A topic's name leaves only its
// changes showing.
export function TeamSummary({ summary, folded, onFold, on, onTopic, names, onWrite }: TeamSummaryProps) {
  return (
    <section className={css.summary} aria-label="Team summary">
      <GroupHead className={css.bar}>
        <Button variant="bare" className={css.fold} aria-expanded={!folded} onClick={onFold}>
          <Icon name={folded ? "chevron-right" : "chevron-down"} size={12} /> Being worked on
        </Button>
        <span className={css.when}>
          {summary.writing ? (
            <>
              <Spinner size={12} /> Writing…
            </>
          ) : (
            <>
              {summary.at && (
                <span>
                  Written <Time iso={summary.at} as="relative" />
                </span>
              )}
              {summary.at && summary.stale && <span className={css.stale}>changed since</span>}
              <Button variant="link" className={css.again} onClick={onWrite} title="Rewrite from current changes" keys={teamKeys.catchUp}>
                Rewrite
              </Button>
            </>
          )}
        </span>
      </GroupHead>
      {!folded && summary.failed && <p className={css.failed}>Could not write summary: {humanError(summary.failed)}</p>}
      {!folded && !!summary.topics?.length && (
        <ul className={css.topics}>
          {summary.topics.map((t) => {
            const pressed = on === t.name;
            return (
              <li key={t.name} className={cx(css.topic, pressed && css.on)}>
                <div className={css.head}>
                  <Button variant="bare" className={css.name} aria-pressed={pressed} onClick={() => onTopic(t)} title={pressed ? "Show every change" : "Only this topic's changes"} keys={pressed ? teamKeys.allChanges : undefined}>
                    {t.name}
                  </Button>
                  <span className={css.meta}>
                    {t.people.length > 0 && <span>{names(t.people).join(", ")}</span>}
                    <span>
                      {t.strands.length} {t.strands.length === 1 ? "change" : "changes"}
                    </span>
                  </span>
                </div>
                <Inline text={t.text} className={css.text} />
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}
