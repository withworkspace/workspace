import { useState } from "react";
import { Link } from "react-router";
import { act, type CapsuleDetail, type Conclusion, type EndedCapsule, type Ending, type MissionDetail } from "../api";
import type { Reload } from "../load";
import { Icon } from "../ui/Icon";
import { Inline } from "../ui/inline";
import { toast } from "../ui/toast";
import { humanError } from "../ui/text";
import { Dialog, DialogActions } from "../ui/Dialog";
import { Button } from "../ui/Button";
import { ToggleButton } from "../ui/ToggleButton";
import { Input } from "../ui/Input";
import { TextArea } from "../ui/TextArea";

// How a mission ends: landed (at Z, its code merged or landed), answered
// (a finding, no code), handed on (its work raised as other missions) or
// dropped, with each part of Z met or not, and how. The orchestrator
// suggests it; you end it. A capsule can end alone: handed on to a
// mission of its own, or dropped.

export const endingWords: Record<Ending, string> = { landed: "landed", answered: "answered", "handed-on": "handed on", dropped: "dropped" };

const endingHelp: Record<Ending, string> = {
  landed: "At Z, its code merged or landed",
  answered: "A finding, and no code: at Z, or stopping early",
  "handed-on": "Its work raised as other missions",
  dropped: "Given up",
};

// ZChecks is Z as checked: each part met or not, and how.
export function ZChecks({ c }: { c: Conclusion }) {
  if (!c.z?.length) return null;
  return (
    <ol className="z-checks">
      {c.z.map((z, i) => (
        <li key={i} className={z.met ? "met" : "unmet"}>
          <Icon name={z.met ? "check-circle" : "x-circle"} size={13} />
          <span>
            <span className="z-part">
              <Inline text={z.z} />
            </span>
            {z.how && (
              <span className="z-how">
                {" "}
                — <Inline text={z.how} />
              </span>
            )}
            {!z.how && !z.met && <span className="z-how dim"> — not checked</span>}
          </span>
        </li>
      ))}
    </ol>
  );
}

// HandedOn names the missions it was handed on to.
export function HandedOn({ to }: { to?: string[] }) {
  if (!to?.length) return null;
  return (
    <div className="handed-on dim">
      Handed on to{" "}
      {to.map((s, i) => (
        <span key={s}>
          {i > 0 && ", "}
          <Link to={`/missions/${s}`} className="link mono">
            {s}
          </Link>
        </span>
      ))}
    </div>
  );
}

// ended toasts an ending, with an undo, and reads the mission again: its
// promise is that read's (Reload).
export function ended(slug: string, message: string, reload: Reload): Promise<void> {
  toast(message, {
    tone: "done",
    action: {
      label: "Undo",
      run: () =>
        act(slug, "unend").then(
          (r) => {
            toast(r.message);
            reload();
          },
          (e: Error) => toast(humanError(e.message), { tone: "danger" }),
        ),
    },
  });
  return reload();
}

