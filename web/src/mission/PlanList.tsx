import { Fragment, useContext, useMemo, useRef, useState } from "react";
import type { MissionDetail, Stage, StageCheck } from "../api";
import { FloatingCard } from "../ui/FloatingCard";
import { MissionPlanNote } from "./MissionPlanNote";
import { useHoverDelay } from "../ui/useHoverDelay";
import { RefContext } from "../refs/context";
import { Icon, type IconName } from "../ui/Icon";
import { Inline } from "../ui/inline";
import { crewStatus } from "../ui/status";
import { duration, stamp } from "../ui/time";
import { CrewBadge, crewHue, MissionData, StepChecks, StepLimit, StepRuns } from "./parts";
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
export function PlanList({ stages }: { stages: Stage[] }) {
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
    onMouseOver: (e: React.MouseEvent) => ((e.target as Element).closest(".plan-now") ? hover.out() : hover.over(s.n)),
    onMouseLeave: hover.out,
    onFocus: () => hover.over(s.n),
    onBlur: hover.out,
  });
  const carded = hover.shown === undefined ? undefined : stages.find((s) => s.n === hover.shown);
  const anchor = hover.shown === undefined ? undefined : els.current.get(hover.shown);

  const rows = (run: Stage[]) => run.map((s) => <StepRow key={s.n} s={s} d={d} card={card(s)} />);
  const fold = (run: Stage[], side: "earlier" | "later") => {
    if (!run.length) return null;
    const toggle = () => {
      hover.clear();
      setOpen((o) => ({ ...o, [side]: !o[side] }));
    };
    if (open[side]) {
      const less = (
        <li key={`less-${side}`}>
          <button className={`plan-fold fold-${side}${side === "later" ? " fold-end" : ""}`} onClick={toggle} aria-expanded>
            <span className="plan-fold-words">
              Fold {run.length} {side}
            </span>
            <Icon name={side === "earlier" ? "chevron-down" : "chevron-up"} size={12} />
          </button>
        </li>
      );
      return side === "earlier" ? [less, ...rows(run)] : [...rows(run), less];
    }
    const note = foldNote(run, side);
    return (
      <li key={`fold-${side}`}>
        <button className={`plan-fold fold-${side}${side === "later" ? " fold-end" : ""}`} onClick={toggle} aria-expanded={false} aria-label={`Show the ${run.length} ${side} steps`}>
          <span className="plan-fold-words">
            {run.length} {side}
            {note && <span className={`plan-note${note.tone ? ` plan-${note.tone}` : ""}`}>{note.text}</span>}
          </span>
          <Icon name={side === "earlier" ? "chevron-up" : "chevron-down"} size={12} />
          <span className="plan-pips">
            {run.map((s) => (
              <i key={s.n} className={`plan-pip ${pipClass(s)}`} {...card(s)} />
            ))}
          </span>
        </button>
      </li>
    );
  };

  return (
    <>
      <ol className="stepper plan-list" ref={list}>
        {fold(earlier, "earlier")}
        {rows(shown)}
        {fold(later, "later")}
      </ol>
      {carded && anchor && (
        <FloatingCard anchor={anchor} beside={list.current} className="plan-card" onEnter={hover.stay} onLeave={hover.out}>
          {/* Inside a card, refs are words: no cards in cards. */}
          <RefContext.Provider value={undefined}>
            <StepCard s={carded} of={stages.length} d={d} time={times.get(carded.n)} />
          </RefContext.Provider>
        </FloatingCard>
      )}
    </>
  );
}

