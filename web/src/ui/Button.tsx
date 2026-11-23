import type { ReactNode, Ref } from "react";
import { Button as AriaButton, type ButtonProps as AriaButtonProps } from "react-aria-components";
import css from "./Button.module.css";
import { cx } from "./cx";

// How a button looks: its variant (a plain button without one), small or
// not, and square, for one that's only an icon.
export type ButtonLook = {
  variant?: "primary" | "danger" | "ghost" | "link";
  size?: "small";
  square?: boolean;
};

// What a button takes: its look, and what react-aria's takes, said as the
// browser says it (disabled, not isDisabled; a className that's a string).
// title and aria-busy are the browser's too: react-aria's button drops
// both, so they're put on the element here.
type ButtonProps = ButtonLook &
  Omit<AriaButtonProps, "className" | "style" | "children" | "isDisabled" | "render"> & {
    className?: string;
    disabled?: boolean;
    title?: string;
    "aria-busy"?: boolean;
    children?: ReactNode;
    ref?: Ref<HTMLButtonElement>;
  };

// Button is a button: react-aria's, in ws's look. So it's pressed the same
// by a mouse, a finger and the keys, and it's what a menu, a popover or a
// tooltip opens from (inside a MenuTrigger it's the trigger, with nothing
// more said). Its type is button unless it's given one: a form's own says
// type="submit". data-ui says it's the library's: styles.css's rules for a
// page's own buttons' states leave it to its module.
export function Button({ variant, size, square, className, disabled, title, "aria-busy": busy, ...rest }: ButtonProps) {
  return (
    <AriaButton
      data-ui=""
      isDisabled={disabled}
      className={cx(lookOf({ variant, size, square }), className)}
      render={(props) => <button {...props} title={title} aria-busy={busy} />}
      {...rest}
    />
  );
}

// buttonClass is a button's look for a link: a <Link> or an <a> that's an
// action, drawn as the button beside it is.
export function buttonClass(look: ButtonLook = {}, className?: string): string {
  return cx(lookOf(look), css.anchor, className);
}

const variants = { primary: css.primary, danger: css.danger, ghost: css.ghost };
const sizes = { small: css.small };

// A link's look is its own: none of a button's box to undo. (The
// library's other buttons, ToggleButton's too, are drawn with it.)
export function lookOf({ variant, size, square }: ButtonLook): string {
  if (variant === "link") return css.link;
  return cx(css.button, variant && variants[variant], square && css.square, size && sizes[size]);
}
