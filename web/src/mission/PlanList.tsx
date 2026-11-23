import { Fragment, useContext, useMemo, useRef, useState } from "react";
import type { MissionDetail, Stage } from "../api";
import { FloatingCard } from "../ui/FloatingCard";
import { MissionCrewBadge } from "./MissionCrewBadge";
import { MissionPlan } from "./MissionPlan";
import { MissionPlanCheckList } from "./MissionPlanCheckList";
import { MissionPlanFold } from "./MissionPlanFold";
import { MissionPlanLine } from "./MissionPlanLine";
import { MissionPlanNote } from "./MissionPlanNote";
import { MissionPlanNow } from "./MissionPlanNow";
import { MissionPlanPip } from "./MissionPlanPip";
import { MissionPlanStep } from "./MissionPlanStep";
import { MissionStepCard, missionStepCardClass } from "./MissionStepCard";
import { useHoverDelay } from "../ui/useHoverDelay";
import { RefContext } from "../refs/context";
import { Icon } from "../ui/Icon";
import { Inline } from "../ui/inline";
import { crewStatus } from "../ui/status";
import { duration, stamp } from "../ui/time";
import { CrewBadge, MissionData, StepChecks, StepLimit, StepRuns } from "./parts";
import { checksUnmet, foldNote, passedUnmet, planWindow, stepBundle, stepKind, stepText, stepTimes, stepYours, type StepTime, waitHow, type WaitHow, watchWords, worthOpening } from "./planRows";
import { Chip } from "../ui/Chip";
import { capital } from "../ui/text";

// PlanList is the mission's plan (or its brief's stages) in its details: a
// row a step, with how it stands, its number and name, and its kind at
// the row's end (in the accent when it's yours). The step it's on is opened
// where that says more than the row does. A long plan shows the steps
// round that one, and folds the rest into a row where they are, each a
// pip; the row opens them. What there isn't room for in a row is its card,
// beside the list.
//
// brief: in a brief's outline, where a row is pressed to go to the step in
// the plan, and the step it's on isn't said in place.
export function PlanList({ stages, brief }: { stages: Stage[]; brief?: boolean }) {
  const d = useContext(MissionData);
  const [open, setOpen] = useState<{ earlier?: boolean; later?: boolean }>({});
  const { earlier, shown, later } = useMemo(() => planWindow(stages), [stages]);
  const times = useMemo(() => stepTimes(d?.events ?? []), [d?.events]);

  // One card for the list: of the step, or the pip, the pointer's on.
  const list = useRef<HTMLOListElement>(null);
  const hover = useHoverDelay<number>();
  const els = useRef(new Map<number, HTMLElement>());
  const card = (s: Stage) => ({
    ref: (el: HTMLElement | null) => {
      if (el) els.current.set(s.n, el);
      else els.current.delete(s.n);
    },
    // The opened step's own block has its crew's cards: not this one too.
    onMouseOver: (e: React.MouseEvent) => ((e.target as Element).closest("[data-plan-now]") ? hover.out() : hover.over(s.n)),
    onMouseLeave: hover.out,
    onFocus: () => hover.over(s.n),
    onBlur: hover.out,
  });
  const carded = hover.shown === undefined ? undefined : stages.find((s) => s.n === hover.shown);
  const anchor = hover.shown === undefined ? undefined : els.current.get(hover.shown);

  const rows = (run: Stage[]) => run.map((s) => <StepRow key={s.n} s={s} d={d} card={card(s)} brief={brief} />);
  const fold = (run: Stage[], side: "earlier" | "later") => {
    if (!run.length) return null;
    const toggle = () => {
      hover.clear();
      setOpen((o) => ({ ...o, [side]: !o[side] }));
    };
    const row = (
      <MissionPlanFold key={`fold-${side}`} side={side} count={run.length} open={open[side]} onToggle={toggle} note={foldNote(run, side)}>
        {run.map((s) => (
          <MissionPlanPip key={s.n} state={s.state} warn={passedUnmet(s)} yours={s.state === "todo" && stepYours(s)} wait={s.type === "Wait"} {...card(s)} />
        ))}
      </MissionPlanFold>
    );
    if (!open[side]) return row;
    return side === "earlier" ? [row, ...rows(run)] : [...rows(run), row];
  };

  return (
    <>
      <MissionPlan rows ref={list}>
        {fold(earlier, "earlier")}
        {rows(shown)}
        {fold(later, "later")}
      </MissionPlan>
      {carded && anchor && (
        <FloatingCard anchor={anchor} beside={list.current} className={missionStepCardClass()} onEnter={hover.stay} onLeave={hover.out}>
          {/* Inside a card, refs are words: no cards in cards. */}
          <RefContext.Provider value={undefined}>
            <StepCard s={carded} of={stages.length} d={d} time={times.get(carded.n)} />
          </RefContext.Provider>
        </FloatingCard>
      )}
    </>
  );
}

