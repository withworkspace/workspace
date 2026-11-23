import type { ReactNode } from "react";
import { Select as AriaSelect, Button, ListBox, ListBoxItem, SelectValue } from "react-aria-components";
import { cx } from "./cx";
import { Icon } from "./Icon";
import { PopoverSurface } from "./Popover";
import css from "./Select.module.css";

// Select is a dropdown, in place of the browser's own select: a button that
// says what's chosen, and a list of the choices under it, both drawn here
// in the page's own look. react-aria-components does the rest: keys
// (arrows, typing a name, Enter, Escape), focus, and what a screen reader
// says. The list opens in a layer of its own, over the page, so a
// scrolling panel can't cut it off.
//
// As with the browser's, a value is a string, one is always chosen, and
// onChange is called with the one picked when it's another.
//
// It's the size of a text box; small is a button's, for a row of
// controls, and tiny sits in a line of words. fill is as wide as what
// it's in. A page that sizes it otherwise gives it a class.
export function Select({
  value,
  onChange,
  disabled,
  size,
  fill,
  className,
  "aria-label": label,
  children,
}: {
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
  size?: "small" | "tiny";
  fill?: boolean;
  className?: string;
  "aria-label": string;
  children: ReactNode; // Options
}) {
  // data-field: keys typed here are the Select's, not the page's (keys.ts).
  // data-select: a dialog doesn't start its focus on one (kit.tsx).
  return (
    <AriaSelect className={cx(css.select, size === "small" && css.small, size === "tiny" && css.tiny, fill && css.fill, className)} value={value} onChange={(v) => v != null && onChange(String(v))} isDisabled={disabled} aria-label={label}>
      <Button className={css.button} data-ui="" data-field data-select="">
        <SelectValue className={css.value}>{({ selectedText }) => selectedText}</SelectValue>
        <Icon name="chevron-down" size={14} className={css.chevron} />
      </Button>
      <PopoverSurface maxHeight={320}>
        <ListBox className={css.list}>{children}</ListBox>
      </PopoverSurface>
    </AriaSelect>
  );
}

// Option is one of a Select's choices: its value, and the words it shows
// (which typing a name finds it by, and which the button says once it's
// chosen). The chosen one is ticked. A note is more about it, said quietly
// after its words in the list alone.
export function Option({ value, children, title, note }: { value: string; children: string; title?: string; note?: string }) {
  return (
    <ListBoxItem id={value} textValue={children} className={css.option}>
      {({ isSelected }) => (
        <>
          <span className={css.optionText} title={title}>
            {children}
            {note && <span className={css.note}>{note}</span>}
          </span>
          {isSelected && <Icon name="check" size={14} className={css.check} />}
        </>
      )}
    </ListBoxItem>
  );
}
