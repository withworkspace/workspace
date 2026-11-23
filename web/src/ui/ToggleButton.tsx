import type { ReactNode, Ref } from "react";
import { ToggleButton as AriaToggleButton, type ToggleButtonProps as AriaToggleButtonProps } from "react-aria-components";
import { type ButtonLook, lookOf } from "./Button";
import { cx } from "./cx";
import css from "./ToggleButton.module.css";
import { Tooltip } from "./Tooltip";

// A toggle's look is a button's, but for a link's, or none.
export type ToggleLook = Omit<ButtonLook, "variant"> & { variant?: Exclude<ButtonLook["variant"], "link" | "bare"> };

type ToggleButtonProps = ToggleLook &
  Omit<AriaToggleButtonProps, "className" | "style" | "children" | "isDisabled" | "isSelected" | "defaultSelected" | "onChange" | "render" | "id"> & {
    pressed: boolean;
    onChange: (pressed: boolean) => void;
    className?: string;
    disabled?: boolean;
    title?: string;
    "aria-busy"?: boolean;
    children?: ReactNode;
    ref?: Ref<HTMLButtonElement>;
  };

// ToggleButton is something that's on or off, by itself: a panel shown, a
// filter on, bold. It's a Button that stays pressed: react-aria's toggle
// button, which says so to a screen reader (aria-pressed). Its words stay
// the same either way; a button whose words change with what pressing it
// does (Show past, Hide past) is a Button. One of a few is a Segmented.
//
// Pressed, a plain one or a ghost is in the accent, on its soft fill. A
// primary or a danger one is as it is: the page gives it that variant
// while it's on, or a class of its own.
export function ToggleButton({ pressed, onChange, variant, size, square, className, disabled, title, "aria-busy": busy, "aria-label": label, ...rest }: ToggleButtonProps) {
  const tinted = variant === undefined || variant === "ghost";
  const button = (
    <AriaToggleButton
      data-ui=""
      isSelected={pressed}
      onChange={onChange}
      isDisabled={disabled}
      aria-label={label ?? (square ? title : undefined)}
      className={cx(lookOf({ variant, size, square }), pressed && tinted && css.pressed, className)}
      render={(props) => <button {...props} title={disabled ? title : undefined} aria-busy={busy} />}
      {...rest}
    />
  );
  // Its title is a Tooltip, as a Button's is; disabled, the browser's.
  return title && !disabled ? <Tooltip text={title}>{button}</Tooltip> : button;
}
