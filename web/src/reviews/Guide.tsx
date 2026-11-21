import { useEffect, useState } from "react";
import { type GuideText, type ReviewGuide, runPRGuide } from "../api";
import type { Order } from "../review/files";
import { Icon } from "../ui/Icon";
import { Inline } from "../ui/inline";
import { humanError, toast } from "../ui/kit";
import { duration, Time } from "../ui/time";
import { Button } from "../ui/Button";
import { capital } from "../ui/text";

// A review's guide, a view of its own: what the change does, where that
// differs from what it says it does, and the order to read its files in.
// It hides nothing: the list of files can follow its order, or their
// folders'.

// GuideMark is how the guide stands, beside its name in the list down the
// side: being written, or written before the last push.
export function GuideMark({ guide }: { guide?: ReviewGuide }) {
  if (guide?.running) return <Icon name="spinner" size={11} className="spin" title="Writing the guide…" />;
  if (guide?.failed) return <Icon name="alert" size={11} className="mark-bad" title="The guide couldn't be written" />;
  if (guide?.guide && guide.moved) return <Icon name="rotate" size={11} className="dim" title="It's been pushed to since the guide was written" />;
  return null;
}

// guideOrder is a guide's groups as an order for the list of files.
export function guideOrder(g: ReviewGuide): Order | undefined {
  if (!g.guide) return undefined;
  const notes: Record<string, string> = {};
  // A file's line is written to follow its path; it's shown on a line of
  // its own, so it starts with a capital.
  for (const group of g.guide.groups) for (const f of group.files) if (f.note) notes[f.path] = capital(f.note);
  return {
    key: `${g.id}:${g.at}`,
    stale: g.moved,
    groups: g.guide.groups.map((x) => ({ name: x.name, why: x.why, skim: x.skim, files: x.files.map((f) => f.path) })),
    notes,
  };
}

export function GuideSection({ api, guide, guided, onGuide, onGuided }: { api: string; guide?: ReviewGuide; guided: boolean; onGuide: (g: ReviewGuide) => void; onGuided: (on: boolean) => void }) {
  const [busy, setBusy] = useState(false);
  if (!guide) {
    return (
      <>
        <h2>Guide</h2>
        <p className="dim">Reading the guide…</p>
      </>
    );
  }
  const run = () => {
    if (busy || guide.running) return;
    setBusy(true);
    runPRGuide(api)
      .then(onGuide, (e: Error) => toast(humanError(e.message), { tone: "danger" }))
      .finally(() => setBusy(false));
  };
  const g = guide.guide;
  const running = !!guide.running || busy;
  return (
    <>
      <h2>Guide</h2>
      <div className="guide">
        {!g && !running && (
          <p className="dim">
            What the change does, where that differs from what it says it does, and the order to read its files in. An agent writes it from the change and the description alone.{" "}
            <Button size="small" onClick={run}>
              Write the guide
            </Button>
          </p>
        )}
        {running && <GuideWriting guide={guide} again={!!g} />}
        {guide.failed && !running && <div className="banner">{humanError(guide.failed)}</div>}
        {running && guide.draft ? (
          <GuideBody g={guide.draft} writing />
        ) : (
          g && (
            <>
              {guide.moved && !running && (
                <p className="rpass-note rpass-moved">
                  <Icon name="rotate" size={12} /> It's been pushed to since this was written{guide.ran ? ` (at ${guide.ran.slice(0, 7)})` : ""}: files it doesn't place come first.
                </p>
              )}
              <GuideBody g={g} />
            </>
          )
        )}
        {g && !(running && guide.draft) && (
          <>
            <div className="guide-foot">
              <label className="rpass-show" title="The list of files, and the diff under it, follow the guide's order. Nothing's hidden: a file it doesn't place is still there">
                <input type="checkbox" checked={guided} onChange={(e) => onGuided(e.target.checked)} /> Read the files in this order
              </label>
              <span className="spacer-left" />
              {guide.at && <Time iso={guide.at} as="relative" className="dim" />}
              <Button variant="ghost" size="small" onClick={run} disabled={running}>
                Write it again
              </Button>
            </div>
          </>
        )}
      </div>
    </>
  );
}