type CardProps = {
  ref: (el: HTMLElement | null) => void;
  onMouseOver: (e: React.MouseEvent) => void;
  onMouseLeave: () => void;
  onFocus: () => void;
  onBlur: () => void;
};

// StepRow is a step: nothing but its name and kind, unless it needs you
// to look (the current step's crew and whether it's yours, checks not met,
// time run out), or it's the one the plan is on and worth opening.
function StepRow({ s, d, card, brief }: { s: Stage; d?: MissionDetail; card: CardProps; brief?: boolean }) {
  const kind = stepKind(s);
  const how = waitHow(s);
  const current = s.state === "current";
  const opened = current && worthOpening(s);
  const yours = current && (stepYours(s) || s.held);
  const late = !!s.outOfTime && s.state !== "accepted";
  const unmet = checksUnmet(s);
  return (
    <MissionPlanStep
      row
      n={s.n}
      title={s.title}
      state={s.state}
      yours={stepYours(s) && s.state !== "accepted"}
      skipped={s.skipped}
      opened={opened}
      pressable={brief}
      kind={kind && { icon: kind[0], says: how && s.state !== "accepted" ? `${kind[1]} · ${how.word.toLowerCase()}: ${how.says}` : kind[1] }}
      under={opened && !brief && <StepNow s={s} d={d} />}
      tabIndex={0}
      {...card}
    >
      {!opened && (yours || late || unmet || (current && !!s.crew?.length)) && (
        <>
          {yours && (
            <MissionPlanNote icon="user" yours title="Yours: only you pass it">
              yours
            </MissionPlanNote>
          )}
          {late && <StepLimit s={s} />}
          {unmet && <StepChecks s={s} />}
          {current && s.crew?.map((c) => <CrewBadge key={c} id={c} />)}
        </>
      )}
    </MissionPlanStep>
  );
}

// StepNow is the step the plan is on, said in place: its kind and where,
// what it makes, who's on it and what they're doing, its clock, and each
// check with how it stands.
function StepNow({ s, d }: { s: Stage; d?: MissionDetail }) {
  const kind = stepKind(s);
  const checks = s.checks ?? [];
  const how = waitHow(s);
  return (
    <MissionPlanNow>
      {kind && (
        <MissionPlanLine icon={kind[0]}>
          <strong>{kind[1]}</strong>
          {how ? s.on && s.on !== "you" && <Inline text={`on ${s.on}`} /> : s.whoSays && <Inline text={s.whoSays} />}
        </MissionPlanLine>
      )}
      {how && <WaitLine how={how} />}
      {s.makes && (
        <MissionPlanLine words>
          Makes <Inline text={s.makes} />
        </MissionPlanLine>
      )}
      {!!s.crew?.length && (
        <MissionPlanLine>
          {s.crew.map((id) => {
            const c = d?.crew.find((x) => x.id === id);
            return (
              <Fragment key={id}>
                <CrewBadge id={id} />
                {c && <span>{crewStatus(c).words.toLowerCase()}</span>}
              </Fragment>
            );
          })}
        </MissionPlanLine>
      )}
      {(stepYours(s) || s.held) && (
        <MissionPlanLine icon="user" yours>
          {how ? "Yours to do" : "Yours: only you pass it"}
        </MissionPlanLine>
      )}
      {!!s.carryOn && <MissionPlanLine icon="reply">Carries on from step {s.carryOn}</MissionPlanLine>}
      {!!s.attempts && <MissionPlanLine icon="rotate">Attempt {s.attempts + 1}</MissionPlanLine>}
      {s.limit && (
        <MissionPlanLine>
          <StepLimit s={s} />
        </MissionPlanLine>
      )}
      {s.onlyIf && (
        <MissionPlanLine icon="question">
          <span>
            Only if <Inline text={s.onlyIf} />
          </span>
        </MissionPlanLine>
      )}
      {s.inEach && (
        <MissionPlanLine>
          <StepRuns s={s} />
        </MissionPlanLine>
      )}
      {checks.length > 0 && (
        <>
          <MissionPlanLine icon={checksUnmet(s) ? "alert" : undefined} warn={checksUnmet(s)}>
            {checksWords(s)}
          </MissionPlanLine>
          <MissionPlanCheckList checks={checks} />
        </>
      )}
    </MissionPlanNow>
  );
}

