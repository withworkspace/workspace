import { useEffect, useState } from "react";
import { act, type ActOpts, type ActVerb, type Bundle, type BundleEvidence, type BundleTry, type MissionDetail } from "../api";
import { useKeys } from "../keys";
import { Icon, type IconName } from "../ui/Icon";
import { Inline } from "../ui/inline";
import { Callout, Copyable, Empty, humanError, Keys, toast } from "../ui/kit";
import { Time } from "../ui/time";
import { checkRows, type CheckRow, listed, rowKey, rowLine, sendable, shortCommit, standing, tally, verdictTone, verdictWords, waiting } from "./verifyRows";

// The mission's Verify tab: its verification bundles, what crew handed
// over to show their work is as its checks say. One bundle is open: who
// made it and of what, its summary, how to try it, then its checks in the
// order your attention's needed, one at a time with what shows it (images
// and videos as they are, text as a transcript). A bundle is yours to
// approve or send back, as a review is: Looked at keeps your place, Not
// right says what's wrong of a check, and the bar at the foot approves it
// or sends it back with what you said.

const verdictIcon: Record<string, IconName> = { met: "check-circle", unmet: "x-circle", "can't tell": "question", yours: "user" };

export function Verify({
  d,
  bundle,
  pick,
  sel,
  select,
  reload,
}: {
  d: MissionDetail;
  bundle?: Bundle; // the one open
  pick: (id: string) => void;
  sel?: string; // the page's selected item
  select: (key: string) => void;
  reload: () => void;
}) {
  // What you've said isn't right, a check each, until it's sent back: by
  // bundle, then by the check's number.
  const [notes, setNotes] = useState<Record<string, Record<number, string>>>({});
  const [busy, setBusy] = useState(false);
  const rows = bundle ? checkRows(bundle) : [];
  const cur = bundle ? (rows.find((r) => rowKey(bundle, r) === sel) ?? rows[0]) : undefined;
  // It's yours to give a verdict of until you have, or something takes its
  // place.
  const open = !!bundle && !bundle.verdict && !bundle.replacedBy;
  const mine = bundle ? (notes[bundle.id] ?? {}) : {};
  const going = sendable(mine);

  const run = (verb: ActVerb, opts: ActOpts, quiet = false) => {
    if (!bundle) return;
    setBusy(true);
    act(d.slug, verb, { bundle: bundle.id, ...opts })
      .then((r) => {
        if (!quiet) toast(r.message, { tone: "done" });
        if (verb === "bundle-send-back") setNotes((all) => ({ ...all, [bundle.id]: {} }));
        reload();
      })
      .catch((e: Error) => toast(humanError(e.message), { tone: "danger" }))
      .finally(() => setBusy(false));
  };
  const look = (r: CheckRow) => {
    if (!bundle) return;
    run("bundle-looked", { check: r.key, looked: !r.looked }, true);
    // Looked at: on to the next that isn't.
    const next = rows.find((x) => x !== r && !x.looked);
    if (!r.looked && next) select(rowKey(bundle, next));
  };
  const note = (n: number, text?: string) => {
    if (!bundle) return;
    setNotes((all) => {
      const of = { ...(all[bundle.id] ?? {}) };
      if (text === undefined) delete of[n];
      else of[n] = text;
      return { ...all, [bundle.id]: of };
    });
  };
  const settle = () => (going.length ? run("bundle-send-back", { notes: going }) : run("bundle-approve", {}));
  useKeys({
    y: () => open && cur && look(cur),
    n: () => open && cur?.check && note(cur.check.n, mine[cur.check.n] ?? ""),
    "mod+Enter": () => open && !busy && settle(),
  });

  if (!bundle) {
    return (
      <Empty icon="list-checks" title="No bundles yet">
        A verification bundle is what crew hand over to show their work is as its checks say: each check with what it came to and what shows it. Crew hand one over with{" "}
        <code>ws mission bundle --set -</code>.
      </Empty>
    );
  }
  const left = rows.filter((r) => r.left);
  const settled = rows.filter((r) => !r.left);
  const group = (title: string, tone: string, list: CheckRow[]) =>
    list.length > 0 && (
      <div className="verify-group">
        <h3 className={tone}>{title}</h3>
        {list.map((r) => {
          const notRight = !!r.check && mine[r.check.n] !== undefined;
          return (
            <button
              key={r.key}
              id={`item-${rowKey(bundle, r)}`}
              type="button"
              className={`verify-row${r === cur ? " selected" : ""}`}
              aria-pressed={r === cur}
              onClick={() => select(rowKey(bundle, r))}
            >
              <span className={`verify-mark tone-${notRight ? "danger" : verdictTone(r.verdict)}`}>
                <Icon name={notRight ? "x-circle" : verdictIcon[r.verdict]} size={16} />
              </span>
              <span className="verify-row-text">
                <span className="verify-row-title">
                  <Inline text={r.title} lines={2} />
                </span>
                <span className="verify-row-line">
                  {notRight ? "Not right, you say" : rowLine(bundle, r)}
                  {r.looked && " · looked at"}
                </span>
              </span>
            </button>
          );
        })}
      </div>
    );
  const unlooked = left.filter((r) => !r.looked).length;
  return (
    <div className="verify">
      {d.bundles.length > 1 && (
        <div className="verify-bundles" role="group" aria-label="This mission's verification bundles">
          {listed(d.bundles).map((b) => (
            <button key={b.id} type="button" className={`verify-bundle${b.id === bundle.id ? " selected" : ""}`} aria-pressed={b.id === bundle.id} onClick={() => pick(b.id)}>
              <span className="ref">{b.id}</span>
              <span className="verify-bundle-text">
                <span className="verify-bundle-title">{stepTitle(b)}</span>
                <span className={`verify-bundle-state${standingTone(b)}`}>{standing(b)}</span>
              </span>
            </button>
          ))}
        </div>
      )}
      <BundleHead b={bundle} pick={pick} />
      {bundle.try?.length ? (
        <section className="verify-try" aria-label="Try it">
          <h3>Try it</h3>
          <ul>
            {bundle.try.map((t, i) => (
              <li key={i}>
                <Try t={t} />
              </li>
            ))}
          </ul>
        </section>
      ) : null}
      <div className="verify-body">
        <div className="verify-list">
          {group("Left for you", "tone-attention", left)}
          {group(left.length ? "Settled without you" : "Its checks", "", settled)}
        </div>
        {cur && (
          <CheckDetail
            b={bundle}
            r={cur}
            open={open}
            busy={busy}
            look={() => look(cur)}
            note={cur.check ? mine[cur.check.n] : undefined}
            setNote={(text) => cur.check && note(cur.check.n, text)}
          />
        )}
      </div>
      {open && (
        <div className="verify-foot">
          <span role="status" className="verify-foot-text">
            {going.length
              ? `${going.length} ${going.length === 1 ? "check goes" : "checks go"} back to ${bundle.crew}, with what the bundle showed and what you said. The one they hand over next takes its place.`
              : `${bundle.asked ? `Approving ${bundle.id} passes step ${bundle.askedAt}.` : `No step waits on ${bundle.id}: your verdict is kept with it.`}${
                  bundle.stale ? " It's stale: its capsule has moved on since." : ""
                }${unlooked ? ` ${unlooked} of the checks left for you ${unlooked === 1 ? "isn't" : "aren't"} marked as looked at.` : ""}`}
          </span>
          <button className="primary" disabled={busy} onClick={settle} title={going.length ? "Send it back to its crew, with what you said" : "Approve it: what it shows is as its checks say"}>
            {going.length ? `Send back to ${bundle.crew} (${going.length})` : `Approve ${bundle.id}`} <Keys k="mod+⏎" />
          </button>
        </div>
      )}
    </div>
  );
}