// GuideBody is a guide: what the change does, where that differs from its
// description, and its groups. One being written (writing) is as far as
// it's got: nothing's said of what differs until it is, and it ends on a
// caret.
function GuideBody({ g, writing }: { g: GuideText; writing?: boolean }) {
  const caret = writing ? <span className="guide-caret" aria-hidden /> : null;
  const groups = g.groups.length > 0;
  return (
    <div className={writing ? "guide-draft" : undefined} aria-busy={writing || undefined}>
      {g.does && (
        <p className="guide-does">
          <Inline text={g.does} />
          {!g.differs?.length && !groups && caret}
        </p>
      )}
      {g.differs?.length ? (
        <div className="guide-differs">
          <strong>Where it differs from what it says it does</strong>
          <ul>
            {g.differs.map((d, i) => (
              <li key={i}>
                <Inline text={d} />
                {i === g.differs!.length - 1 && !groups && caret}
              </li>
            ))}
          </ul>
        </div>
      ) : (
        // Being written, it's said nothing differs once it's on to the files.
        (!writing || groups) && <p className="dim guide-same">It does what its description says.</p>
      )}
      {groups && (
        <ol className="guide-groups">
          {g.groups.map((x, i) => (
            <li key={i} className={x.skim ? "skim" : ""}>
              <strong>{x.name}</strong>{" "}
              <span className="dim">
                · {x.files.length} {x.files.length === 1 ? "file" : "files"}
                {writing && i === g.groups.length - 1 ? " so far" : ""}
                {x.skim ? " · to skim" : ""}
              </span>
              {x.why && (
                <span className="guide-why">
                  <Inline text={x.why} />
                  {i === g.groups.length - 1 && caret}
                </span>
              )}
              {!x.why && i === g.groups.length - 1 && caret}
            </li>
          ))}
        </ol>
      )}
    </div>
  );
}

// leftNames are what a file left out of what the guide's agent read is
// left out for, counted: "2 lockfiles".
const leftNames: Record<string, [string, string]> = {
  lockfile: ["lockfile", "lockfiles"],
  generated: ["generated", "generated"],
  snapshot: ["snapshot", "snapshots"],
  renamed: ["only renamed", "only renamed"],
  binary: ["binary", "binary"],
  "too large": ["too large", "too large"],
  "no room": ["past the room", "past the room"],
};

// leftWords says what was left out, by why: "2 lockfiles, 1 generated".
function leftWords(left: { why: string }[]): string {
  const n = new Map<string, number>();
  for (const l of left) n.set(l.why, (n.get(l.why) ?? 0) + 1);
  return [...n].map(([why, k]) => `${k} ${(leftNames[why] ?? [why, why])[k === 1 ? 0 : 1]}`).join(", ");
}

// GuideWriting says the guide's being written: for how long, and what its
// agent was given to write it from.
function GuideWriting({ guide, again }: { guide: ReviewGuide; again: boolean }) {
  // Its own clock, each second: the page's ticks each minute.
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (!guide.since) return;
    const t = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(t);
  }, [guide.since]);
  const given = guide.given;
  const left = given?.left ?? [];
  return (
    <div className="guide-writing">
      <p className="dim">
        <Icon name="spinner" className="spin" size={12} /> {!guide.since ? "Waiting to start…" : again ? "Writing it again" : "Writing the guide"}
        {guide.since && <span title="Since it started"> · {duration(guide.since, now)}</span>}
        {guide.thinking && <span title="Its agent thinks before it writes: what it thinks isn't shown"> · thinking it over</span>}
      </p>
      {given && (
        <p className="dim guide-given" title={left.length ? `Not shown to it:\n${left.map((l) => `${l.path} (${l.why})`).join("\n")}` : undefined}>
          From the description and the changes to {given.files.length - left.length} of {given.files.length} {given.files.length === 1 ? "file" : "files"}
          {left.length ? ` · not shown: ${leftWords(left)}` : ""}
        </p>
      )}
    </div>
  );
}