const upperFirst = (t: string) => (t.startsWith("ws ") ? t : capital(t));

// WaitLine says how a Wait is passed: watched by ws, or not at all.
function WaitLine({ how }: { how: WaitHow }) {
  return (
    <MissionPlanLine>
      <Chip tone={how.auto ? "progress" : "attention"}>{how.word}</Chip>
      <span>{how.says}</span>
    </MissionPlanLine>
  );
}

const checksWords = (s: Stage) => {
  const checks = s.checks ?? [];
  // A Wait's checks aren't run in rounds: ws looks, every so often.
  if (s.type === "Wait") return s.state === "accepted" ? (checks.length === 1 ? "Its check, as ws last saw it" : "Its checks, as ws last saw them") : upperFirst(watchWords(s, (iso) => stamp(iso)));
  if (!s.round) return `${checks.length} ${checks.length === 1 ? "check" : "checks"}, not run yet`;
  return `Checks, round ${s.round}: ${checks.filter((c) => c.result === "met").length} of ${checks.length} met`;
};

// StepCard is a step in full: what there isn't room for in its row. Its
// text, who's on it and what it makes; once it's passed, why, how long it
// took and the bundle that showed it; and its checks.
function StepCard({ s, of, d, time }: { s: Stage; of: number; d?: MissionDetail; time?: StepTime }) {
  const kind = stepKind(s);
  const bundle = d ? stepBundle(d.bundles, s.n) : undefined;
  const checks = s.checks ?? [];
  const noun = d?.plan ? "step" : "stage";
  const where =
    s.skipped ? (
      "Skipped"
    ) : s.state === "accepted" ? (
      <>
        <Icon name="check" size={12} /> Done{time?.to ? ` ${stamp(time.to)}` : ""}
      </>
    ) : s.state === "current" ? (
      `Now${time?.from ? `, since ${stamp(time.from)}` : ""}`
    ) : stepYours(s) ? (
      "To come, yours"
    ) : (
      "To come"
    );
  const rows: [string, React.ReactNode][] = [];
  const row = (label: string, value: React.ReactNode) => value && rows.push([label, value]);
  const how = waitHow(s);
  row("Waits on", how && (s.on === "you" ? "you: it's yours to do" : s.on ? <Inline text={s.on} /> : "something outside ws"));
  row(
    "Passed by",
    how && s.state !== "accepted" && (
      <>
        <Chip tone={how.auto ? "progress" : "attention"}>{how.word}</Chip> {how.says}
      </>
    ),
  );
  row(
    "Who",
    !how && (s.whoSays || s.who || !!s.crew?.length) && (
      <>
        {(s.whoSays || s.who) && <Inline text={s.whoSays || s.who || ""} />}{" "}
        {s.crew?.map((id) => (
          <MissionCrewBadge key={id} id={id} />
        ))}
      </>
    ),
  );
  row("Makes", s.makes && <Inline text={s.makes} />);
  row("Opens", s.opensPR && "a pull request, through ws");
  row("Carries on", !!s.carryOn && `from step ${s.carryOn}, with its crew`);
  row("Limit", s.limit && `${s.limit}${s.outOfTime ? ", out of time" : ""}`);
  row("Only if", s.onlyIf && <Inline text={s.onlyIf} />);
  row("After it", s.checkIn && s.state !== "accepted" && "the orchestrator stops for you");
  row(s.skipped ? "Skipped" : "Passed", s.state === "accepted" && s.why && <Inline text={s.why} />);
  row(
    "Took",
    time?.from && time.to && (
      <>
        {duration(time.from, time.to)}{" "}
        <span className="dim">
          ({stamp(time.from)} to {stamp(time.to)})
        </span>
      </>
    ),
  );
  row("Shown by", bundle && `${bundle.id}, ${bundle.verdict ?? "waiting for you"}`);
  return (
    <MissionStepCard
      kind={kind && { icon: kind[0], says: kind[1] }}
      which={`${noun} ${s.n} of ${of}`}
      state={s.skipped ? "todo" : s.state}
      stands={where}
      title={s.title}
      text={<Inline text={stepText(s)} lines={4} />}
      rows={rows}
      checksSay={checksWords(s)}
      checks={checks.length > 0 && <MissionPlanCheckList checks={checks} said />}
    />
  );
}
