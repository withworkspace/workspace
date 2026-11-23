import { useEffect, useRef, useState } from "react";
import { Link } from "react-router";
import { act, type ActOpts, type ActVerb, type Bundle, type BundleEvidence, type BundleTry, type MissionDetail } from "../api";
import type { Reload } from "../load";
import { useKeys } from "../keys";
import { Icon, type IconName } from "../ui/Icon";
import { Inline } from "../ui/Inline";
import { toast } from "../ui/toast";
import { Empty } from "../ui/Empty";
import { Callout } from "../ui/Callout";
import { Copyable } from "../ui/Copyable";
import { Keys } from "../ui/Keys";
import { Time } from "../ui/Time";
import {
  allClearWords,
  type BundleGroup,
  checkRows,
  type CheckRow,
  groupTitles,
  nothingLeftWords,
  queued,
  rowKey,
  rowLine,
  sendable,
  shortCommit,
  standing,
  stillLeft,
  tally,
  toApprove,
  verdictTone,
  verdictWords,
  waiting,
} from "./verifyRows";
import { Chip } from "../ui/Chip";
import { Count } from "../ui/Count";
import { Button } from "../ui/Button";
import { Clamp } from "../ui/Clamp";
import { MoreLink } from "../ui/MoreLink";
import { capital, humanError } from "../ui/text";
import { ToggleButton } from "../ui/ToggleButton";
import { cx } from "../ui/cx";
import { Menu, MenuItem, MenuSection, MenuSeparator, MenuText } from "../ui/Menu";
import { TextArea } from "../ui/TextArea";

