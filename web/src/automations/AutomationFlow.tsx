import type { ReactNode } from "react";
import type { AutomationOutline, AutomationOutlineStep, AutomationStepCount } from "../api";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import { countsAt, countWords, failuresOn, stepLook } from "./flowWords";
import css from "./AutomationFlow.module.css";

// AutomationFlow is an automation's script drawn as what it does, top to
// bottom (its outline, internal/automation's OutlineOf): what it runs on,
// then each step, an ifs, a for's and a wait's branches nested under it.
// With counts (steps, failures), each step says how it's gone over its
// runs, or (dry) how it would have over a dry run's. What's filled in as
// it runs is in braces in the words, and drawn as code.
export function AutomationFlow({
  outline,
  steps,
  failures,
  dry,
}: {
  outline: AutomationOutline;
  steps?: AutomationStepCount[];
  failures?: Record<string, number>;
  dry?: boolean;
}) {
  const counts = countsAt(steps);
  const failed = failuresOn(outline.steps, failures);
  return (
    <div className={css.flow}>
      <div className={cx(css.node, css.trigger)}>
        <span className={css.mark}>
          <Icon name="bolt" size={12} />
        </span>
        <div className={css.body}>
          <span className={css.what}>When</span>
          {outline.when.map((w, i) => (
            <div key={i} className={css.when}>
              {i > 0 && <span className={css.or}>or</span>}
              <code>{w.match}</code>
              {w.pace && <span className={css.pace}>{w.pace}</span>}
            </div>
          ))}
        </div>
      </div>
      {outline.default ? (
        <div className={css.node}>
          <span className={css.mark}>
            <Icon name="bell" size={12} />
          </span>
          <div className={css.body}>
            <span className={css.what}>Notify you</span> <span className={css.dim}>of the event, as ws events says it</span>
          </div>
        </div>
      ) : (
        <Steps list={outline.steps} counts={counts} failed={failed} dry={dry} />
      )}
    </div>
  );
}

function Steps({ list, counts, failed, dry }: { list: AutomationOutlineStep[]; counts: Map<string, AutomationStepCount>; failed: Map<string, number>; dry?: boolean }) {
  if (list.length === 0) return <div className={css.none}>nothing</div>;
  return (
    <>
      {list.map((s, i) => (
        <Step key={`${s.at}-${i}`} s={s} counts={counts} failed={failed} dry={dry} />
      ))}
    </>
  );
}

function Step({ s, counts, failed, dry }: { s: AutomationOutlineStep; counts: Map<string, AutomationStepCount>; failed: Map<string, number>; dry?: boolean }) {
  const look = stepLook(s.kind);
  const said = countWords(s.kind, counts.get(s.at), dry);
  const fails = failed.get(s.at);
  const quiet = s.kind === "code" || s.kind === "if" || s.kind === "for";
  return (
    <div className={cx(css.node, quiet && css.quiet, s.kind === "stop" && css.stop)}>
      <span className={css.mark}>
        <Icon name={look.icon} size={12} />
      </span>
      <div className={css.body}>
        <div className={css.line}>
          <span className={css.main}>
            {s.kind === "code" ? (
              <code className={css.code}>{s.text}</code>
            ) : s.kind === "if" ? (
              <>
                <span className={css.what}>If</span> <code className={css.code}>{s.text}</code>
              </>
            ) : s.kind === "for" ? (
              <>
                <span className={css.what}>For each</span> <code className={css.code}>{s.text}</code>
              </>
            ) : (
              <>
                <span className={css.what}>{look.words}</span>
                {s.text && <> {filled(s.text, s.kind === "read" || s.kind === "wait")}</>}
              </>
            )}
            {s.detail && <span className={css.dim}> · {s.kind === "read" ? `as ${s.detail}` : s.detail}</span>}
          </span>
          <span className={css.counts}>
            {said.map((w) => (
              <span key={w} className={css.count}>
                {w}
              </span>
            ))}
            {fails ? <span className={cx(css.count, css.failed)}>{dry ? `would fail ${fails}×` : `failed ${fails}×`}</span> : null}
            <span className={css.at}>line {s.line}</span>
          </span>
        </div>
        {s.branches?.map((b, i) =>
          // An if's first branch and a for's body are said on its line; an
          // else with nothing in it isn't drawn.
          s.kind === "if" && b.label === "else" && b.steps.length === 0 ? null : (
          <div key={i} className={css.branch}>
            {!((s.kind === "if" || s.kind === "for") && i === 0) && <div className={css.label}>{branchLabel(s.kind, b.label)}</div>}
            <div className={css.nested}>
              <Steps list={b.steps} counts={counts} failed={failed} dry={dry} />
            </div>
          </div>
          ),
        )}
      </div>
    </div>
  );
}

// branchLabel is what a branch is called: an if's and a for's as written,
// a wait's in words.
function branchLabel(kind: string, label: string): ReactNode {
  if (kind === "wait") return label === "met" ? "When it comes" : "If it doesn't, in time";
  if (label === "else") return "Otherwise";
  if (label.startsWith("elif ")) {
    return (
      <>
        Or if <code>{label.slice(5)}</code>
      </>
    );
  }
  return <code>{label}</code>;
}

// filled is words with what's filled in as it runs ({e.title}) drawn as
// code; all of it code where it's code (a read's call, a wait's event).
function filled(text: string, code: boolean): ReactNode {
  if (code) return <code className={css.code}>{text}</code>;
  const parts = text.split(/(\{[^}]*\})/);
  return (
    <span className={css.words}>
      “
      {parts.map((p, i) =>
        p.startsWith("{") && p.endsWith("}") ? (
          <code key={i} className={css.fill}>
            {p.slice(1, -1)}
          </code>
        ) : (
          p
        ),
      )}
      ”
    </span>
  );
}
