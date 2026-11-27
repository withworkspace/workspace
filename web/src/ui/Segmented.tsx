import { type KeyLike, useSaid } from "../keymap/keymap";
import { createContext, type ReactNode, useContext } from "react";
import { ToggleButton, ToggleButtonGroup } from "react-aria-components";
import { cx } from "./cx";
import { keyHint, pointed, titleWithKeys } from "./keyHint";
import css from "./Segmented.module.css";
import { Tooltip } from "./Tooltip";

// Whether the whole control is disabled, for each segment to know.
const Disabled = createContext(false);

// How a segmented control looks: small, in a head or a line of words, and
// fill, where it's as wide as what it's in and its segments share that.
export type SegmentedLook = { size?: "small"; fill?: boolean };

// Segmented is one of a few, chosen at once and always one of them: how a
// diff is laid out, a width, a theme. It's a track with a segment for
// each, the chosen one raised. react-aria's toggle group does the rest:
// it's one stop for Tab, the arrows move along it, and a screen reader
// hears a group of radios. onChange is called with the one picked when
// it's another.
export function Segmented<T extends string>({
  value,
  onChange,
  disabled,
  size,
  fill,
  className,
  id,
  "aria-label": label,
  children,
}: SegmentedLook & {
  value: T;
  onChange: (value: T) => void;
  disabled?: boolean;
  className?: string;
  id?: string;
  "aria-label": string;
  children: ReactNode; // Segments
}) {
  return (
    <ToggleButtonGroup
      render={(props) => <div {...props} id={id} />}
      className={segmentedClass({ size, fill }, className)}
      selectionMode="single"
      disallowEmptySelection
      selectedKeys={[value]}
      onSelectionChange={(keys) => {
        const [picked] = keys;
        if (picked != null && picked !== value) onChange(String(picked) as T);
      }}
      isDisabled={disabled}
      aria-label={label}
    >
      <Disabled.Provider value={!!disabled}>{children}</Disabled.Provider>
    </ToggleButtonGroup>
  );
}

// Segment is one of a Segmented's choices: its value, and what it shows.
// A title says more about it, in a tooltip. keys is the key that picks
// it (or goes on to the next, as one key often does for all of them),
// said and hinted as a Button's is.
export function Segment({ value, disabled: own, title, keys: given, className, children }: { value: string; disabled?: boolean; title?: string; keys?: KeyLike; className?: string; children: ReactNode }) {
  const all = useContext(Disabled);
  const disabled = own || all;
  const keys = useSaid(given);
  const titled = titleWithKeys(title, keys);
  const segment = (
    <ToggleButton
      id={value}
      data-ui=""
      isDisabled={own}
      className={cx(css.segment, className)}
      onPress={keys ? (e) => pointed(e) && keyHint(keys) : undefined}
      render={(props) => <button {...props} title={disabled ? titled : undefined} />}
    >
      {children}
    </ToggleButton>
  );
  // Its title is a Tooltip, as a Button's is; disabled, the browser's.
  return titled && !disabled ? <Tooltip text={titled}>{segment}</Tooltip> : segment;
}

// segmentedClass and segmentClass are a segmented control's look for
// what can't be one: a track whose segments are something else's to draw
// (each inside a hover card, say). The chosen segment says data-selected.
export function segmentedClass(look: SegmentedLook = {}, className?: string): string {
  return cx(css.segmented, look.size === "small" && css.small, look.fill && css.fill, className);
}

export function segmentClass(className?: string): string {
  return cx(css.segment, className);
}
