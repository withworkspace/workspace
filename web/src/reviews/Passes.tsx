import { useState } from "react";
import { type DismissWhy, type PassFinding, type PassMark, type ReviewPass, runPRPass, settlePRFinding } from "../api";
import { FindingActions, followWords, reviewDismissWhys } from "../review/Findings";
import { Banner } from "../ui/Banner";
import { Icon } from "../ui/Icon";
import { Spinner } from "../ui/Spinner";
import { Inline } from "../ui/inline";
import { toast } from "../ui/toast";
import { Time } from "../ui/Time";
import { Button } from "../ui/Button";
import { clip, humanError } from "../ui/text";
import { Checkbox } from "../ui/Checkbox";
import { Excerpt } from "../ui/Excerpt";

// A review's passes, a view of its own: each pass, go or no go, what it said
// of the change, and what it found, to take into your review or dismiss. Go
// or no go is worked out from what's still open, so it moves as you settle
// them.

export const standingWords = (st: PassMark["standing"]): string => {
  const over = st.overruled ? ` · ${st.overruled} ${st.overruled === 1 ? "blocker" : "blockers"} dismissed` : "";
  switch (st.status) {
    case "none":
      return "Not run";
    case "running":
      return "Running";
    case "failed":
      return "Couldn't run";
    case "nogo":
      return `No go · ${st.blocking} blocking`;
    case "notes":
      return `Go, with notes · ${st.open}${over}`;
    default:
      return `Go${over}`;
  }
};

export const standingIcon = (st: PassMark["standing"]) => (st.status === "nogo" || st.status === "failed" ? "alert" : st.status === "go" || st.status === "notes" ? "check-circle" : "sparkle");

const where = (f: PassFinding) => `${f.path}:${f.line}${f.endLine && f.line && f.endLine > f.line ? `–${f.endLine}` : ""}${f.side === "old" ? " (removed)" : ""}`;