// stepTitle is what a bundle is of, as its step is listed.
function stepTitle(b: Bundle): string {
  if (b.step && b.stepName) return `Step ${b.step}: ${b.stepName}`;
  return b.capsule ? `Of ${b.capsule}` : "Of no step";
}

// standingTone is the tone a bundle's standing is said in.
function standingTone(b: Bundle): string {
  if (b.replacedBy) return "";
  if (b.verdict === "approved") return " tone-done";
  if (waiting(b) || b.verdict === "sent back" || b.stale) return " tone-attention";
  return "";
}

// BundleHead is the open bundle: who made it and of what, whether it's
// still of its capsule as it is, what you've said of it, its summary, and
// what it didn't verify.
function BundleHead({ b, pick }: { b: Bundle; pick: (id: string) => void }) {
  return (
    <section className="verify-head" aria-label="The bundle">
      <div className="verify-head-top">
        <h2>
          <span className="ref">{b.id}</span> {stepTitle(b)}
        </h2>
        <span className="verify-meta">
          From {b.crew}
          {b.by ? ` (${b.by})` : ""}
          {b.own ? ", who did the work it shows" : ", a verifier who didn't do the work"}
          {b.capsule && (
            <>
              , of <span className="mono">{b.capsule}</span>
              {b.commit && (
                <>
                  {" "}
                  at{" "}
                  <Copyable text={b.commit} className="mono" title="Copy the commit">
                    {shortCommit(b.commit)}
                  </Copyable>
                </>
              )}
            </>
          )}
          , <Time iso={b.at} as="relative" />
        </span>
        {b.capsule && b.commit && !b.stale && !b.replacedBy && <span className="verify-current">Current: its capsule hasn't moved since</span>}
      </div>
      {b.replacedBy && (
        <Callout tone="quiet" icon="info" actions={<button onClick={() => pick(b.replacedBy!)}>Open {b.replacedBy}</button>}>
          {b.replacedBy} has taken its place.
        </Callout>
      )}
      {b.verdict === "approved" && (
        <Callout tone="done" icon="check-circle">
          You approved it
          {b.verdictAt && (
            <>
              {" "}
              <Time iso={b.verdictAt} as="relative" />
            </>
          )}
          .
        </Callout>
      )}
      {b.verdict === "sent back" && (
        <Callout
          tone="attention"
          icon="reply"
          title={
            <>
              You sent it back
              {b.verdictAt && (
                <>
                  {" "}
                  <Time iso={b.verdictAt} as="relative" />
                </>
              )}
            </>
          }
        >
          <ul className="verify-notes">
            {(b.notes ?? []).map((n, i) => (
              <li key={i}>
                {n.check ? `Check ${n.check}: ` : ""}
                <Inline text={n.text} />
              </li>
            ))}
          </ul>
          {!b.replacedBy && <>It's with its crew: the bundle they hand over next takes its place.</>}
        </Callout>
      )}
      {b.stale && !b.replacedBy && (
        <Callout tone="attention" title="Stale">
          Its capsule has moved on to <span className="mono">{shortCommit(b.head)}</span> since. What this shows is of <span className="mono">{shortCommit(b.commit)}</span>.
        </Callout>
      )}
      <div className="verify-summary">
        {b.summary.split(/\n\s*\n/).map((para, i) => (
          <p key={i}>
            <Inline text={para} />
          </p>
        ))}
      </div>
      <div className="verify-tally">
        {tally(b).map(([v, n]) => (
          <span key={v} className={`chip tone-${verdictTone(v)}`}>
            {n} {v}
          </span>
        ))}
        {b.replaces && (
          <span className="verify-replaces">
            It took the place of{" "}
            <button type="button" className="link" onClick={() => pick(b.replaces!)}>
              {b.replaces}
            </button>
            .
          </span>
        )}
      </div>
      {b.notVerified?.length ? (
        <div className="verify-not">
          <h3>Not verified</h3>
          <ul>
            {b.notVerified.map((n, i) => (
              <li key={i}>
                <Inline text={n} />
              </li>
            ))}
          </ul>
        </div>
      ) : null}
    </section>
  );
}

// Try is something to do to see it yourself: its command to copy, then
// the rest in words; or what to look for.
function Try({ t }: { t: BundleTry }) {
  if (t.look) {
    return (
      <span className="verify-look">
        <span className="dim">Look for</span> <Inline text={t.text} />
      </span>
    );
  }
  if (!t.command) return <Inline text={t.text} />;
  const rest = t.text.replace(/^`[^`]+`\s*/, "");
  return (
    <>
      <Copyable text={t.command} className="verify-command" title="Copy the command" />
      {rest && (
        <>
          {" "}
          <Inline text={rest} />
        </>
      )}
    </>
  );
}

// CheckDetail is one check of the open bundle: what it came to, what was
// said of it, what shows it, and (while the bundle's yours to settle)
// Looked at and Not right.
function CheckDetail({
  b,
  r,
  open,
  busy,
  look,
  note,
  setNote,
}: {
  b: Bundle;
  r: CheckRow;
  open: boolean;
  busy: boolean;
  look: () => void;
  note?: string; // what you've said isn't right of it; undefined when you haven't
  setNote: (text?: string) => void;
}) {
  const c = r.check;
  return (
    <article className="verify-check" aria-label="The check, and what shows it">
      <header>
        <div className="verify-check-top">
          <span className={`chip tone-${verdictTone(r.verdict)}`}>{verdictWords[r.verdict]}</span>
          <span className="verify-meta">
            {r.n ? `Check ${r.n}, ` : ""}
            {r.ran ? "ws ran it" : r.verdict === "yours" ? `left for you by ${b.crew}` : `${b.crew}${b.own ? ", who did the work" : ", its verifier"}`}
          </span>
        </div>
        <h3>
          <Inline text={r.title} />
        </h3>
        {c?.check && (
          <p className="verify-said">
            <Inline text={c.said} />
          </p>
        )}
      </header>
      {c?.judged && (
        <p className={`verify-judged tone-${verdictTone(c.judged.result)}`}>
          <strong>
            {capital(c.judged.by)} says {c.judged.result}
          </strong>
          {c.judged.evidence ? (
            <>
              : <Inline text={c.judged.evidence} />
            </>
          ) : (
            "."
          )}
        </p>
      )}
      {r.ran?.evidence && <pre className="verify-output">{r.ran.evidence}</pre>}
      {c?.shows?.map((e) => (
        <Evidence key={e.name} e={e} />
      ))}
      {c?.lines?.length ? (
        <p className="verify-lines">
          Where it's done:{" "}
          {c.lines.map((l, i) => (
            <span key={l}>
              {i > 0 && ", "}
              <Copyable text={l} className="mono" title="Copy where it is" />
            </span>
          ))}
        </p>
      ) : null}
      {c?.try?.length ? (
        <div className="verify-yourself">
          <h4>To see it yourself</h4>
          <ul>
            {c.try.map((t, i) => (
              <li key={i}>
                <Try t={t} />
              </li>
            ))}
          </ul>
        </div>
      ) : null}
      {open && (
        <div className="verify-yours">
          <div className="verify-yours-buttons">
            <button type="button" className={r.looked ? "on" : ""} aria-pressed={r.looked} disabled={busy} onClick={look} title="Mark it as looked at, to keep your place: approving doesn't need it">
              <Icon name="check" size={14} /> Looked at <Keys k="y" />
            </button>
            {c && (
              <button
                type="button"
                className={note !== undefined ? "on danger" : ""}
                aria-pressed={note !== undefined}
                onClick={() => setNote(note === undefined ? "" : undefined)}
                title="Say what's not right of it: it goes back to its crew when you send the bundle back"
              >
                <Icon name="x" size={14} /> Not right <Keys k="n" />
              </button>
            )}
          </div>
          {c && note !== undefined && (
            <label className="verify-note">
              <span>
                What's not right? It goes back to {b.crew} with this check and what's shown here.
              </span>
              <textarea autoFocus rows={2} value={note} onChange={(e) => setNote(e.target.value)} placeholder="Say what you saw" />
            </label>
          )}
          {c?.asks?.length ? (
            <div className="verify-asks">
              <h4>{b.crew} suggests, to send back</h4>
              <div className="verify-ask-chips">
                {c.asks.map((a, i) => (
                  <button key={i} type="button" onClick={() => setNote(a)} title="Send this back with the bundle, as what's to do">
                    {a}
                  </button>
                ))}
              </div>
            </div>
          ) : null}
        </div>
      )}
      {!open && c?.asks?.length ? (
        <div className="verify-asks">
          <h4>{b.crew} suggested</h4>
          <ul>
            {c.asks.map((a, i) => (
              <li key={i}>
                <Inline text={a} />
              </li>
            ))}
          </ul>
        </div>
      ) : null}
    </article>
  );
}

function capital(s: string): string {
  return s.slice(0, 1).toUpperCase() + s.slice(1);
}

// Evidence is a file that shows a check, as it is: an image (beside the
// same on ground, where the bundle has it), a video to play, or text as a
// transcript; then what it shows, and the command that made it.
function Evidence({ e }: { e: BundleEvidence }) {
  return (
    <figure className="verify-evidence">
      {e.kind === "image" && e.before ? (
        <div className="verify-pair">
          <Shot e={e.before} label="Before, on ground" />
          <Shot e={e} label="After, in the capsule" />
        </div>
      ) : (
        <Shot e={e} />
      )}
      {(e.caption || e.madeBy) && (
        <figcaption>
          {e.caption && <Inline text={e.caption} />}
          {e.madeBy && (
            <span className="verify-made">
              Made by <Copyable text={e.madeBy} className="verify-command" title="Copy the command" />
            </span>
          )}
        </figcaption>
      )}
    </figure>
  );
}

// Shot is one file, shown as its kind.
function Shot({ e, label }: { e: BundleEvidence; label?: string }) {
  const what = e.caption || e.name;
  return (
    <div className="verify-shot">
      {label && <span className="verify-shot-label">{label}</span>}
      {e.kind === "video" ? (
        <video src={e.url} controls preload="metadata" aria-label={what} />
      ) : e.kind === "image" ? (
        <a href={e.url} target="_blank" rel="noopener noreferrer" title="Open it on its own, full size">
          <img src={e.url} alt={label ? `${label}: ${what}` : what} />
        </a>
      ) : (
        <Transcript url={e.url} />
      )}
    </div>
  );
}

// Transcript is a bundle's text, read from where it's served.
function Transcript({ url }: { url: string }) {
  const [text, setText] = useState<string>();
  const [failed, setFailed] = useState(false);
  useEffect(() => {
    let gone = false;
    setText(undefined);
    setFailed(false);
    fetch(url)
      .then((res) => (res.ok ? res.text() : Promise.reject(new Error(String(res.status)))))
      .then((t) => !gone && setText(t))
      .catch(() => !gone && setFailed(true));
    return () => {
      gone = true;
    };
  }, [url]);
  if (failed) return <p className="verify-meta">It couldn't be read.</p>;
  return <pre className="verify-output">{text ?? " "}</pre>;
}
