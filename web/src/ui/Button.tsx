import type { CSSProperties, ReactNode, Ref } from "react";
import { Button as AriaButton, type ButtonProps as AriaButtonProps } from "react-aria-components";
import css from "./Button.module.css";
import { cx } from "./cx";
import { toast } from "./toast";
import { Tooltip } from "./Tooltip";

// How a button looks: its variant (a plain button without one), small or
// not, and square, for one that's only an icon. A bare one has none of
// the library's look: it's a part's own button, drawn by the class that
// part gives it (the workspace's name, a grip), and a Button for what a
// Button does.
export type ButtonLook = {
  variant?: "primary" | "danger" | "ghost" | "link" | "bare";
  size?: "small";
  square?: boolean;
};

// What a button takes: its look, and what react-aria's takes, said as the
// browser says it (disabled, not isDisabled; a className that's a string).
// aria-busy is the browser's too: react-aria's button drops it, so it's
// put on the element here.
type ButtonProps = ButtonLook &
  Omit<AriaButtonProps, "className" | "style" | "children" | "isDisabled" | "render"> & {
    className?: string;
    style?: CSSProperties;
    disabled?: boolean;
    // Why it can't be used now, where that's worth saying: see below.
    unavailable?: string;
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
//
// Its title is a Tooltip: what it does, and its key. One that's square
// (an icon alone) is named by it, unless it has an aria-label.
//
// A button that can't be used now is one of two things. disabled: it's
// out of reach, to the pointer and the keys, and says nothing (while
// something's being sent, say). Its title, if it has one, is the
// browser's: a tooltip isn't shown on what can't be pointed at.
// unavailable="why": it looks disabled, but can still be pointed at and
// focused, so its tooltip says why; pressing it does nothing but say why
// again, in a toast, for whoever pressed before reading. Disabled as
// well, it's disabled.
export function Button({ variant, size, square, className, disabled, unavailable, title, "aria-busy": busy, "aria-label": label, type, onClick, onPress, ...rest }: ButtonProps) {
  const why = disabled ? undefined : unavailable || undefined;
  const says = why ?? (disabled ? undefined : title);
  const button = (
    <AriaButton
      data-ui=""
      isDisabled={disabled}
      aria-label={label ?? (square ? title : undefined)}
      className={cx(lookOf({ variant, size, square }), why && css.unavailable, className)}
      type={why ? "button" : type}
      onClick={why ? () => toast(why) : onClick}
      onPress={why ? undefined : onPress}
      render={(props) => <button {...props} title={disabled ? title : undefined} aria-busy={busy} aria-disabled={why ? true : undefined} />}
      {...rest}
    />
  );
  return says ? <Tooltip text={says}>{button}</Tooltip> : button;
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
  if (variant === "bare") return "";
  if (variant === "link") return css.link;
  return cx(css.button, variant && variants[variant], square && css.square, size && sizes[size]);
}
