import { useSaid } from "../../keymap/keymap";
import { verifyKeys } from "./keys";
import { cx } from "../../ui/cx";
import { Icon } from "../../ui/Icon";
import { Keys } from "../../ui/Keys";
import { ToggleButton } from "../../ui/ToggleButton";
import css from "./MissionVerifyYours.module.css";
import { RichTextArea } from "../../ui/RichTextArea";

type MissionVerifyYoursProps = {
  // The crew it goes back to.
  crew: string;
  // It's a check left for you: yours to approve. One that's settled is
  // only looked at, to keep your place.
  left?: boolean;
  // You've approved it, or looked at it.
  looked?: boolean;
  busy?: boolean;
  onLook: () => void;
  // What you've said isn't right of it; undefined when you haven't. Left
  // out (no onNote), there's nothing to say isn't right: ws ran it.
  note?: string;
  onNote?: (text?: string) => void;
};

// MissionVerifyYours is what's yours of a check while its bundle is yours
// to settle: Approve check (Looked at, of one that's settled) and Not
// right, with what you say isn't. It's kept at the pane's foot as the
// check scrolls, so a long one needn't be read to its end before you can
// say.
export function MissionVerifyYours({ crew, left, looked, busy, onLook, note, onNote }: MissionVerifyYoursProps) {
  const approveKey = useSaid(verifyKeys.approve);
  const rejectKey = useSaid(verifyKeys.reject);
  return (
    <div className={css.yours}>
      {onNote && note !== undefined && (
        <label className={css.note}>
          <span>What's not right? Goes back to {crew} with this check.</span>
          <RichTextArea marks lists quote code refs look="field" autoFocus rows={2} value={note} onChange={onNote} placeholder="Say what you saw" aria-label="What's not right" />
        </label>
      )}
      <div className={css.buttons}>
        <ToggleButton
          className={looked ? css.on : undefined}
          pressed={!!looked}
          disabled={busy}
          onChange={onLook}
          title={left ? "Mark as fine" : "Mark as looked at"}
          keys={verifyKeys.approve}
        >
          <Icon name="check" size={14} /> {left ? "Approve check" : "Looked at"} {approveKey && <Keys k={approveKey} />}
        </ToggleButton>
        {onNote && (
          <ToggleButton
            variant={note !== undefined ? "danger" : undefined}
            className={note !== undefined ? cx(css.on, css.danger) : undefined}
            pressed={note !== undefined}
            onChange={(on) => onNote(on ? "" : undefined)}
            title="Say what's not right"
            keys={verifyKeys.reject}
          >
            <Icon name="x" size={14} /> Not right {rejectKey && <Keys k={rejectKey} />}
          </ToggleButton>
        )}
      </div>
    </div>
  );
}
