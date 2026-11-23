import type { ComponentProps } from "react";
import { Input as AriaInput } from "react-aria-components";
import { cx } from "./cx";
import css from "./Input.module.css";

// Input is a line of text to type: a text box, drawn here. It takes what
// the browser's input takes (value, onChange, placeholder, a ref, keys),
// and is react-aria's input, which also says when it's hovered, focused
// or invalid (data-hovered, data-focused, data-invalid) for a rule to
// read. A box that's ticked is a Checkbox; more than a line is a
// TextArea.
//
// A page that draws one otherwise (no box, in a bar of its own) gives it
// a class, or styles the input in its bar: a page's rule outweighs these.
export function Input({ className, ...rest }: Omit<ComponentProps<typeof AriaInput>, "className" | "style"> & { className?: string; style?: React.CSSProperties }) {
  return <AriaInput className={cx(css.input, className)} {...rest} />;
}
