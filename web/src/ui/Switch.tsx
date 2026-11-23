import { Switch as AriaSwitch } from "react-aria-components";
import { cx } from "./cx";
import css from "./Switch.module.css";

// Switch is something that's on or off, changed at once: a setting, not a
// box ticked before a form is sent. It's react-aria's switch, which says
// which it is to a screen reader (role switch), and is worked by a press
// or by Space.
export function Switch({ on, onChange, disabled, className, "aria-label": label }: { on: boolean; onChange: (on: boolean) => void; disabled?: boolean; className?: string; "aria-label": string }) {
  return <AriaSwitch className={cx(css.switch, className)} isSelected={on} onChange={onChange} isDisabled={disabled} aria-label={label} />;
}