// EndDialog ends the mission as you say: how it ends, what it came to,
// and each part of Z met or not, with how. It starts from what's
// suggested, if anything is.
export function EndDialog({ d, onClose, reload }: { d: MissionDetail; onClose: () => void; reload: Reload }) {
  const from = d.conclude?.ending ? d.conclude : undefined;
  const z = d.z ?? [];
  const [ending, setEnding] = useState<Ending | undefined>(from?.ending);
  const [text, setText] = useState(from?.text ?? "");
  const [checks, setChecks] = useState<{ met?: boolean; how: string }[]>(() =>
    z.map((part) => {
      const was = from?.z?.find((c) => c.z === part);
      return was ? { met: was.met, how: was.how ?? "" } : { how: "" };
    }),
  );
  const [to, setTo] = useState((from?.to ?? []).join(" "));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const set = (i: number, c: Partial<{ met?: boolean; how: string }>) => setChecks((all) => all.map((x, j) => (j === i ? { ...x, ...c } : x)));
  const submit = () => {
    if (!ending) return;
    const met: string[] = [];
    const unmet: string[] = [];
    checks.forEach((c, i) => {
      if (c.met === true) met.push(`${i + 1}: ${c.how.trim()}`);
      if (c.met === false) unmet.push(`${i + 1}: ${c.how.trim()}`);
    });
    setBusy(true);
    setError("");
    act(d.slug, "end", { ending, text, met, unmet, to: ending === "handed-on" ? to.split(/[\s,]+/).filter(Boolean) : undefined })
      .then((r) => {
        onClose();
        return ended(d.slug, r.message, reload);
      })
      .catch((e: Error) => setError(e.message))
      .finally(() => setBusy(false));
  };
  const handedOn = d.endedCapsules?.filter((c) => c.ending === "handed-on" && c.to).map((c) => c.to!) ?? [];
  return (
    <Dialog title="End the mission" onClose={onClose} className="end-mission" wide>
      <p>How it ends, what it came to, and each part of Z: met or not, and how. It's in the past then, out of what needs you; archive it to put it out of sight.</p>
      <div className="verdicts" role="radiogroup" aria-label="How it ends">
        {(Object.keys(endingWords) as Ending[]).map((e) => (
          <button key={e} role="radio" aria-checked={ending === e} className={`verdict v-end${ending === e ? " on" : ""}`} onClick={() => setEnding(e)} title={endingHelp[e]}>
            {endingWords[e][0].toUpperCase() + endingWords[e].slice(1)}
          </button>
        ))}
      </div>
      {ending && <div className="dim end-help">{endingHelp[ending]}.</div>}
      <TextArea value={text} rows={3} placeholder="What it came to: the finding, what landed, why it stops" aria-label="What it came to" onChange={(e) => setText(e.target.value)} />
      {ending === "handed-on" && (
        <label className="end-to">
          <span>Handed on to</span>
          <Input value={to} onChange={(e) => setTo(e.target.value)} placeholder={handedOn.length ? `and ${handedOn.join(", ")}, its capsules'` : "mission slugs"} />
        </label>
      )}
      {z.length > 0 && (
        <fieldset className="z-form">
          <legend>Z{ending === "dropped" ? ", if you want to say" : ""}</legend>
          {z.map((part, i) => (
            <div key={i} className="z-row">
              <div className="z-part">
                <span className="dim">{i + 1}.</span> <Inline text={part} />
              </div>
              <div className="z-say">
                <ToggleButton size="small" className={checks[i].met === true ? "on-met" : undefined} pressed={checks[i].met === true} onChange={(on) => set(i, { met: on ? true : undefined })}>
                  <Icon name="check" size={12} /> Met
                </ToggleButton>
                <ToggleButton size="small" className={checks[i].met === false ? "on-unmet" : undefined} pressed={checks[i].met === false} onChange={(on) => set(i, { met: on ? false : undefined })}>
                  <Icon name="x" size={12} /> Not met
                </ToggleButton>
                <Input value={checks[i].how} onChange={(e) => set(i, { how: e.target.value })} placeholder={checks[i].met === false ? "Why not" : "How: what shows it"} aria-label={`How part ${i + 1} of Z is met, or why not`} />
              </div>
            </div>
          ))}
        </fieldset>
      )}
      {error && <div className="banner">{humanError(error)}</div>}
      <DialogActions>
        <Button variant="ghost" onClick={onClose}>
          Cancel
        </Button>
        <Button variant={ending === "dropped" ? "danger" : "primary"} onClick={submit} disabled={!ending || !text.trim() || busy}>
          {ending ? `End it, ${endingWords[ending]}` : "End it"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}

// EndButton opens the End dialog: ending the mission yourself, whatever's
// suggested.
export function EndButton({ d, reload }: { d: MissionDetail; reload: Reload }) {
  const [open, setOpen] = useState(false);
  if (d.ending || d.state === "archived") return null;
  return (
    <>
      <Button variant="ghost" size="small" onClick={() => setOpen(true)} title="Say how it ends, with Z checked">
        <Icon name="check-circle" size={12} /> End
      </Button>
      {open && <EndDialog d={d} onClose={() => setOpen(false)} reload={reload} />}
    </>
  );
}

// CapsuleEnds are what ends a capsule alone: handed on to a mission of its
// own (raised for it, or one there is), which takes it, or dropped.
export function CapsuleEnds({ slug, c, reload }: { slug: string; c: CapsuleDetail; reload: Reload }) {
  const [open, setOpen] = useState<"hand-on" | "drop">();
  const [text, setText] = useState("");
  const [to, setTo] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const name = `${c.repo}/${c.name}`;
  const close = () => {
    setOpen(undefined);
    setError("");
  };
  const submit = () => {
    if (!open) return;
    setBusy(true);
    act(slug, open, { capsule: name, text, to: to.trim() ? [to.trim()] : undefined })
      .then((r) => {
        close();
        toast(r.message, { tone: "done" });
        return reload();
      })
      .catch((e: Error) => setError(e.message))
      .finally(() => setBusy(false));
  };
  return (
    <>
      <Button variant="ghost" size="small" onClick={() => setOpen("hand-on")} title="Hand it on to a mission of its own, which takes it">
        <Icon name="arrow-right" size={12} /> Hand on
      </Button>
      <Button variant="ghost" size="small" onClick={() => setOpen("drop")} title="Drop it: its work won't be part of the mission">
        <Icon name="x" size={12} /> Drop
      </Button>
      {open && (
        <Dialog title={open === "drop" ? `Drop ${name}` : `Hand ${name} on`} onClose={close} className="end-mission" wide>
          <p>
            {open === "drop"
              ? "Its work won't be part of the mission, and it no longer holds it open. Its worktree is kept, in no mission."
              : "A mission is raised for it from what you say, to be briefed, or name one there is: it takes the capsule, which no longer holds this one open."}
          </p>
          <TextArea
            value={text}
            rows={3}
            autoFocus
            placeholder={open === "drop" ? "Why it's dropped" : "What the mission it goes to is for"}
            aria-label={open === "drop" ? "Why" : "What it's for"}
            onChange={(e) => setText(e.target.value)}
          />
          {open === "hand-on" && (
            <label className="end-to">
              <span>Or to a mission there is</span>
              <Input value={to} onChange={(e) => setTo(e.target.value)} placeholder="its slug" />
            </label>
          )}
          {error && <div className="banner">{humanError(error)}</div>}
          <DialogActions>
            <Button variant="ghost" onClick={close}>
              Cancel
            </Button>
            <Button variant={open === "drop" ? "danger" : "primary"} onClick={submit} disabled={busy || (!text.trim() && !(open === "hand-on" && to.trim()))}>
              {open === "drop" ? "Drop it" : "Hand it on"}
            </Button>
          </DialogActions>
        </Dialog>
      )}
    </>
  );
}

// EndedCapsules are the capsules that ended alone: no longer the
// mission's, and not holding it open.
export function EndedCapsules({ list }: { list?: EndedCapsule[] }) {
  if (!list?.length) return null;
  return (
    <ul className="ended-capsules">
      {list.map((c) => (
        <li key={`${c.repo}/${c.name}`} title={c.why}>
          <Icon name={c.ending === "dropped" ? "x" : "arrow-right"} size={12} className="dim" />
          <span className="mono">
            {c.repo}/{c.name}
          </span>{" "}
          {c.ending === "dropped" ? (
            <span className="dim">dropped{c.why ? `: ${c.why}` : ""}</span>
          ) : (
            <span className="dim">
              handed on to{" "}
              <Link to={`/missions/${c.to}`} className="link mono">
                {c.to}
              </Link>
            </span>
          )}
        </li>
      ))}
    </ul>
  );
}
