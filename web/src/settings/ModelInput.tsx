import { useState } from "react";
import { useLoaded } from "../load";
import type { ControlProps } from "./controlProps";
import { modelsOf } from "../models/models";
import { shown } from "./sections";
import { SettingsNote } from "./SettingsNote";
import { ModelPicker } from "../models/ModelPicker";
import { TextInput } from "./TextInput";

// ModelInput is a role's model: one of those its harness lists, picked
// from them as a tree (a model, then its effort, thinking, fast), or
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
    <>
      <ModelPicker
        stretch={stretch}
        models={models}
        value={cur}
        placeholder={placeholder}
        busy={asking}
        aria-label={s.label}
        onChange={(v) => void set(v)}
        onAnother={() => setTyping(true)}
      />
      {note}
    </>
  );
}
