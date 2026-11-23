import { useEffect, useMemo, useState } from "react";
import { decide, type Decision, type MissionDetail, withdrawDecision } from "../api";
import type { Reload } from "../load";
import { Button } from "../ui/Button";
import { Icon } from "../ui/Icon";
import { Empty } from "../ui/Empty";
import { humanError } from "../ui/text";
import { toast } from "../ui/toast";
import css from "./DecisionsTab.module.css";
import { groupDecisions, stepOf } from "./decisions";
import { MissionDecision } from "./MissionDecision";
import { type DecisionFormKind, MissionDecisionForm } from "./MissionDecisionForm";
import { MissionDecisionGroup } from "./MissionDecisionGroup";
import { CrewBadge } from "./parts";

// A mission's Decisions tab: everything decided on it, and the place a
// decision's name leads to (?tab=decisions&d=d4, which marks it). Yours
// first, since they stand over what the plan asks; then what crew and the
// orchestrator settled as they went, by the step each was made at; then,
// folded, those replaced or withdrawn. It's also where one's written: a
// decision of your own, one in place of another, or one withdrawn, each
// kept in the record as a decision is never edited.
export function DecisionsTab({ d, marked, mark, reload }: { d: MissionDetail; marked?: string; mark: (id?: string) => void; reload: Reload }) {
  const g = useMemo(() => groupDecisions(d.decisions, d.stages), [d.decisions, d.stages]);
  // The form that's open: a new one's, or one on a decision.
  const [open, setOpen] = useState<{ kind: DecisionFormKind; of?: string }>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  const [past, setPast] = useState(false);
  const markedGone = !!marked && g.gone.some((x) => x.id === marked);

  // The one the address names is brought into view, once it's drawn.
  useEffect(() => {
    if (!marked) return;
    document.querySelector(`[data-decision="${CSS.escape(marked)}"]`)?.scrollIntoView({ block: "center" });
  }, [marked, markedGone]);

  const start = (kind: DecisionFormKind, of?: string) => {
    setError(undefined);
    setOpen({ kind, of });
  };
  const close = () => setOpen(undefined);
  const send = (x: { what: string; why: string }) => {
    if (!open || busy) return;
    setBusy(true);
    setError(undefined);
    const done =
      open.kind === "withdraw"
        ? withdrawDecision(d.slug, open.of!, x.why).then((r) => ({ message: r.message, id: undefined as string | undefined }))
        : decide(d.slug, { what: x.what, why: x.why, supersedes: open.kind === "replace" ? open.of : undefined });
    done
      .then(async (r) => {
        toast(r.message, { tone: "done" });
        await reload();
        close();
        mark(r.id);
      })
      .catch((e: Error) => setError(humanError(e.message)))
      .finally(() => setBusy(false));
  };

  const form = (x?: Decision) =>
    open &&
    open.of === x?.id && <MissionDecisionForm key={`${open.kind}:${open.of ?? ""}`} kind={open.kind} of={open.of} what={open.kind === "replace" ? x?.what : undefined} busy={busy} error={error} onSend={send} onCancel={close} />;
  const row = (x: Decision, yours?: boolean) => (
    <MissionDecision
      key={x.id}
      d={x}
      yours={yours}
      step={yours ? stepOf(x, d.stages) : undefined}
      who={d.crew.some((c) => c.id === x.by) ? <CrewBadge id={x.by} /> : undefined}
      marked={marked === x.id}
      actions={
        x.state === "current" && (
          <>
            <Button variant="ghost" size="small" onClick={() => start("replace", x.id)} title={`Record another in place of ${x.id}`}>
              Replace
            </Button>
            <Button variant="ghost" size="small" onClick={() => start("withdraw", x.id)} title={`Take ${x.id} back, saying why`}>
              Withdraw
            </Button>
          </>
        )
      }
    >
      {form(x)}
    </MissionDecision>
  );
  const record = (
    <Button variant="ghost" size="small" onClick={() => start("record")}>
      <Icon name="plus" size={12} /> Record a decision
    </Button>
  );

  if (!d.decisions.length && !open) {
    return (
      <Empty icon="gavel" title="Nothing's decided yet">
        What you answer when you're asked is kept here as a decision of yours, beside what crew and the orchestrator settle as they go.
        <div className={css.emptyAct}>{record}</div>
      </Empty>
    );
  }
  return (
    <div className={css.tab}>
      <MissionDecisionGroup title="Yours" count={g.yours.length} note="They stand over what the plan asks." end={!(open?.kind === "record") && record}>
        {form()}
        {g.yours.map((x) => row(x, true))}
        {!g.yours.length && open?.kind !== "record" && <p className={css.none}>None yet. What you answer when you're asked is kept here, and you can record one of your own.</p>}
      </MissionDecisionGroup>
      {g.work.length > 0 && (
        <MissionDecisionGroup title="From the work" count={g.worked} note="What crew and the orchestrator settled as they went.">
          {g.work.map((s) => (
            <MissionDecisionGroup
              key={s.n}
              step
              title={
                s.n ? (
                  <>
                    Step {s.n}
                    {s.step && <span className={css.stepTitle}>{s.step.title}</span>}
                  </>
                ) : (
                  "At no step"
                )
              }
            >
              {s.list.map((x) => row(x))}
            </MissionDecisionGroup>
          ))}
        </MissionDecisionGroup>
      )}
      {g.gone.length > 0 && (
        <details className={css.gone} open={past || markedGone} onToggle={(e) => setPast(e.currentTarget.open)}>
          <summary>
            {g.gone.length} replaced or withdrawn
          </summary>
          <div className={css.goneList}>{g.gone.map((x) => row(x, x.by === "you"))}</div>
        </details>
      )}
    </div>
  );
}
