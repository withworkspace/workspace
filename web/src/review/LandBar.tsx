import { useState } from "react";
import { useNavigate } from "react-router";
import { getLanding, landCapsule, type Landing, type LandStopped } from "../api";
import { useShortcut } from "../keys";
import { Icon } from "../ui/Icon";
import { toast } from "../ui/toast";
import { Callout } from "../ui/Callout";
import { Keys } from "../ui/Keys";
import { useLoaded } from "../load";
import { Button } from "../ui/Button";
import css from "./LandBar.module.css";
import { Text } from "../ui/Text";

// LandBar is how a capsule lands when its repo has no pull requests: its
// commits go straight to the default branch, when you say. It sits on the
// capsule's review, since that's the only review there'll be: approve it,
// then land it (two acts; L, then ⏎). Nothing shows for a repo that takes
// pull requests.
//
// Landing is ws land: rebased onto the default branch, checked with the
// repo's verify command, pushed, and the capsule burned. A landing that
// stops (a clash, a failed check, a refused push) pushes nothing, and says
// why here.
export function LandBar({ repo, name, updated, back }: { repo: string; name: string; updated?: unknown; back: string }) {
  const navigate = useNavigate();
  const [asking, setAsking] = useState(false);
  const [busy, setBusy] = useState(false);
  const [stopped, setStopped] = useState<LandStopped>();
  const [error, setError] = useState<string>();
  const { value: l, reload: load } = useLoaded<Landing | undefined>(() => getLanding(repo, name).catch(() => undefined), [repo, name, updated], { keep: true });

  const can = !!l?.direct && l.commits.length > 0 && !l.dirty && !l.landed;
  const land = () => {
    setBusy(true);
    setError(undefined);
    setStopped(undefined);
    landCapsule(repo, name)
      .then(
        (r) => {
          setAsking(false);
          if (r.stopped) {
            setStopped(r.stopped);
            load();
            return;
          }
          if (!r.landed) return;
          const n = r.landed.commits.length;
          toast(`Landed ${repo}/${name} on ${r.landed.branch}: ${n} ${n === 1 ? "commit" : "commits"}`, { tone: "done" });
          for (const w of r.landed.warnings ?? []) toast(w, { ms: 8000 });
          // The capsule's gone: back to what it was for.
          navigate(back);
        },
        (e: Error) => setError(e.message),
      )
      .finally(() => setBusy(false));
  };
  // L asks, then ⏎ lands or esc takes it back: each key's bound here, and
  // its button is given what comes back, which says the key.
  const ask = useShortcut("L", () => setAsking(true), { enabled: can && !busy });
  const confirm = { enabled: asking && !busy, modal: asking };
  const landIt = useShortcut("Enter", land, confirm);
  const cancel = useShortcut("Escape", () => setAsking(false), confirm);

  if (!l?.direct) return null;
  const n = l.commits.length;
  const commits = `${n} ${n === 1 ? "commit" : "commits"}`;
  const how = (
    <>
      There's no pull request: {n === 1 ? "it goes" : "they go"} straight to {l.branch}, rebased onto it{l.verify ? <>, once <Text mono>{l.verify}</Text> passes</> : null}. The capsule is burned after.
    </>
  );
  if (l.landed) {
    return (
      <Callout tone="done" title={`Landed on ${l.landed.branch}, at ${l.landed.head.slice(0, 7)}`}>
        The capsule's still here, with nothing left to land: ws debrief removes it.
      </Callout>
    );
  }
  return (
    <>
      {stopped && (
        <Callout tone="danger" title="Landing stopped" actions={<Button onClick={() => setStopped(undefined)}>Dismiss</Button>}>
          {stopped.message}
          {stopped.output && <pre className={css.output}>{stopped.output}</pre>}
        </Callout>
      )}
      {error && <Callout tone="danger" title="Couldn't land it">{error}</Callout>}
      {l.pending && n === 0 ? (
        <Callout
          tone="attention"
          title="Landing needs to be checked"
          actions={
            <Button variant="primary" onClick={land} disabled={busy}>
              {busy ? "Checking…" : "Retry landing"}
            </Button>
          }
        >
          Check whether the earlier push landed and finish recording it.
        </Callout>
      ) : n === 0 ? (
        <Callout tone="quiet" icon="info" title={`Nothing to land yet`}>
          This repo has no pull requests: what's committed here lands straight on {l.branch}, when you say.
        </Callout>
      ) : l.dirty ? (
        <Callout tone="attention" title={`${commits} to land on ${l.branch}, and changes not committed`}>
          {how} What isn't committed wouldn't land: commit it (or put it aside) first.
        </Callout>
      ) : asking ? (
        <Callout
          tone="attention"
          icon="alert"
          title={`Land ${commits} on ${l.branch}?`}
          actions={
            <>
              <Button variant="primary" onClick={landIt} disabled={busy}>
                {busy ? "Landing…" : `Land on ${l.branch}`} <Keys k="⏎" />
              </Button>
              <Button variant="ghost" onClick={cancel} disabled={busy}>
                Cancel <Keys k="esc" />
              </Button>
            </>
          }
        >
          Everyone sees {n === 1 ? "it" : "them"} at once, and it can't be taken back: a mistake is put right with another commit.
          {!l.approved && " You haven't approved this as it is now."}
          <ul className={css.commits}>
            {l.commits.map((c, i) => (
              <li key={i}>{c}</li>
            ))}
          </ul>
        </Callout>
      ) : (
        <Callout
          tone={l.approved ? "done" : "info"}
          icon={l.approved ? "check-circle" : "info"}
          title={l.approved ? `Approved: land its ${commits} on ${l.branch}` : `${commits} to land on ${l.branch}, when you say`}
          actions={
            <Button variant={l.approved ? "primary" : undefined} onClick={ask}>
              <Icon name="arrow-right" size={13} /> Land on {l.branch} <Keys k="L" />
            </Button>
          }
        >
          {how}
          {!l.approved && " You're the only review there is: read it, and approve it, before it lands."}
        </Callout>
      )}
    </>
  );
}
