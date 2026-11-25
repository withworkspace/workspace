import { useState } from "react";
import { type DismissWhy, type PassFinding, type ReviewPass, runPRPass, settlePRFinding } from "../api";
import { followWords, reviewDismissWhys } from "../review/findings";
import { ReviewFindingActions } from "../review/ReviewFindingActions";
import { Banner } from "../ui/Banner";
import { Button } from "../ui/Button";
import { Excerpt } from "../ui/Excerpt";
import { clip, humanError } from "../ui/strings";
import { toast } from "../ui/toast";
import { PassCard } from "./PassCard";
import { PassFinding as PassFindingPart } from "./PassFinding";
import { PassFindingLink } from "./PassFindingLink";
import { PassFindings } from "./PassFindings";
import { PassNote } from "./PassNote";
import { PassSettled } from "./PassSettled";
import { PassShow } from "./PassShow";
import { PassStanding } from "./PassStanding";
import { PassSummary } from "./PassSummary";
import { findingWhere as where, standingWords } from "./passWords";
import css from "./PassView.module.css";
import { Text } from "../ui/Text";

// PassView is one of a review's passes, as the Passes view has it: how it
// stands, run (again), whether what it found is drawn in the diff, what
// it said, and what it found, open and settled, each to take, dismiss,
// ask about or open again.
export function PassView({
  api,
  p,
  hidden,
  canAsk,
  toFinding,
  toComment,
  onResolve,
  onPass,
  onTook,
  onHide,
  onAsk,
}: {
  api: string;
  p: ReviewPass;
  hidden: boolean;
  canAsk: boolean;
  toFinding: (pass: string, key: string) => boolean;
  toComment: (id: string) => boolean;
  onResolve: (thread: string) => Promise<unknown>;
  onPass: (p: ReviewPass) => void;
  onTook: () => void;
  onHide: (pass: string, hide: boolean) => void;
  onAsk: (pass: ReviewPass, f: PassFinding) => void;
}) {
  const [busy, setBusy] = useState(false);
  // Findings whose lines are shown here: they aren't in the diff to go to.
  const [lines, setLines] = useState<Record<string, boolean>>({});
  const [dismissing, setDismissing] = useState<string>();
  const failed = (e: Error) => toast(humanError(e.message), { tone: "danger" });
  const running = !!p.running || p.standing.status === "running";
  const run = () => {
    if (busy || running) return;
    setBusy(true);
    runPRPass(api, p.name)
      .then(onPass, failed)
      .finally(() => setBusy(false));
  };
  const settle = (f: PassFinding, state: "taken" | "dismissed" | "open", why?: DismissWhy) => {
    setDismissing(undefined);
    settlePRFinding(api, p.name, f.key, state, why).then((next) => {
      onPass(next);
      if (state === "taken") onTook();
    }, failed);
  };
  const open = p.findings.filter((f) => f.state === "open");
  const settled = p.findings.filter((f) => f.state !== "open");
  const onLines = open.some((f) => f.path && !f.thread);
  const steps = p.run?.calls ?? [];
  return (
    <PassCard
      standing={
        <PassStanding standing={p.standing} spin={running}>
          {running && p.runs ? "Running again" : standingWords(p.standing)}
        </PassStanding>
      }
      title={p.title}
      about={p.about}
      tools={
        <>
          {onLines && (
            <PassShow dim checked={!hidden} onChange={(on) => onHide(p.name, !on)} title="Draw what it found on the lines it's about, in the diff">
              In the diff
            </PassShow>
          )}
          <Button
            size="small"
            onClick={run}
            disabled={busy || running}
            unavailable={p.unavailable}
            title={p.cold ? "Its agent is given the change and the description, and nothing else" : "Its agent reads the change with the code checked out here (ws docks it first). It reads, and that's all"}
          >
            {running ? "Running…" : p.runs ? "Run again" : "Run"}
          </Button>
        </>
      }
    >
      {p.unavailable && <PassNote>It can't be run: {p.unavailable}.</PassNote>}
      {p.failed && !running && <Banner>{humanError(p.failed)}</Banner>}
      {running && steps.length > 0 && (
        <PassNote kind="steps">
          {steps.length} {steps.length === 1 ? "step" : "steps"}: {clip(`${steps[steps.length - 1].tool} ${steps[steps.length - 1].input ?? ""}`, 110)}
        </PassNote>
      )}
      {p.moved && !running && (
        <PassNote kind="moved">
          {" "}
          It's been pushed to since this ran{p.ran ? ` (at ${p.ran.slice(0, 7)})` : ""}: what it found may not stand. Run it again.
        </PassNote>
      )}
      {p.summary && <PassSummary text={p.summary} at={p.at} />}
      {open.length > 0 && (
        <PassFindings>
          {open.map((f) => (
            <PassFindingPart
              key={f.key}
              blocks={f.blocks}
              kind={
                <span className={`kind k-found${f.blocks ? " k-blocking" : ""}${f.kind === "addressed" ? " k-addressed" : ""}`} title={f.blocks ? "It blocks: the pass is no go while it's open" : undefined}>
                  {f.thread ? (followWords[f.kind] ?? f.kind) : f.kind}
                </span>
              }
              where={
                f.path ? (
                  <PassFindingLink onClick={() => (f.thread ? toComment(f.thread) : toFinding(p.name, f.key)) || setLines((l) => ({ ...l, [f.key]: !l[f.key] }))}>{where(f)}</PassFindingLink>
                ) : (
                  <Text dim>The change as a whole</Text>
                )
              }
              said={f.suggest ? <Text dim>· suggests a change</Text> : null}
              end={
                f.thread ? (
                  // It's about a comment you've made: there's nothing to take.
                  <span className={css.actions}>
                    <Button size="small" onClick={() => onResolve(f.thread!).catch(failed)} title="Resolve the comment's thread on GitHub: it's dealt with. It posts at once">
                      Resolve the comment
                    </Button>
                    <Button variant="ghost" size="small" onClick={() => settle(f, "dismissed", "wrong")} title="The follow-up has it wrong: don't count it">
                      Dismiss
                    </Button>
                  </span>
                ) : (
                  <ReviewFindingActions
                    dismissing={dismissing === f.key}
                    canAsk={canAsk}
                    onTake={() => settle(f, "taken")}
                    onDismissing={(on) => setDismissing(on ? f.key : undefined)}
                    onDismiss={(why) => settle(f, "dismissed", why)}
                    onAsk={() => onAsk(p, f)}
                  />
                )
              }
              lines={lines[f.key] ? <Excerpt lines={f.excerpt ?? []} from={f.line ?? 0} title="The lines it's on: they aren't in this diff" /> : undefined}
              text={f.text}
            />
          ))}
        </PassFindings>
      )}
      {settled.length > 0 && (
        <PassSettled summary={`${settled.length} settled: ${settled.filter((f) => f.state === "done").length} taken, ${settled.filter((f) => f.state === "dismissed").length} dismissed`}>
          <PassFindings>
            {settled.map((f) => (
              <PassFindingPart
                key={f.key}
                settled
                kind={<span className="kind k-found">{f.kind}</span>}
                where={<Text dim mono>{f.path ? where(f) : "The change as a whole"}</Text>}
                said={
                  <Text dim>
                    {f.state === "done" ? (f.thread ? "Its comment's resolved" : "Taken into your review") : `Dismissed${f.why ? `: ${reviewDismissWhys.find(([w]) => w === f.why)?.[1].toLowerCase()}` : ""}`}
                  </Text>
                }
                end={
                  <Button variant="ghost" size="small" onClick={() => settle(f, "open")} title="It's open again: it counts toward go or no go">
                    Open again
                  </Button>
                }
                text={f.text}
              />
            ))}
          </PassFindings>
        </PassSettled>
      )}
    </PassCard>
  );
}
