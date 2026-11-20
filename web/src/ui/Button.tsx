import css from "./Button.module.css";
import { cx } from "./cx";

// How a button looks: its variant (a plain button without one), small or
// not, and square, for one that's only an icon.
export type ButtonLook = {
  variant?: "primary" | "danger" | "ghost" | "link";
  size?: "small";
  square?: boolean;
};

type ButtonProps = ButtonLook & React.ComponentProps<"button">;

// Button is a button. Its type is the browser's (submit, in a form) unless
// it's given one. data-ui says it's the library's: styles.css's rules for a
// page's own buttons' states leave it to its module.
export function Button({ variant, size, square, className, ...rest }: ButtonProps) {
  return <button data-ui="" className={cx(lookOf({ variant, size, square }), className)} {...rest} />;
}

// buttonClass is a button's look for a link: a <Link> or an <a> that's an
// action, drawn as the button beside it is.
export function buttonClass(look: ButtonLook = {}, className?: string): string {
  return cx(lookOf(look), css.anchor, className);
}

const variants = { primary: css.primary, danger: css.danger, ghost: css.ghost, link: css.link };
const sizes = { small: css.small };

function lookOf({ variant, size, square }: ButtonLook): string {
  return cx(css.button, variant && variants[variant], square && css.square, size && sizes[size]);
}
