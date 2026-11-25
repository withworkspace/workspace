import type { Setting } from "../api";

// A control's set writes a value, done when it's written (or refused).
export type Set = (v: unknown) => Promise<void>;

// What a setting's control is given: the setting, the way to write it,
// and reset, which takes yours away, so the team's or the default applies
// again. stretch is for one in a table's cell, which it runs across.
export interface ControlProps {
  s: Setting;
  set: Set;
  reset: () => void;
  stretch?: boolean;
}
