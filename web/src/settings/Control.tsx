import type { Setting } from "../api";
import { Switch } from "../ui/Switch";
import { AccentInput } from "./AccentInput";
import { Choice } from "./Choice";
import type { ControlProps } from "./controlProps";
import { Few } from "./Few";
import { ModelInput } from "./ModelInput";
import { Picks } from "./Picks";
import { Stepper } from "./Stepper";
import { TextInput } from "./TextInput";
import { TintInput } from "./TintInput";

// A setting's control, by its kind: what it's set to, and the way to set
// it. Each writes as soon as it's changed (a switch flipped, a field
// left), so there's nothing to save. None waits for another's write: the
// page makes them in turn.
export function Control(p: ControlProps) {
  const { s, set } = p;
  switch (s.kind) {
    case "harness":
      return <Choice {...p} />;
    case "choice":
      return few(s) ? <Few {...p} /> : <Choice {...p} />;
    case "bool":
      return <Switch on={s.value === true} onChange={(on) => void set(on)} aria-label={s.label} />;
    case "model":
      return <ModelInput {...p} />;
    case "picks":
      return <Picks {...p} />;
    case "number":
      return <Stepper {...p} />;
    case "color":
      // The one colour setting is the accent, which the page wears.
      return s.section === "ui" && s.key === "accent" ? <AccentInput {...p} /> : <TextInput {...p} />;
    case "tint":
      return <TintInput {...p} />;
    default:
      return <TextInput {...p} short />;
  }
}

// few is a choice with so few, so briefly said, that they're all on show.
const few = (s: Setting) => !!s.choices && s.choices.length > 1 && s.choices.length <= 3 && s.choices.every((c) => (c.label ?? c.value).length <= 14);
