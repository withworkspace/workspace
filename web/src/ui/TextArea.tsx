import type { ComponentProps } from "react";
import { TextArea as AriaTextArea } from "react-aria-components";
import { cx } from "./cx";
import css from "./TextArea.module.css";

// TextArea is more than a line of text to type: a comment, a reply, a
// description. It's a text box as an Input is, drawn here, and takes what
// the browser's textarea takes (value, onChange, rows, a ref, keys); it's
// react-aria's, which also says when it's hovered, focused or invalid.
// It isn't dragged larger: one that grows with its words has useAutosize
// (useAutosize.ts) on its ref, and a page that lets it be dragged says so by a
// class.
export function TextArea({ className, ...rest }: Omit<ComponentProps<typeof AriaTextArea>, "className" | "style"> & { className?: string; style?: React.CSSProperties }) {
  return <AriaTextArea className={cx(css.textarea, className)} {...rest} />;
}
