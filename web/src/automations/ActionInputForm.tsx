import type { AutomationInputField } from "../api";
import { Checkbox } from "../ui/Checkbox";
import { Input } from "../ui/Input";
import { Option, Select } from "../ui/Select";
import css from "./ActionInputForm.module.css";

// ActionInputForm is what an automation's action asks before it runs, a
// field each: words or a number typed, a yes ticked, one of its words
// picked. values are as given so far, by field; Enter in a box is go.
export function ActionInputForm({
  fields,
  values,
  onChange,
  onGo,
}: {
  fields: AutomationInputField[];
  values: Record<string, unknown>;
  onChange: (name: string, value: unknown) => void;
  onGo: () => void;
}) {
  return (
    <div className={css.form}>
      {fields.map((f, i) => {
        const words = f.name.replace(/_/g, " ");
        const label = words.charAt(0).toUpperCase() + words.slice(1);
        if (f.type === "bool")
          return (
            <Checkbox key={f.name} className={css.tick} checked={values[f.name] === true} onChange={(on) => onChange(f.name, on)}>
              {label}
            </Checkbox>
          );
        return (
          <label key={f.name} className={css.field}>
            <span className={css.label}>{label}</span>
            {f.type === "choice" ? (
              <Select fill value={String(values[f.name] ?? f.choices?.[0] ?? "")} onChange={(v) => onChange(f.name, v)} aria-label={label}>
                {(f.choices ?? []).map((c) => (
                  <Option key={c} value={c}>
                    {c}
                  </Option>
                ))}
              </Select>
            ) : (
              <Input
                autoFocus={i === 0}
                inputMode={f.type === "text" ? undefined : "decimal"}
                value={String(values[f.name] ?? "")}
                onChange={(e) => onChange(f.name, e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") {
                    e.preventDefault();
                    onGo();
                  }
                }}
              />
            )}
          </label>
        );
      })}
    </div>
  );
}
