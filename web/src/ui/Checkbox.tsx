import type { MouseEvent, ReactNode } from "react";
import { Checkbox as AriaCheckbox } from "react-aria-components";
import css from "./Checkbox.module.css";
import { cx } from "./cx";

// Checkbox is a box that's ticked or not, with its words beside it:
// something chosen before it's sent, or a thing kept on (a Switch is a
// setting changed at once). It's react-aria's checkbox: the whole of it
// is pressed, words too, and Space ticks it.
//
// The words are its children; one with none says what it is by
// aria-label. A page lays it out by a class it passes.
export function Checkbox({
  checked,
  onChange,
  disabled,
  title,
  className,
  onClick,
  "aria-label": label,
  children,
}: {
  checked: boolean;
  onChange: (checked: boolean) => void;
  disabled?: boolean;
  title?: string;
  className?: string;
  onClick?: (e: MouseEvent<HTMLLabelElement>) => void;
  "aria-label"?: string;
  children?: ReactNode;
}) {
  return (
    <AriaCheckbox
      className={cx(css.checkbox, className)}
      isSelected={checked}
      onChange={onChange}
      isDisabled={disabled}
      aria-label={label}
      render={(props) => (
        <label
          {...props}
          title={title}
          onClick={(e) => {
            props.onClick?.(e);
            onClick?.(e);
          }}
        />
      )}
    >
      <span className={css.box} aria-hidden="true">
        <svg viewBox="0 0 12 12" className={css.tick}>
          <path d="M2.5 6.5 5 9l4.5-5.5" />
        </svg>
      </span>
      {children}
    </AriaCheckbox>
  );
}
