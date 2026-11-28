import type { ReactNode } from "react";
import { Link } from "react-router";
import type { Lesson } from "../api";
import { Chip } from "../ui/Chip";
import { cx } from "../ui/cx";
import { Inline } from "../ui/Inline";
import { Row } from "../ui/Rows";
import { Text } from "../ui/Text";
import { Time } from "../ui/Time";
import { LessonSightings } from "./LessonSightings";
import { fixOf, fixTone, lessonAnchor, seenWords } from "./lessonWords";
import css from "./LessonCard.module.css";

// LessonCard is one lesson on the Lessons page: what crew run into, what
// would stop it and who changes it, how many crew ran into it and when
// last, why, each crew's words, and what you can do with it (actions,
// the page's). One fixed or dismissed is quieter; one crew ran into
// after it was fixed says so.
export function LessonCard({ lesson: l, actions, flash, sightingsOpen }: { lesson: Lesson; actions?: ReactNode; flash?: boolean; sightingsOpen?: boolean }) {
  const settled = l.state !== "open";
  return (
    <Row id={lessonAnchor(l.id)} flash={flash} className={cx(css.card, settled && css.settled)}>
      <div className={css.main}>
        <div className={css.head}>
          <h3 className={css.title}>{l.title}</h3>
          <Chip tone={fixTone(l.fix)}>{fixOf(l.fix)}</Chip>
          {l.state === "fixed" && <Chip tone="done">Fixed</Chip>}
          {l.state === "dismissed" && <Chip tone="quiet">Dismissed</Chip>}
          {l.state === "open" && l.again && <Chip tone="attention">Back after fix</Chip>}
        </div>
        <Text as="div" dim className={css.facts}>
          {l.repo && <span className={css.repo}>{l.repo}</span>}
          <span>{seenWords(l)}</span>
          <span>
            Last seen <Time iso={l.lastSeen} as="relative" />
          </span>
          {l.mission && (
            <Link to={`/missions/${encodeURIComponent(l.mission)}`} className={css.mission}>
              Mission
            </Link>
          )}
        </Text>
        {l.change && <Inline text={l.change} className={css.change} />}
        {l.why && <Inline text={l.why} className={css.why} />}
        <LessonSightings seen={l.seen} open={sightingsOpen} />
      </div>
      {actions && <div className={css.actions}>{actions}</div>}
    </Row>
  );
}
