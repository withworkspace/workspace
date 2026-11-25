import type { MissionDetail, Stage } from "../api";
import { Chip } from "../ui/Chip";
import { Icon } from "../ui/Icon";
import { Inline } from "../ui/Inline";
import { duration, stamp } from "../ui/when";
import { MissionCrewBadge } from "./MissionCrewBadge";
import { MissionPlanCheckList } from "./MissionPlanCheckList";
import { MissionStepCard } from "./MissionStepCard";
import { checksWords, stepBundle, stepKind, stepText, stepYours, type StepTime, waitHow } from "./planRows";
import { Text } from "../ui/Text";

// PlanStepCard is a step in full: what there isn't room for in its row. Its
// text, who's on it and what it makes; once it's passed, why, how long it
// took and the bundle that showed it; and its checks.
export function PlanStepCard({ s, of, d, time }: { s: Stage; of: number; d?: MissionDetail; time?: StepTime }) {
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
        <Text dim>
          ({stamp(time.from)} to {stamp(time.to)})
        </Text>
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
      checksSay={checksWords(s, stamp)}
      checks={checks.length > 0 && <MissionPlanCheckList checks={checks} said />}
    />
  );
}