const pipClass = (s: Stage) => `${s.state}${passedUnmet(s) ? " plan-warn" : ""}${s.state === "todo" && stepYours(s) ? " plan-yours" : ""}${s.type === "Wait" ? " plan-wait" : ""}`;

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
function StepRow({ s, d, card }: { s: Stage; d?: MissionDetail; card: CardProps }) {
  const kind = stepKind(s);
  const how = waitHow(s);
  const current = s.state === "current";
  const opened = current && worthOpening(s);
  const yours = current && (stepYours(s) || s.held);
  const late = !!s.outOfTime && s.state !== "accepted";
  const unmet = checksUnmet(s);
  return (
    <li className={`step ${s.state}${stepYours(s) && s.state !== "accepted" ? " plan-yours" : ""}${s.skipped ? " plan-skipped" : ""}${opened ? " plan-opened" : ""}`} tabIndex={0} {...card}>
      <span className="step-mark" aria-hidden>
        {s.skipped ? <Icon name="minus" size={10} /> : s.state === "accepted" ? <Icon name="check" size={10} /> : null}
      </span>
      <span className="step-body">
        <span className="step-title">
          <span className="step-n">{s.n}</span> {s.title}
          <span className="sr-only">{s.skipped ? " (skipped)" : s.state === "accepted" ? " (done)" : current ? " (now)" : ""}</span>
        </span>
        {!opened && (yours || late || unmet || (current && !!s.crew?.length)) && (
          <span className="step-meta">
            {yours && (
              <MissionPlanNote icon="user" yours title="Yours: only you pass it">
                yours
              </MissionPlanNote>
            )}
            {late && <StepLimit s={s} />}
            {unmet && <StepChecks s={s} />}
            {current && s.crew?.map((c) => <CrewBadge key={c} id={c} />)}
          </span>
        )}
        {opened && <StepNow s={s} d={d} />}
      </span>
      {kind && !opened && (
        <span className="step-kind" title={how && s.state !== "accepted" ? `${kind[1]} · ${how.word.toLowerCase()}: ${how.says}` : kind[1]}>
          <Icon name={kind[0]} size={12} />
        </span>
      )}
    </li>
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
    <div className="plan-now">
      {kind && (
        <div className="plan-line">
          <Icon name={kind[0]} size={12} />
          <strong>{kind[1]}</strong>
          {how ? s.on && s.on !== "you" && <Inline text={`on ${s.on}`} /> : s.whoSays && <Inline text={s.whoSays} />}
        </div>
      )}
      {how && <WaitLine how={how} />}
      {s.makes && (
        <div className="plan-makes">
          Makes <Inline text={s.makes} />
        </div>
      )}
      {!!s.crew?.length && (
        <div className="plan-line">
          {s.crew.map((id) => {
            const c = d?.crew.find((x) => x.id === id);
            return (
              <Fragment key={id}>
                <CrewBadge id={id} />
                {c && <span>{crewStatus(c).words.toLowerCase()}</span>}
              </Fragment>
            );
          })}
        </div>
      )}
      {(stepYours(s) || s.held) && (
        <div className="plan-line plan-yours">
          <Icon name="user" size={12} /> {how ? "Yours to do" : "Yours: only you pass it"}
        </div>
      )}
      {!!s.carryOn && (
        <div className="plan-line">
          <Icon name="reply" size={12} /> Carries on from step {s.carryOn}
        </div>
      )}
      {!!s.attempts && (
        <div className="plan-line">
          <Icon name="rotate" size={12} /> Attempt {s.attempts + 1}
        </div>
      )}
      {s.limit && (
        <div className="plan-line">
          <StepLimit s={s} />
        </div>
      )}
      {s.onlyIf && (
        <div className="plan-line">
          <Icon name="question" size={12} />
          <span>
            Only if <Inline text={s.onlyIf} />
          </span>
        </div>
      )}
      {s.inEach && (
        <div className="plan-line">
          <StepRuns s={s} />
        </div>
      )}
      {checks.length > 0 && (
        <>
          <div className={`plan-line${checksUnmet(s) ? " plan-warn" : ""}`}>
            {checksUnmet(s) && <Icon name="alert" size={12} />}
            {checksWords(s)}
          </div>
          <CheckList checks={checks} />
        </>
      )}
    </div>
  );
}

const upperFirst = (t: string) => (t.startsWith("ws ") ? t : capital(t));

// WaitLine says how a Wait is passed: watched by ws, or not at all.
function WaitLine({ how }: { how: WaitHow }) {
  return (
    <div className="plan-line plan-wait-how">
      <Chip tone={how.auto ? "progress" : "attention"}>{how.word}</Chip>
      <span>{how.says}</span>
    </div>
  );
}

const checksWords = (s: Stage) => {
  const checks = s.checks ?? [];
  // A Wait's checks aren't run in rounds: ws looks, every so often.
  if (s.type === "Wait") return s.state === "accepted" ? (checks.length === 1 ? "Its check, as ws last saw it" : "Its checks, as ws last saw them") : upperFirst(watchWords(s, (iso) => stamp(iso)));
  if (!s.round) return `${checks.length} ${checks.length === 1 ? "check" : "checks"}, not run yet`;
  return `Checks, round ${s.round}: ${checks.filter((c) => c.result === "met").length} of ${checks.length} met`;
};

const results: Record<NonNullable<StageCheck["result"]>, [IconName, string]> = {
  met: ["check", "met"],
  unmet: ["x", "unmet"],
  "can't tell": ["question", "cant"],
  yours: ["user", "yours"],
};

// CheckList is a step's checks, each with how it last came out (or that
// it hasn't been run); said, with who said it, where it isn't met.
function CheckList({ checks, said }: { checks: StageCheck[]; said?: boolean }) {
  return (
    <ul className="plan-checks">
      {checks.map((c, i) => {
        const [icon, cls] = c.result ? results[c.result] : (["circle-dashed", "none"] as [IconName, string]);
        return (
          <li key={i} className={`plan-check plan-check-${cls}`}>
            <Icon name={icon} size={12} />
            <span className="plan-check-text">
              <Inline text={c.text} />
              {said && c.result && c.result !== "met" && (
                <span className="dim">
                  {" "}
                  ({c.result}
                  {c.by ? `: ${c.by}` : ""})
                </span>
              )}
            </span>
          </li>
        );
      })}
    </ul>
  );
}

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
          <span key={id} className="crew-badge" style={{ "--hue": crewHue(id) } as React.CSSProperties}>
            {id}
          </span>
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
    <>
      <div className="plan-card-top">
        {kind && <Icon name={kind[0]} size={13} />}
        <span>
          {kind ? `${kind[1]} · ` : ""}
          {noun} {s.n} of {of}
        </span>
        <span className={`plan-card-state ${s.skipped ? "todo" : s.state}`}>{where}</span>
      </div>
      <div>
        <div className="plan-card-title">{s.title}</div>
        <Inline text={stepText(s)} lines={4} className="plan-card-text" />
      </div>
      {rows.length > 0 && (
        <dl>
          {rows.map(([label, value]) => (
            <Fragment key={label}>
              <dt>{label}</dt>
              <dd>{value}</dd>
            </Fragment>
          ))}
        </dl>
      )}
      {checks.length > 0 && (
        <div className="plan-card-checks">
          <div className="dim">{checksWords(s)}</div>
          <CheckList checks={checks} said />
        </div>
      )}
    </>
  );
}
