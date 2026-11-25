import type { ControlProps } from "./controls";
import type { Value } from "./tweaks";
import { TweakChoose } from "./TweakChoose";
import { TweakColour } from "./TweakColour";
import { TweakNumber } from "./TweakNumber";
import { TweakToggle } from "./TweakToggle";
import { TweakWords } from "./TweakWords";

// TweakControl is a tweak's control, by its kind.
export function TweakControl({ id, t, value, set }: ControlProps<Value>) {
  switch (t.kind) {
    case "toggle":
      return <TweakToggle id={id} t={t} value={value === true} set={set} />;
    case "choice":
      return <TweakChoose id={id} t={t} value={String(value)} set={set} />;
    case "number":
      return <TweakNumber id={id} t={t} value={Number(value)} set={set} />;
    case "color":
      return <TweakColour id={id} t={t} value={String(value)} set={set} />;
    case "text":
      return <TweakWords id={id} t={t} value={String(value)} set={set} />;
  }
}
