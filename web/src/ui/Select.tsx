import type { ReactNode } from "react";
import { Select as AriaSelect, Button, ListBox, ListBoxItem, Popover, SelectValue } from "react-aria-components";
import { Icon } from "./Icon";

// Select is a dropdown, in place of the browser's own select: a button that
// says what's chosen, and a list of the choices under it, both drawn here
// in the page's own look. react-aria-components does the rest: keys
// (arrows, typing a name, Enter, Escape), focus, and what a screen reader
// says. The list opens in a layer of its own, over the page, so a
// scrolling panel can't cut it off.
//
// As with the browser's, a value is a string, one is always chosen, and
// onChange is called with the one picked when it's another.
export function Select({
  value,
  onChange,
  disabled,
  "aria-label": label,
  children,
}: {
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
  "aria-label": string;
  children: ReactNode; // Options
}) {
  // data-field: keys typed here are the Select's, not the page's (keys.ts).
  return (
    <AriaSelect className="select" value={value} onChange={(v) => v != null && onChange(String(v))} isDisabled={disabled} aria-label={label}>
      <Button className="select-button" data-field>
        <SelectValue className="select-value">{({ selectedText }) => selectedText}</SelectValue>
        <Icon name="chevron-down" size={14} className="select-chevron" />
      </Button>
      <Popover className="select-popover" offset={4} maxHeight={320} data-field>
        <ListBox className="select-list">{children}</ListBox>
      </Popover>
    </AriaSelect>
  );
}

// Option is one of a Select's choices: its value, and the words it shows
// (which typing a name finds it by). The chosen one is ticked.
export function Option({ value, children, title }: { value: string; children: string; title?: string }) {
  return (
    <ListBoxItem id={value} textValue={children} className="select-option">
      {({ isSelected }) => (
        <>
          <span className="select-option-text" title={title}>
            {children}
          </span>
          {isSelected && <Icon name="check" size={14} className="select-check" />}
        </>
      )}
    </ListBoxItem>
  );
}
