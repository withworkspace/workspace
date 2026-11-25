import { useState } from "react";
import { Link } from "react-router";
import { act, type CapsuleDetail, type Conclusion, type EndedCapsule, type MissionDetail } from "../api";
import type { Reload } from "../load";
import { Banner } from "../ui/Banner";
import { Icon } from "../ui/Icon";
import { Inline } from "../ui/Inline";
import { toast } from "../ui/toast";
import { humanError } from "../ui/text";
import { Dialog, DialogActions } from "../ui/Dialog";
import { Button } from "../ui/Button";
import { Input } from "../ui/Input";
import { TextArea } from "../ui/TextArea";
import { MissionDrop } from "./MissionDrop";

// How a mission ends: landed (at Z, its code merged or landed), answered
// (a finding, no code), handed on (its work raised as other missions) or
// dropped. Whoever suggests the ending checks Z, each part met or not, and
// how: that's shown, and nothing about it is asked of you. A landed
// mission ws ends itself; any other you end in a press, or drop. A
// capsule can end alone: handed on to a mission of its own, or dropped.

// ZChecks is Z as it was checked: each part met or not, and how. A part
// nobody checked says so.
export function ZChecks({ c }: { c: Conclusion }) {
  // Z nobody checked at all isn't listed: there's nothing to say of it.
  if (!c.z?.some((z) => z.met || z.how)) return null;
  return (
    <ol className="z-checks">
      {c.z.map((z, i) => {
        const checked = z.met || !!z.how;
        return (
          <li key={i} className={z.met ? "met" : checked ? "unmet" : "unchecked"}>
            <Icon name={z.met ? "check-circle" : checked ? "x-circle" : "circle-dashed"} size={13} />
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
              {!checked && <span className="z-how dim"> — not checked</span>}
            </span>
          </li>
        );
      })}
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

// DropButton gives the mission up, from beside its state: it ends
// dropped, with why if you say. The one ending that's yours to begin:
// every other is suggested, or ws's own.
export function DropButton({ d, reload }: { d: MissionDetail; reload: Reload }) {
  const [busy, setBusy] = useState(false);
  if (d.ending || d.state === "archived") return null;
  const drop = (why: string) => {
    setBusy(true);
    act(d.slug, "end", { ending: "dropped", text: why })
      .then(
        (r) => ended(d.slug, r.message, reload),
        (e: Error) => toast(humanError(e.message), { tone: "danger" }),
      )
      .finally(() => setBusy(false));
  };
  return <MissionDrop busy={busy} onDrop={drop} />;
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
          {error && <Banner>{humanError(error)}</Banner>}
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
