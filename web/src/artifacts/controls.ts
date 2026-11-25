import type { Tweak, Value } from "./tweaks";

// What a tweak's control is given: the id its label is for, the tweak,
// what it's set to, and the way to set it.
export type ControlProps<V> = { id: string; t: Tweak; value: V; set: (v: Value) => void };

// short is a choice with few enough options, and short enough, to be a
// row of buttons: otherwise it's a dropdown.
export const short = (t: Tweak) => !!t.options && t.options.length <= 4 && t.options.reduce((n, o) => n + o.label.length, 0) <= 26;

// stepOf is a slider's step where the page gives none: whole numbers
// between whole ends, else a hundredth of the way.
export const stepOf = (t: Tweak, min: number, max: number) => t.step ?? (Number.isInteger(min) && Number.isInteger(max) ? 1 : (max - min) / 100);

// six is a colour as a colour box takes it: #rgb written out as #rrggbb.
export const six = (c: string) => (c.length === 4 ? `#${c[1]}${c[1]}${c[2]}${c[2]}${c[3]}${c[3]}` : c);
