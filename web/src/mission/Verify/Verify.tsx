import { useEffect, useRef, useState } from "react";
import { act, type ActOpts, type ActVerb, type Bundle, type MissionDetail } from "../../api";
import { useKeys } from "../../keys";
import type { Reload } from "../../load";
import { Empty } from "../../ui/Empty";
import { Inline } from "../../ui/Inline";
import { Keys } from "../../ui/Keys";
import { humanError } from "../../ui/text";
import { toast } from "../../ui/toast";
import { checkRows, type CheckRow, rowKey, rowLine, sendable, stillLeft, toApprove, verdictTone } from "../verifyRows";
import { MissionVerify } from "./MissionVerify";
import { MissionVerifyApprove } from "./MissionVerifyApprove";
import { MissionVerifyGroup } from "./MissionVerifyGroup";
import { MissionVerifyList } from "./MissionVerifyList";
import { MissionVerifyPanes } from "./MissionVerifyPanes";
import { MissionVerifyRow } from "./MissionVerifyRow";
import { VerifyAllClear } from "./VerifyAllClear";
import { VerifyBundleHead } from "./VerifyBundleHead";
import { VerifyBundleRow } from "./VerifyBundleRow";
import { VerifyCheck } from "./VerifyCheck";
import { VerifyNothingLeft } from "./VerifyNothingLeft";
import { settles, verdictIcon } from "./words";

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
//
// This is what the tab knows and does: what you've said isn't right, the
// check that's open, what's sent to the daemon and where you go after.
// What it draws is the parts beside it (MissionVerify…), each given what
// to draw; the components named Verify… say which part a thing of the
// bundle's is drawn as.
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
    if (d.bundles.length) return <VerifyNothingLeft bundles={d.bundles} pick={pick} />;
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
  const row = (r: CheckRow, lines: number, kind?: "wanted" | "approved" | "settled") => {
    const notRight = !!r.check && mine[r.check.n] !== undefined;
    return (
      <MissionVerifyRow
        key={r.key}
        id={`item-${rowKey(bundle, r)}`}
        icon={notRight ? "x-circle" : verdictIcon[r.verdict]}
        tone={notRight ? "danger" : verdictTone(r.verdict)}
        title={<Inline text={r.title} lines={lines} />}
        line={
          <>
            {notRight ? "Not right, you say" : rowLine(bundle, r)}
            {r.looked && !r.left && " · looked at"}
          </>
        }
        selected={r === cur}
        kind={kind}
        onChoose={() => choose(r)}
      />
    );
  };
  const todo = toApprove(d.bundles);
  const settleTitle = going.length ? "Send it back to its crew, with what you said" : left.length ? "Approve it as it is: checks are still left for you" : "Approve it: what it shows is as its checks say";
  return (
    <MissionVerify fills>
      {(d.bundles.length > 1 || !todo.includes(bundle)) && <VerifyBundleRow bundles={d.bundles} open={bundle} pick={pick} />}
      <MissionVerifyPanes
        fills
        side={
          <>
            <VerifyBundleHead b={bundle} pick={pick} />
            <MissionVerifyList>
              {left.length > 0 && (
                <MissionVerifyGroup title="Left for you" n={left.length} attention>
                  {/* They stand out while the bundle's yours to approve. */}
                  {left.map((r) => row(r, 2, open ? "wanted" : undefined))}
                </MissionVerifyGroup>
              )}
              {open && (
                // The one to press once nothing's left for you; before that it still approves, as it is.
                <MissionVerifyApprove
                  plain={!going.length && left.length > 0}
                  disabled={busy}
                  onSettle={settle}
                  title={settleTitle}
                  keys="mod+⏎"
                  says={
                    going.length
                      ? `${going.length} ${going.length === 1 ? "check goes" : "checks go"} back to ${bundle.crew}, with what the bundle showed and what you said. The one they hand over next takes its place.`
                      : `${settles(bundle)}${left.length ? ` ${left.length} ${left.length === 1 ? "check is" : "checks are"} still left for you.` : ""}`
                  }
                >
                  {going.length ? `Send back to ${bundle.crew} (${going.length})` : `Approve ${bundle.id}`} <Keys k="mod+⏎" />
                </MissionVerifyApprove>
              )}
              {approved.length > 0 && (
                <MissionVerifyGroup title="You approved" n={approved.length} fold={{ open: showApproved, onToggle: () => setUnfolded(!showApproved) }}>
                  {showApproved && approved.map((r) => row(r, 1, "approved"))}
                </MissionVerifyGroup>
              )}
              {settled.length > 0 && (
                <MissionVerifyGroup title={left.length || approved.length ? "Settled without you" : "Its checks"} n={settled.length}>
                  {settled.map((r) => row(r, 1, "settled"))}
                </MissionVerifyGroup>
              )}
            </MissionVerifyList>
          </>
        }
      >
        {clear ? (
          <VerifyAllClear b={bundle} rows={rows} busy={busy} settle={settle} again={() => rows[0] && choose(rows[0])} />
        ) : (
          cur && <VerifyCheck b={bundle} r={cur} open={open} busy={busy} look={() => look(cur)} note={cur.check ? mine[cur.check.n] : undefined} setNote={(text) => cur.check && note(cur.check.n, text)} />
        )}
      </MissionVerifyPanes>
    </MissionVerify>
  );
}
