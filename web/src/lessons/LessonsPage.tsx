import { useEffect, useState } from "react";
import { Link, useLocation, useNavigate } from "react-router";
import { getLessons, type Lesson, type LessonsInfo, type LessonStateName, makeLessonMission, setLessonState, useEvents } from "../api";
import { useLoaded } from "../load";
import { Button, buttonClass } from "../ui/Button";
import { Empty } from "../ui/Empty";
import { ErrorCard } from "../ui/ErrorCard";
import { GroupHead } from "../ui/GroupHead";
import { Icon } from "../ui/Icon";
import { PageHead } from "../ui/PageHead";
import { Rows } from "../ui/Rows";
import { Skeleton } from "../ui/Skeleton";
import { toast } from "../ui/toast";
import { useTitle } from "../ui/useTitle";
import { LessonCard } from "./LessonCard";
import { LessonsFoot, LessonsFrame, LessonsLede, LessonsSection } from "./LessonsFrame";
import { lessonAnchor, lessonGroups, lessonOfAnchor, readingWords } from "./lessonWords";

// The Lessons page: what crew keep running into where they work, read
// from their handoffs (internal/lessons), each with what would stop it.
// Recommendations (two crew or more) first, then what one crew ran into,
// then what nobody has lately; fixed and dismissed ones folded at the
// foot. A lesson is made a mission of (waiting to be briefed: its page
// opens), marked fixed, dismissed, or opened again. A notification opens
// /lessons#lesson-3, which is scrolled to and flashed. Read again as the
// server says they've moved (the lessons topic).
export function LessonsPage() {
  useTitle("Lessons");
  const { value: info, error, reload, set } = useLoaded<LessonsInfo>(getLessons, [], { keep: true });
  useEvents("lessons", { hello: reload, changed: reload });
  const navigate = useNavigate();
  const { hash } = useLocation();
  const [busy, setBusy] = useState(0);
  const [settledOpen, setSettledOpen] = useState(false);
  const wanted = lessonOfAnchor(hash);

  // The lesson a notification named: shown, folded or not, and scrolled to.
  const wantedSettled = info?.lessons.some((l) => l.id === wanted && l.state !== "open");
  useEffect(() => {
    if (wanted === undefined || !info) return;
    if (wantedSettled) setSettledOpen(true);
    requestAnimationFrame(() => document.getElementById(lessonAnchor(wanted))?.scrollIntoView({ block: "start" }));
  }, [wanted, info !== undefined, wantedSettled]); // eslint-disable-line react-hooks/exhaustive-deps

  const mark = (l: Lesson, state: LessonStateName, said: string) => {
    setBusy(l.id);
    set((was) => was && { ...was, lessons: was.lessons.map((x) => (x.id === l.id ? { ...x, state, again: state === "open" ? x.again : undefined } : x)) });
    setLessonState(l.id, state)
      .then(() => toast(said, { action: { label: "Undo", run: () => void setLessonState(l.id, l.state as LessonStateName).then(reload) } }))
      .catch((e: Error) => {
        toast(`Could not update the lesson: ${e.message}`, { tone: "danger" });
        void reload();
      })
      .finally(() => setBusy(0));
  };
  const startMission = (l: Lesson) => {
    setBusy(l.id);
    makeLessonMission(l.id)
      .then((made) => navigate(`/missions/${encodeURIComponent(made.mission)}`))
      .catch((e: Error) => toast(`Could not start a mission: ${e.message}`, { tone: "danger" }))
      .finally(() => setBusy(0));
  };
  const actionsOf = (l: Lesson) => {
    const off = busy === l.id;
    if (l.state !== "open") {
      return (
        <Button size="small" variant="ghost" disabled={off} onClick={() => mark(l, "open", "Reopened")}>
          Reopen
        </Button>
      );
    }
    return (
      <>
        {l.mission ? (
          <Link to={`/missions/${encodeURIComponent(l.mission)}`} className={buttonClass({ size: "small" })}>
            Open mission
          </Link>
        ) : (
          <Button size="small" disabled={off} onClick={() => startMission(l)}>
            Start mission
          </Button>
        )}
        <Button size="small" variant="ghost" disabled={off} onClick={() => mark(l, "fixed", "Marked fixed")}>
          Mark fixed
        </Button>
        <Button size="small" variant="ghost" disabled={off} onClick={() => mark(l, "dismissed", "Dismissed")}>
          Dismiss
        </Button>
      </>
    );
  };

  const groups = info ? lessonGroups(info.lessons, Date.now()) : [];
  return (
    <LessonsFrame>
      <PageHead title="Lessons" />
      <LessonsLede>What crew keep running into where they work, from their handoffs, and what would stop it.</LessonsLede>
      {error && !info && <ErrorCard error={error} retry={reload} />}
      {!info && !error && <Skeleton rows={3} />}
      {info && groups.length === 0 && (
        <Empty icon="tool" title="No lessons yet">
          Each crew notes what got in its way under Environment in its handoff. When two crew run into the same thing, it
          shows here with what would fix it.
        </Empty>
      )}
      {groups.map((g) => {
        const folded = g.id === "settled" && !settledOpen;
        return (
          <LessonsSection key={g.id}>
            <GroupHead count={g.lessons.length}>
              {g.id === "settled" ? (
                <Button variant="link" onClick={() => setSettledOpen(!settledOpen)} aria-expanded={!folded}>
                  <Icon name={folded ? "more" : "fold"} size={12} /> {g.title}
                </Button>
              ) : (
                g.title
              )}
            </GroupHead>
            {!folded && (
              <Rows>
                {g.lessons.map((l) => (
                  <LessonCard key={l.id} lesson={l} actions={actionsOf(l)} flash={l.id === wanted} sightingsOpen={l.id === wanted} />
                ))}
              </Rows>
            )}
          </LessonsSection>
        );
      })}
      {info && (
        <LessonsFoot>
          {readingWords(info)}
          {!info.on && (
            <>
              {" · "}
              <Link to="/settings/agents">Settings</Link>
            </>
          )}
        </LessonsFoot>
      )}
    </LessonsFrame>
  );
}