export function PassesSection({
  api,
  passes,
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
  passes?: ReviewPass[];
  hidden: Record<string, boolean>; // passes whose findings aren't drawn in the diff
  canAsk: boolean;
  toFinding: (pass: string, key: string) => boolean; // whether it's in the diff to go to
  toComment: (id: string) => boolean;
  onResolve: (thread: string) => Promise<unknown>; // resolve the comment a follow-up's finding is about
  onPass: (p: ReviewPass) => void; // a pass as it now stands
  onTook: () => void; // a finding went into the draft
  onHide: (pass: string, hide: boolean) => void;
  onAsk: (pass: ReviewPass, f: PassFinding) => void;
}) {
  if (!passes) {
    return (
      <>
        <h2>Passes</h2>
        <p className="dim">Reading the passes…</p>
      </>
    );
  }
  return (
    <>
      <h2>Passes</h2>
      {passes.length === 0 ? (
        <p className="dim">
          No pass is set up for reviews here. One is a skill or a few words saying what to look for, in <span className="mono">ws.toml</span> (the team's) or{" "}
          <span className="mono">ws.local.toml</span> (yours): <span className="mono">[reviews.passes.&lt;name&gt;]</span>.
        </p>
      ) : (
        <div className="rpass-list">
          {passes.map((p) => (
            <PassView key={p.name} api={api} p={p} hidden={!!hidden[p.name]} canAsk={canAsk} toFinding={toFinding} toComment={toComment} onResolve={onResolve} onPass={onPass} onTook={onTook} onHide={onHide} onAsk={onAsk} />
          ))}
        </div>
      )}
    </>
  );
}

function PassView({
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
    <article className={`review-pass st-${p.standing.status}`}>
      <header>
        <span className={`rpass-standing st-${p.standing.status}`}>
          {running ? <Spinner size={13} /> : <Icon name={standingIcon(p.standing)} size={13} />} {running && p.runs ? "Running again" : standingWords(p.standing)}
        </span>
        <strong>{p.title}</strong>
        <span className="dim rpass-about">{p.about}</span>
        <span className="spacer-left" />
        {onLines && (
          <Checkbox className="rpass-show dim" checked={!hidden} onChange={(on) => onHide(p.name, !on)} title="Draw what it found on the lines it's about, in the diff">
            In the diff
          </Checkbox>
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
      </header>
      {p.unavailable && <p className="dim rpass-note">It can't be run: {p.unavailable}.</p>}
      {p.failed && !running && <Banner>{humanError(p.failed)}</Banner>}
      {running && steps.length > 0 && (
        <p className="dim rpass-note mono">
          {steps.length} {steps.length === 1 ? "step" : "steps"}: {clip(`${steps[steps.length - 1].tool} ${steps[steps.length - 1].input ?? ""}`, 110)}
        </p>
      )}
      {p.moved && !running && (
        <p className="rpass-note rpass-moved">
          <Icon name="rotate" size={12} /> It's been pushed to since this ran{p.ran ? ` (at ${p.ran.slice(0, 7)})` : ""}: what it found may not stand. Run it again.
        </p>
      )}
      {p.summary && (
        <p className="rpass-summary">
          <Inline text={p.summary} /> {p.at && <Time iso={p.at} as="relative" className="dim" />}
        </p>
      )}
      {open.length > 0 && (
        <ul className="rpass-findings">
          {open.map((f) => (
            <li key={f.key} className={f.blocks ? "blocks" : ""}>
              <div className="rpass-finding-head">
                <span className={`kind k-found${f.blocks ? " k-blocking" : ""}${f.kind === "addressed" ? " k-addressed" : ""}`} title={f.blocks ? "It blocks: the pass is no go while it's open" : undefined}>
                  {f.thread ? (followWords[f.kind] ?? f.kind) : f.kind}
                </span>
                {f.path ? (
                  <Button
                    variant="link"
                    className="finding-link"
                    onClick={() => (f.thread ? toComment(f.thread) : toFinding(p.name, f.key)) || setLines((l) => ({ ...l, [f.key]: !l[f.key] }))}
                    title="Go to it in the diff, or, where its lines aren't in the diff, show them here"
                  >
                    {where(f)}
                  </Button>
                ) : (
                  <span className="dim">The change as a whole</span>
                )}
                {f.suggest ? <span className="dim">· suggests a change</span> : null}
                <span className="spacer-left" />
                {f.thread ? (
                  // It's about a comment you've made: there's nothing to take.
                  <span className="finding-actions">
                    <Button size="small" onClick={() => onResolve(f.thread!).catch(failed)} title="Resolve the comment's thread on GitHub: it's dealt with. It posts at once">
                      Resolve the comment
                    </Button>
                    <Button variant="ghost" size="small" onClick={() => settle(f, "dismissed", "wrong")} title="The follow-up has it wrong: don't count it">
                      Dismiss
                    </Button>
                  </span>
                ) : (
                  <FindingActions
                    dismissing={dismissing === f.key}
                    canAsk={canAsk}
                    onTake={() => settle(f, "taken")}
                    onDismissing={(on) => setDismissing(on ? f.key : undefined)}
                    onDismiss={(why) => settle(f, "dismissed", why)}
                    onAsk={() => onAsk(p, f)}
                  />
                )}
              </div>
              {lines[f.key] && (
                <Excerpt lines={f.excerpt ?? []} from={f.line ?? 0} title="The lines it's on: they aren't in this diff" />
              )}
              <div className="rpass-finding-text">
                <Inline text={f.text} />
              </div>
            </li>
          ))}
        </ul>
      )}
      {settled.length > 0 && (
        <details className="rpass-settled">
          <summary className="dim">
            {settled.length} settled: {settled.filter((f) => f.state === "done").length} taken, {settled.filter((f) => f.state === "dismissed").length} dismissed
          </summary>
          <ul className="rpass-findings">
            {settled.map((f) => (
              <li key={f.key} className="settled">
                <div className="rpass-finding-head">
                  <span className="kind k-found">{f.kind}</span>
                  <span className="dim mono">{f.path ? where(f) : "The change as a whole"}</span>
                  <span className="dim">
                    {f.state === "done" ? (f.thread ? "Its comment's resolved" : "Taken into your review") : `Dismissed${f.why ? `: ${reviewDismissWhys.find(([w]) => w === f.why)?.[1].toLowerCase()}` : ""}`}
                  </span>
                  <span className="spacer-left" />
                  <Button variant="ghost" size="small" onClick={() => settle(f, "open")} title="It's open again: it counts toward go or no go">
                    Open again
                  </Button>
                </div>
                <div className="rpass-finding-text dim">
                  <Inline text={f.text} />
                </div>
              </li>
            ))}
          </ul>
        </details>
      )}
    </article>
  );
}

// PassMarks are how each pass that's been run stands, a mark each: beside
// Passes in the list down the side, so go or no go shows from any view.
export function PassMarks({ passes }: { passes?: ReviewPass[] }) {
  const ran = (passes ?? []).filter((p) => p.standing.status !== "none" || p.running);
  if (!ran.length) return null;
  return (
    <>
      {ran.map((p) => {
        const running = !!p.running || p.standing.status === "running";
        return (
          <span key={p.name} className={`rpass-mark st-${running ? "running" : p.standing.status}`} title={`${p.title}: ${running ? "running" : standingWords(p.standing).toLowerCase()}`}>
            {running ? <Spinner size={11} /> : <Icon name={standingIcon(p.standing)} size={11} />}
          </span>
        );
      })}
    </>
  );
}

// RowPasses are how the passes run over a pull request stand, on its row
// in the Reviews list: what you know before you open it.
export function RowPasses({ marks }: { marks?: PassMark[] }) {
  if (!marks?.length) return null;
  return (
    <>
      {marks.map((m) => (
        <span
          key={m.name}
          className={`rpass-standing st-${m.standing.status}${m.moved ? " moved" : ""}`}
          title={`The ${m.title} pass: ${standingWords(m.standing).toLowerCase()}${m.moved ? ". It's been pushed to since it ran" : ""}`}
        >
          {m.standing.status === "running" ? <Spinner size={11} /> : <Icon name={standingIcon(m.standing)} size={11} />} {m.title}:{" "}
          {standingWords(m.standing).toLowerCase()}
        </span>
      ))}
    </>
  );
}
