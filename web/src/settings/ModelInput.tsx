import { useState } from "react";
import { useLoaded } from "../load";
import { Option } from "../ui/Select";
import type { ControlProps } from "./controlProps";
import { modelsOf } from "./models";
import { shown } from "./sections";
import { SettingsNote } from "./SettingsNote";
import { SettingsSelect } from "./SettingsSelect";
import { TextInput } from "./TextInput";

const own = "\u0000own";
const another = "\u0000another";

// ModelInput is a role's model: one of those its harness lists, or
// another, typed by name (a harness that lists none has the field alone).
export function ModelInput(p: ControlProps) {
  const { s, set, stretch } = p;
  const harness = s.harness;
  // Another harness's list isn't kept while this one's is asked for: till
  // it's come there's what's set, and another by name.
  const got = useLoaded(harness ? () => modelsOf(harness) : null, [harness]);
  const models = got.value ?? [];
  const asking = !!harness && got.value === undefined && !got.error;
  const [typing, setTyping] = useState(false);
  const cur = shown(s.value);
  const def = shown(s.default);
  const placeholder = def ? `${def} (its default)` : "the agent's own default";
  const note = got.error && (
    <SettingsNote left={stretch}>
      Couldn't list {harness}'s models: {got.error}
    </SettingsNote>
  );
  if (typing || (models.length === 0 && !asking)) {
    return (
      <>
        <TextInput {...p} placeholder={typing ? "in that agent's own naming" : placeholder} blank={typing} done={() => setTyping(false)} />
        {note}
      </>
    );
  }
  return (
    <SettingsSelect
      stretch={stretch}
      value={cur || own}
      aria-label={s.label}
      onChange={(v) => {
        if (v === another) setTyping(true);
        else if (v !== own) void set(v);
      }}
    >
      {!cur && <Option value={own}>The agent's own default</Option>}
      {cur && !models.some((m) => m.id === cur) && <Option value={cur}>{cur}</Option>}
      {models.map((m) => (
        <Option key={m.id} value={m.id} note={[m.about ?? (m.name !== m.id ? m.name : ""), m.id === def ? "its default" : ""].filter(Boolean).join(" · ")}>
          {m.id}
        </Option>
      ))}
      <Option value={another}>Another, by name…</Option>
    </SettingsSelect>
  );
}
