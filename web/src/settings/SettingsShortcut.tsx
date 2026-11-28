import { Button } from "../ui/Button";
import { Icon } from "../ui/Icon";
import { Keys } from "../ui/Keys";
import css from "./SettingsShortcut.module.css";
import { SettingsShortcutLines } from "./SettingsShortcutLines";

export type SettingsShortcutProps = {
  // What the command does.
  title: string;
  // Its keys now, the first the one that's said.
  keys: readonly string[];
  // You've set its keys (else they're its own).
  yours: boolean;
  // While a key's being set for it: what's been pressed so far ([] is
  // nothing yet).
  setting?: readonly string[];
  // What else its keys do, where that's yours: "j also does Open (Team)".
  // Notes are what's so but no trouble: the pages where a plugin's key is
  // the page's own.
  clashes?: readonly string[];
  notes?: readonly string[];
  // Keys it's suggested and hasn't (a plugin's), with what taking them
  // would clash with, to see before Use.
  suggested?: readonly string[];
  suggestedClashes?: readonly string[];
  suggestedNotes?: readonly string[];
  onUseSuggested?: () => void;
  onAdd: () => void;
  onRemove: (key: string) => void;
  onReset: () => void;
  onCancel: () => void;
};

// SettingsShortcut is a command in Settings › Keyboard: what it does, its
// keys (each taken off with its ×), a key added by pressing it, and its
// own keys back where you've set others. What its keys clash with is said
// under it, and so are keys it's suggested (a plugin's), with Use to take
// them, and what they'd clash with.
export function SettingsShortcut({ title, keys, yours, setting, clashes, notes, suggested, suggestedClashes, suggestedNotes, onUseSuggested, onAdd, onRemove, onReset, onCancel }: SettingsShortcutProps) {
  return (
    <div className={css.shortcut} data-yours={yours || undefined}>
      <div className={css.title}>{title}</div>
      <div className={css.keys}>
        {keys.map((k) => (
          <span key={k} className={css.key}>
            <Keys k={k} />
            <Button variant="bare" className={css.remove} onClick={() => onRemove(k)} aria-label={`Remove ${k}`}>
              <Icon name="x" />
            </Button>
          </span>
        ))}
        {keys.length === 0 && !setting && <span className={css.none}>No key</span>}
        {setting ? (
          <span className={css.setting} role="status">
            {setting.length > 0 ? <Keys k={setting.join(" ")} /> : "Press a key"}
            <span className={css.then}>{setting.length > 0 ? "…or another after it" : "…or two, one after the other"}</span>
            <Button variant="link" onClick={onCancel}>
              Cancel
            </Button>
          </span>
        ) : (
          <Button variant="ghost" size="small" square title="Add a key" onClick={onAdd}>
            <Icon name="plus" />
          </Button>
        )}
        {yours && !setting && (
          <Button variant="link" onClick={onReset}>
            Reset
          </Button>
        )}
      </div>
      {suggested && suggested.length > 0 && !setting && (
        <div className={css.suggested}>
          Suggested{" "}
          {suggested.map((k) => (
            <Keys key={k} k={k} />
          ))}
          <Button variant="link" onClick={onUseSuggested}>
            Use
          </Button>
        </div>
      )}
      <SettingsShortcutLines clashes={[...(clashes ?? []), ...(!setting ? (suggestedClashes ?? []) : [])]} notes={[...(notes ?? []), ...(!setting ? (suggestedNotes ?? []) : [])]} />
    </div>
  );
}
