import { Banner } from "../../ui/Banner";
import { Button } from "../../ui/Button";
import { Dialog, DialogActions } from "../../ui/Dialog";
import { Input } from "../../ui/Input";
import { humanError } from "../../ui/strings";
import { TextArea } from "../../ui/TextArea";
import css from "./MissionEndingDialog.module.css";

type MissionEndingDialogProps = {
  // Dropping the capsule, or handing it on.
  ending: "drop" | "hand-on";
  // The capsule, repo/name.
  name: string;
  // Why it's dropped, or what the mission it goes to is for.
  text: string;
  onText: (text: string) => void;
  // The mission it's handed on to, if it's one there is (its slug).
  to: string;
  onTo: (to: string) => void;
  error?: string;
  busy?: boolean;
  onClose: () => void;
  onEnd: () => void;
};

// MissionEndingDialog ends a capsule alone: dropped, its work no longer
// the mission's, or handed on to a mission of its own (raised for it from
// what's said, or one there is), which takes it.
export function MissionEndingDialog({ ending, name, text, onText, to, onTo, error, busy, onClose, onEnd }: MissionEndingDialogProps) {
  const drop = ending === "drop";
  return (
    <Dialog title={drop ? `Drop ${name}` : `Hand ${name} on`} onClose={onClose} className={css.dialog} wide>
      <p>
        {drop
          ? "Work won't be part of the mission, and won't hold it open. The capsule is kept, outside any mission."
          : "Describe a new mission for it, or name an existing one. The capsule moves there and no longer holds this mission open."}
      </p>
      <TextArea
        value={text}
        rows={3}
        autoFocus
        placeholder={drop ? "Reason for dropping" : "What the new mission is for"}
        aria-label={drop ? "Reason" : "New mission"}
        onChange={(e) => onText(e.target.value)}
      />
      {!drop && (
        <label className={css.to}>
          <span>Or an existing mission</span>
          <Input value={to} onChange={(e) => onTo(e.target.value)} placeholder="Mission slug" />
        </label>
      )}
      {error && <Banner>{humanError(error)}</Banner>}
      <DialogActions>
        <Button variant="ghost" onClick={onClose}>
          Cancel
        </Button>
        <Button variant={drop ? "danger" : "primary"} onClick={onEnd} disabled={busy || (!text.trim() && !(!drop && to.trim()))}>
          {drop ? "Drop" : "Hand on"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
