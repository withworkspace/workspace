import type { ComponentProps, ReactNode } from "react";
import { ListBox as AriaListBox, ListBoxItem } from "react-aria-components";
import { cx } from "./cx";
import css from "./ListBox.module.css";

// ListBox is a list to pick one of, each a row of its own: the repos to
// add, say. value is the one picked (or none yet), and onChange is called
// with another when it's picked. react-aria's list box does the rest:
// it's one stop for Tab, the arrows move along it, Enter or Space picks,
// typing a name goes to it, and a screen reader hears a list of options.
// empty is what it shows with nothing in it.
//
// A list that goes somewhere when a row is chosen (a page's list of
// links) is Rows; one driven from a text box is a finder's.
export function ListBox<T extends string>({
  value,
  onChange,
  className,
  empty,
  "aria-label": label,
  children,
}: {
  value: T | undefined;
  onChange: (value: T) => void;
  className?: string;
  empty?: ReactNode;
  "aria-label": string;
  children: ReactNode; // ListBoxOptions
}) {
  return (
    <AriaListBox
      className={cx(css.list, className)}
      aria-label={label}
      selectionMode="single"
      disallowEmptySelection
      selectedKeys={value === undefined ? [] : [value]}
      onSelectionChange={(keys) => {
        if (keys === "all") return;
        const [picked] = keys;
        if (picked != null && picked !== value) onChange(String(picked) as T);
      }}
      renderEmptyState={empty === undefined ? undefined : () => empty}
    >
      {children}
    </AriaListBox>
  );
}

// ListBoxOption is one of them: its value, the words typing finds it by
// (textValue), and what it shows. A disabled one can't be picked; a title
// says more about it on hover.
export function ListBoxOption({ value, textValue, disabled, title, className, children }: { value: string; textValue: string; disabled?: boolean; title?: string; className?: string; children: ReactNode }) {
  return (
    <ListBoxItem id={value} textValue={textValue} isDisabled={disabled} className={cx(css.option, className)} render={(props) => <div {...(props as ComponentProps<"div">)} title={title} />}>
      {children}
    </ListBoxItem>
  );
}