// The mission's Verify tab: its verification bundles, what crew handed
// over to show their work is as its checks say. Those yours to approve are
// in a row at the top, the rest behind a button. One bundle is open, in
// two columns that fill the tab and scroll by themselves: who made it and
// of what, its summary, then its checks in the order your attention's
// needed; and beside them the check that's open, with what shows it
// (images and videos as they are, text as a transcript). A bundle is yours
// to approve or send back, as a review is. A check left for you is yours
// to approve (it leaves what's left for a fold of its own) or to say isn't
// right; under what's left is the bundle's approval, or its sending back
// with what you said. With the last check approved its pane says you're
// clear, and with the last bundle the tab says nothing's left.

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
  bundle?: Bundle; // the one open: none when nothing's yours to approve and you haven't opened one
  pick: (id?: string) => void; // open a bundle, or (none) close the one that's open
  sel?: string; // the page's selected item
  select: (key: string) => void;
  reload: Reload;
}) {
  // What you've said isn't right, a check each, until it's sent back: by
  // bundle, then by the check's number.
  const [notes, setNotes] = useState<Record<string, Record<number, string>>>({});
  const [busy, setBusy] = useState(false);
  // The bundle whose last check left for you you've just approved: its
  // pane says you're clear, until you go to a check again.
  const [cleared, setCleared] = useState<string>();
  // Whether the checks you've approved are unfolded.
  const [unfolded, setUnfolded] = useState(false);
  useEffect(() => setCleared(undefined), [sel]);
  useEffect(() => setUnfolded(false), [bundle?.id]);

  const rows = bundle ? checkRows(bundle) : [];
  // It's yours to give a verdict of until you have, or something takes its
  // place.
  const open = !!bundle && !bundle.verdict && !bundle.replacedBy;
  const mine = bundle ? (notes[bundle.id] ?? {}) : {};
  const going = sendable(mine);
  const left = stillLeft(rows);
  const clear = open && cleared === bundle?.id && !going.length;
  const cur = bundle && !clear ? (rows.find((r) => rowKey(bundle, r) === sel) ?? rows[0]) : undefined;
  const choose = (r: CheckRow) => {
    if (!bundle) return;
    setCleared(undefined);
    select(rowKey(bundle, r));
  };

  // The bundle you're on now, for what's done once the daemon has
  // answered: none once you've left the tab, or the page.
  const here = useRef<string | undefined>(undefined);
  useEffect(() => {
    here.current = bundle?.id;
    return () => {
      here.current = undefined;
    };
  }, [bundle?.id]);

  const run = (verb: ActVerb, opts: ActOpts, quiet = false, then?: () => void) => {
    if (!bundle) return;
    setBusy(true);
    act(d.slug, verb, { bundle: bundle.id, ...opts })
      .then((r) => {
        if (!quiet) toast(r.message, { tone: "done" });
        if (verb === "bundle-send-back") setNotes((all) => ({ ...all, [bundle.id]: {} }));
        // What follows (on to the next bundle) is for where you were when
        // you said it: gone somewhere else since, you're left there.
        if (here.current === bundle.id) then?.();
        return reload();
      })
      .catch((e: Error) => {
        setCleared(undefined);
        toast(humanError(e.message), { tone: "danger" });
      })
      .finally(() => setBusy(false));
  };
  // Marking a check: one left for you is approved, and leaves what's left
  // for the next of them, or, when it was the last, for the all clear. A
  // settled one is looked at, to keep your place: on to the next that isn't.
  const look = (r: CheckRow) => {
    if (!bundle) return;
    run("bundle-looked", { check: r.key, looked: !r.looked }, true);
    if (r.looked) return;
    const next = r.left ? left.find((x) => x !== r) : rows.find((x) => x !== r && !x.left && !x.looked);
    if (next) choose(next);
    else if (r.left && !going.length) setCleared(bundle.id);
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
  // Approved or sent back, it's on to the next that's yours to approve;
  // with none, the tab says there's nothing left.
  const settle = () => {
    if (!bundle) return;
    const on = () => pick(toApprove(d.bundles).find((b) => b.id !== bundle.id)?.id);
    if (going.length) run("bundle-send-back", { notes: going }, false, on);
    else run("bundle-approve", {}, false, on);
  };
  useKeys({
    y: () => open && cur && look(cur),
    n: () => open && cur?.check && note(cur.check.n, mine[cur.check.n] ?? ""),
    "mod+Enter": () => open && !busy && settle(),
  });

  if (!bundle) {
    if (d.bundles.length) return <NothingLeft bundles={d.bundles} pick={pick} />;
    return (
      <Empty icon="list-checks" title="No bundles yet">
        A verification bundle is what crew hand over to show their work is as its checks say: each check with what it came to and what shows it. Crew hand one over with{" "}
        <code>ws mission bundle --set -</code>.
      </Empty>
    );
  }
  const approved = rows.filter((r) => r.left && r.looked);
  const settled = rows.filter((r) => !r.left);
  const showApproved = unfolded || (!!cur && approved.includes(cur));
  const row = (r: CheckRow, lines: number) => {
    const notRight = !!r.check && mine[r.check.n] !== undefined;
    return (
      <button key={r.key} id={`item-${rowKey(bundle, r)}`} type="button" className={`verify-row${r === cur ? " selected" : ""}`} aria-pressed={r === cur} onClick={() => choose(r)}>
        <span className={`verify-mark tone-${notRight ? "danger" : verdictTone(r.verdict)}`}>
          <Icon name={notRight ? "x-circle" : verdictIcon[r.verdict]} size={16} />
        </span>
        <span className="verify-row-text">
          <span className="verify-row-title">
            <Inline text={r.title} lines={lines} />
          </span>
          <span className="verify-row-line">
            {notRight ? "Not right, you say" : rowLine(bundle, r)}
            {r.looked && !r.left && " · looked at"}
          </span>
        </span>
      </button>
    );
  };
  const todo = toApprove(d.bundles);
  const settleTitle = going.length ? "Send it back to its crew, with what you said" : left.length ? "Approve it as it is: checks are still left for you" : "Approve it: what it shows is as its checks say";
  return (
    <div className="verify">
      {(d.bundles.length > 1 || !todo.includes(bundle)) && <BundleRow bundles={d.bundles} open={bundle} pick={pick} />}
      <div className="verify-panes">
        <div className="verify-side">
          <BundleHead b={bundle} pick={pick} />
          <div className={`verify-list${open ? " verify-list-open" : ""}`}>
            {left.length > 0 && (
              <div className="verify-group verify-left">
                <h3 className="tone-attention">
                  Left for you<span className="verify-n">{left.length}</span>
                </h3>
                {left.map((r) => row(r, 2))}
              </div>
            )}
            {open && (
              <div className="verify-approve">
                {/* The one to press once nothing's left for you; before that it still approves, as it is. */}
                <Button variant={going.length || !left.length ? "primary" : undefined} className={going.length || !left.length ? undefined : "verify-plain"} disabled={busy} onClick={settle} title={settleTitle}>
                  {going.length ? `Send back to ${bundle.crew} (${going.length})` : `Approve ${bundle.id}`} <Keys k="mod+⏎" />
                </Button>
                <span role="status" className="verify-approve-text">
                  {going.length
                    ? `${going.length} ${going.length === 1 ? "check goes" : "checks go"} back to ${bundle.crew}, with what the bundle showed and what you said. The one they hand over next takes its place.`
                    : `${settles(bundle)}${left.length ? ` ${left.length} ${left.length === 1 ? "check is" : "checks are"} still left for you.` : ""}`}
                </span>
              </div>
            )}
            {approved.length > 0 && (
              <div className="verify-group verify-approved">
                <h3>
                  <button type="button" className="verify-fold-toggle" aria-expanded={showApproved} onClick={() => setUnfolded(!showApproved)}>
                    <Icon name={showApproved ? "chevron-down" : "chevron-right"} size={12} />
                    You approved<span className="verify-n">{approved.length}</span>
                  </button>
                </h3>
                {showApproved && approved.map((r) => row(r, 1))}
              </div>
            )}
            {settled.length > 0 && (
              <div className="verify-group verify-settled">
                <h3>
                  {left.length || approved.length ? "Settled without you" : "Its checks"}
                  <span className="verify-n">{settled.length}</span>
                </h3>
                {settled.map((r) => row(r, 1))}
              </div>
            )}
          </div>
        </div>
        <div className="verify-pane">
          {clear ? (
            <AllClear b={bundle} rows={rows} busy={busy} settle={settle} again={() => rows[0] && choose(rows[0])} />
          ) : (
            cur && (
              <CheckDetail
                b={bundle}
                r={cur}
                open={open}
                busy={busy}
                look={() => look(cur)}
                note={cur.check ? mine[cur.check.n] : undefined}
                setNote={(text) => cur.check && note(cur.check.n, text)}
              />
            )
          )}
        </div>
      </div>
    </div>
  );
}

// settles is what approving a bundle does, and that it's stale where it is.
function settles(b: Bundle): string {
  return `${b.asked ? `Approving ${b.id} passes step ${b.askedAt}.` : `No step waits on ${b.id}: your verdict is kept with it.`}${b.stale ? " It's stale: its capsule has moved on since." : ""}`;
}

// stepTitle is what a bundle is of, as its step is listed.
function stepTitle(b: Bundle): string {
  if (b.step && b.stepName) return `Step ${b.step}: ${b.stepName}`;
  return b.capsule ? `Of ${b.capsule}` : "Of no step";
}

// standingTone is the tone a bundle's standing is said in.
function standingTone(b: Bundle): string {
  if (b.replacedBy) return "";
  if (b.verdict === "approved") return b.stale ? "" : " tone-done";
  if (waiting(b) || b.verdict === "sent back" || b.stale) return " tone-attention";
  return "";
}

// BundleRow is the mission's bundles in a line: those yours to approve,
// each with how many of its checks are left for you; the one that's open,
// when it isn't one of those (with a way to close it when there are none);
// and the rest behind a button.
function BundleRow({ bundles, open, pick }: { bundles: Bundle[]; open: Bundle; pick: (id?: string) => void }) {
  const groups = queued(bundles);
  const todo = groups.find((g) => g.group === "todo")?.bundles ?? [];
  const rest = groups.filter((g) => g.group !== "todo");
  const others = rest.reduce((n, g) => n + g.bundles.length, 0);
  const chip = (b: Bundle, other = false) => {
    const left = !b.verdict && !b.replacedBy ? stillLeft(checkRows(b)).length : 0;
    return (
      <button key={b.id} type="button" className={`verify-bundle${b.id === open.id ? " selected" : ""}${other ? " verify-bundle-other" : ""}`} aria-pressed={b.id === open.id} onClick={() => pick(b.id)}>
        <span className="ref">{b.id}</span>
        <span className="verify-bundle-title">{stepTitle(b)}</span>
        <span className={`verify-bundle-state${standingTone(b)}`}>{standing(b)}</span>
        {left > 0 && (
          <Count tone="attention" title={`${left} of its checks ${left === 1 ? "is" : "are"} left for you`}>
            {left}
          </Count>
        )}
      </button>
    );
  };
  return (
    <div className="verify-bundles" role="group" aria-label="This mission's verification bundles">
      <span className={`verify-bundles-label${todo.length ? " tone-attention" : ""}`}>{todo.length ? "To approve" : "Nothing to approve"}</span>
      {todo.map((b) => chip(b))}
      {!todo.includes(open) && chip(open, true)}
      {!todo.length && (
        <Button variant="ghost" size="small" square type="button" onClick={() => pick()} title={`Close ${open.id} (esc)`} aria-label={`Close ${open.id}`}>
          <Icon name="x" size={13} />
        </Button>
      )}
      {others > 0 && <BundleMenu groups={rest} label={`${others} ${todo.length ? "other " : ""}${others === 1 ? "bundle" : "bundles"}`} pick={pick} end />}
    </div>
  );
}

// BundleMenu is the bundles that aren't in the row, behind a button: by
// where they stand, those that no longer stand sitting back.
function BundleMenu({ groups, label, pick, end }: { groups: { group: BundleGroup; bundles: Bundle[] }[]; label: string; pick: (id?: string) => void; end?: boolean }) {
  // data-field: keys typed in the menu are its own, not the page's (keys.ts).
  return (
    <Menu
      className="verify-menu"
      placement={end ? "bottom end" : "bottom"}
      aria-label="This mission's verification bundles"
      onAction={pick}
      trigger={
        <Button className={cx("verify-more", end && "verify-more-end")} data-field>
          {label} <Icon name="chevron-down" size={12} />
        </Button>
      }
    >
      {groups.flatMap((g, i) => [
        ...(i > 0 ? [<MenuSeparator key={`sep-${g.group}`} />] : []),
        <MenuSection key={g.group} title={groupTitles[g.group]}>
          {g.bundles.map((b) => (
            <MenuItem key={b.id} id={b.id} textValue={`${b.id} ${stepTitle(b)}`} className={cx("verify-menu-item", g.group === "stale" && "verify-menu-stale")}>
              <span className="ref">{b.id}</span>
              <MenuText>{stepTitle(b)}</MenuText>
              <span className={`verify-bundle-state${standingTone(b)}`}>{standing(b)}</span>
            </MenuItem>
          ))}
        </MenuSection>,
      ])}
    </Menu>
  );
}

// NothingLeft is the whole tab when no bundle is yours to approve and you
// haven't opened one: that, where the rest stand, and a way to them.
function NothingLeft({ bundles, pick }: { bundles: Bundle[]; pick: (id?: string) => void }) {
  return (
    <div className="verify verify-done">
      <Empty
        big
        done
        icon="check"
        title="Nothing left to approve"
        action={<BundleMenu groups={queued(bundles)} label={bundles.length === 1 ? "Look at the bundle" : `Look at the ${bundles.length} bundles`} pick={pick} />}
      >
        {nothingLeftWords(bundles)} A bundle that needs you will show here, and on the tab.
      </Empty>
    </div>
  );
}

// AllClear is the check's pane once the last check left for you is
// approved: that, what still stands as it was, and the bundle's approval
// at a size you can't miss.
function AllClear({ b, rows, busy, settle, again }: { b: Bundle; rows: CheckRow[]; busy: boolean; settle: () => void; again: () => void }) {
  return (
    <article className="verify-check verify-clear" aria-label="Nothing's left for you">
      <Empty
        big
        done
        icon="check"
        title="You're all clear"
        action={
          <Button variant="primary" className="verify-big" disabled={busy} onClick={settle} title="Approve it, with the checks as you've approved them">
            Approve {b.id} <Keys k="mod+⏎" />
          </Button>
        }
        foot={
          <>
            <div className="verify-clear-after">{settles(b)}</div>
            <MoreLink onClick={again}>Look again</MoreLink>
          </>
        }
      >
        {allClearWords(b, rows)}
      </Empty>
    </article>
  );
}

// BundleHead is the open bundle: who made it and of what, whether it's
// still of its capsule as it is, what you've said of it, its summary (a
// few lines, until you ask for the rest), and, folded, what it didn't
// verify and how to try it.
function BundleHead({ b, pick }: { b: Bundle; pick: (id?: string) => void }) {
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
                  <Copyable text={b.commit} title="Copy the commit">
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
        <Callout className="verify-head-note" tone="quiet" icon="info" actions={<Button onClick={() => pick(b.replacedBy!)}>Open {b.replacedBy}</Button>}>
          {b.replacedBy} has taken its place.
        </Callout>
      )}
      {b.verdict === "approved" && (
        <Callout className="verify-head-note" tone="done" icon="check-circle">
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
        <Callout className="verify-head-note"
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
        <Callout className="verify-head-note" tone="attention" title="Stale">
          Its capsule has moved on to <span className="mono">{shortCommit(b.head)}</span> since. What this shows is of <span className="mono">{shortCommit(b.commit)}</span>.
        </Callout>
      )}
      <Summary text={b.summary} />
      <div className="verify-tally">
        {tally(b).map(([v, n]) => (
          <Chip tone={verdictTone(v)} key={v}>
            {n} {v}
          </Chip>
        ))}
        {b.replaces && (
          <span className="verify-replaces">
            It took the place of{" "}
            <Button variant="link" type="button" onClick={() => pick(b.replaces!)}>
              {b.replaces}
            </Button>
            .
          </span>
        )}
      </div>
      {b.notVerified?.length ? (
        <details className="verify-fold">
          <summary>
            Not verified<span className="verify-n">{b.notVerified.length}</span>
          </summary>
          <ul>
            {b.notVerified.map((n, i) => (
              <li key={i}>
                <Inline text={n} />
              </li>
            ))}
          </ul>
        </details>
      ) : null}
      {b.noticed?.length ? (
        <details className="verify-fold">
          <summary>
            Noticed<span className="verify-n">{b.noticed.length}</span>
          </summary>
          <ul>
            {b.noticed.map((n, i) => (
              <li key={i}>
                <Inline text={n} />
              </li>
            ))}
          </ul>
        </details>
      ) : null}
      {b.try?.length ? (
        <details className="verify-fold">
          <summary>
            Try it<span className="verify-n">{b.try.length}</span>
          </summary>
          <ul>
            {b.try.map((t, i) => (
              <li key={i}>
                <Try t={t} />
              </li>
            ))}
          </ul>
        </details>
      ) : null}
    </section>
  );
}

