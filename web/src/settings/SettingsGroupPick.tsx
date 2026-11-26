import { useEffect, useRef, useState } from "react";
import { cx } from "../ui/cx";
import { Option, Select } from "../ui/Select";
import css from "./SettingsGroupPick.module.css";
import { Input } from "../ui/Input";
import { textClass } from "../ui/Text";

const NONE = "\u0000none";
const NEW = "\u0000new";

// SettingsGroupPick is which group a repo's in, picked: one the workspace has, a
// new one named, or none, where it can be in none (a repo of the team's
// in a group of the team's can't be, for you alone). The team's group is
// said as the team's. Groups are offered in the order given, by their
// labels (display names) where they have them.
export function SettingsGroupPick({ value, groups, labels, team, none, label, onPick, disabled }: { value: string; groups: string[]; labels?: Map<string, string>; team?: string; none: boolean; label: string; onPick: (group: string) => void; disabled?: boolean }) {
  const [naming, setNaming] = useState(false);
  const [name, setName] = useState("");
  const field = useRef<HTMLInputElement>(null);
  // It comes in place of the list it was picked from, which holds the
  // keys until it's closed: focused once it has.
  useEffect(() => {
    if (!naming) return;
    const frame = requestAnimationFrame(() => field.current?.focus());
    return () => cancelAnimationFrame(frame);
  }, [naming]);
  if (naming) {
    const done = (keep: boolean) => {
      setNaming(false);
      const n = name.trim();
      setName("");
      if (keep && n && n !== value) onPick(n);
    };
    const bad = /[\s,]/.test(name.trim());
    return (
      <Input
        ref={field}
        className={cx(textClass("mono"), css.naming)}
        aria-label={`${label}: new group`}
        placeholder="new group"
        value={name}
        aria-invalid={bad}
        title={bad ? "No spaces or commas" : undefined}
        onChange={(e) => setName(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter" && !bad) done(true);
          if (e.key === "Escape") {
            e.stopPropagation();
            done(false);
          }
        }}
        onBlur={() => done(!bad)}
      />
    );
  }
  const all = [...new Set([...groups, ...[value, team].filter((g): g is string => !!g && !groups.includes(g)).sort()])];
  return (
    <Select
      aria-label={label}
      value={value || NONE}
      disabled={disabled}
      onChange={(v) => {
        if (v === NEW) setNaming(true);
        else if (v === NONE) onPick("");
        else if (v !== value) onPick(v);
      }}
    >
      {[
        ...(none ? [<Option key={NONE} value={NONE}>No group</Option>] : []),
        ...all.map((g) => (
          <Option key={g} value={g} note={g === team ? "the team's" : undefined}>
            {labels?.get(g) ?? g}
          </Option>
        )),
        <Option key={NEW} value={NEW}>
          New group…
        </Option>,
      ]}
    </Select>
  );
}
