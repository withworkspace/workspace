import type { Ref } from "react";
import { Button } from "../ui/Button";
import { Input } from "../ui/Input";
import { SettingsShortcut, type SettingsShortcutProps } from "./SettingsShortcut";
import css from "./SettingsShortcuts.module.css";
import { SettingsUnder } from "./SettingsUnder";

export type SettingsShortcutsGroup = {
  // The scope's name: Everywhere, Team, Picture viewer.
  title: string;
  shortcuts: (SettingsShortcutProps & { id: string })[];
};

type SettingsShortcutsProps = {
  groups: SettingsShortcutsGroup[];
  // What's typed to find a command, by what it does or its key.
  filter: string;
  onFilter: (to: string) => void;
  filterRef?: Ref<HTMLInputElement>;
  // How many you've set (none: there's nothing to reset, and no Reset all).
  yours: number;
  onResetAll: () => void;
};

// SettingsShortcuts is every command a key does, in Settings › Keyboard:
// under a heading, a box to find one, and a group for each scope
// (Everywhere first), each command's keys yours to change.
export function SettingsShortcuts({ groups, filter, onFilter, filterRef, yours, onResetAll }: SettingsShortcutsProps) {
  return (
    <section aria-label="Shortcuts">
      <SettingsUnder title="Shortcuts" about="Kept in this browser. Click + and press a key, or two one after the other, to add one; esc to give up." />
      <div className={css.bar}>
        <Input ref={filterRef} className={css.filter} value={filter} onChange={(e) => onFilter(e.target.value)} placeholder="Find a shortcut by what it does or its key" aria-label="Find a shortcut" />
        {yours > 0 && (
          <Button variant="ghost" size="small" onClick={onResetAll}>
            Reset all
          </Button>
        )}
      </div>
      {groups.map((g) => (
        <div key={g.title} className={css.group}>
          <h3 className={css.title}>{g.title}</h3>
          {g.shortcuts.map(({ id, ...s }) => (
            <SettingsShortcut key={id} {...s} />
          ))}
        </div>
      ))}
      {groups.length === 0 && <p className={css.none}>No shortcut does that.</p>}
    </section>
  );
}
