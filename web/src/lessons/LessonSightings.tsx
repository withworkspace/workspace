import { useEffect, useState } from "react";
import { Link } from "react-router";
import type { LessonSighting } from "../api";
import { Button } from "../ui/Button";
import { Icon } from "../ui/Icon";
import { Inline } from "../ui/Inline";
import { Text } from "../ui/Text";
import { Time } from "../ui/Time";
import css from "./LessonSightings.module.css";

// LessonSightings is each crew that ran into a lesson, in their words, the
// latest first: folded under a count until it's opened, or asked open.
export function LessonSightings({ seen, open: openAtFirst = false }: { seen: LessonSighting[]; open?: boolean }) {
  const [open, setOpen] = useState(openAtFirst);
  // Asked open again (a notification for it, opened while on the page): open.
  useEffect(() => {
    if (openAtFirst) setOpen(true);
  }, [openAtFirst]);
  if (seen.length === 0) return null;
  return (
    <div className={css.sightings}>
      <Button variant="link" size="small" onClick={() => setOpen(!open)} aria-expanded={open} className={css.toggle}>
        <Icon name={open ? "fold" : "more"} size={12} /> What crew said ({seen.length})
      </Button>
      {open && (
        <ul className={css.list}>
          {seen.map((s) => (
            <li key={`${s.mission}/${s.crew}`} className={css.item}>
              <div className={css.where}>
                <Link to={`/missions/${encodeURIComponent(s.mission)}`} className={css.mission}>
                  {s.mission}
                </Link>
                <Text dim>
                  {s.crew} · <Time iso={s.at} as="relative" />
                </Text>
              </div>
              <Inline text={s.said} className={css.said} />
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