// Summary is what a bundle's crew say of it, a few lines of it until you
// ask for the rest: its checks are what you came for.
function Summary({ text }: { text: string }) {
  return (
    <Clamp className="verify-summary" tall={7.6} more="More" less="Less" of={text}>
      {text.split(/\n\s*\n/).map((para, i) => (
        <p key={i}>
          <Inline text={para} />
        </p>
      ))}
    </Clamp>
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
// said of it, what shows it, and (while the bundle's yours to settle, kept
// at the pane's foot as the check scrolls) what's yours of it: Approve
// check for one left for you, Looked at for one that's settled, and Not
// right.
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
          <Chip tone={verdictTone(r.verdict)}>{verdictWords[r.verdict]}</Chip>
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
        {c?.points?.length ? (
          <ul className="verify-points">
            {c.points.map((pt, i) => (
              <li key={i}>
                <Inline text={pt} />
              </li>
            ))}
          </ul>
        ) : null}
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
      {r.ran?.kept && (
        <p className="verify-kept">
          <Link to={`/runs?show=${encodeURIComponent(r.ran.kept)}`}>
            <Icon name="terminal" size={11} /> All it printed, and what it wrote
          </Link>
        </p>
      )}
      {c?.shows?.map((e) => (
        <Evidence key={e.name} e={e} />
      ))}
      {c?.lines?.length ? (
        <p className="verify-lines">
          Where it's done:{" "}
          {c.lines.map((l, i) => (
            <span key={l}>
              {i > 0 && ", "}
              <Copyable text={l} title="Copy where it is" />
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
      {c?.asks?.length ? (
        open ? (
          <div className="verify-asks">
            <h4>{b.crew} suggests, to send back</h4>
            <div className="verify-ask-chips">
              {c.asks.map((a, i) => (
                <Button key={i} type="button" onClick={() => setNote(a)} title="Send this back with the bundle, as what's to do">
                  {a}
                </Button>
              ))}
            </div>
          </div>
        ) : (
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
        )
      ) : null}
      {open && (
        <div className="verify-yours">
          {c && note !== undefined && (
            <label className="verify-note">
              <span>
                What's not right? It goes back to {b.crew} with this check and what's shown here.
              </span>
              <TextArea autoFocus rows={2} value={note} onChange={(e) => setNote(e.target.value)} placeholder="Say what you saw" />
            </label>
          )}
          <div className="verify-yours-buttons">
            <ToggleButton
              className={r.looked ? "on" : undefined}
              pressed={!!r.looked}
              disabled={busy}
              onChange={look}
              title={r.left ? "Say it's fine as it is: it leaves what's left for you" : "Mark it as looked at, to keep your place: approving doesn't need it"}
            >
              <Icon name="check" size={14} /> {r.left ? "Approve check" : "Looked at"} <Keys k="y" />
            </ToggleButton>
            {c && (
              <ToggleButton
                variant={note !== undefined ? "danger" : undefined}
                className={note !== undefined ? "on on-danger" : undefined}
                pressed={note !== undefined}
                onChange={(on) => setNote(on ? "" : undefined)}
                title="Say what's not right of it: it goes back to its crew when you send the bundle back"
              >
                <Icon name="x" size={14} /> Not right <Keys k="n" />
              </ToggleButton>
            )}
          </div>
        </div>
      )}
    </article>
  );
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
        <img src={e.url} alt={label ? `${label}: ${what}` : what} data-viewable={label ? `${label}: ${what}` : what} title="Open it larger" />
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
