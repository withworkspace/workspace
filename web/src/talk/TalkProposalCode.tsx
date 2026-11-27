import type { ComponentProps } from "react";
import { TextArea } from "../ui/TextArea";
import { cx } from "../ui/cx";
import css from "./TalkProposalCode.module.css";

// TalkProposalCode is a card's text box for code (a config file being
// edited before it's confirmed): the mono face, no wrapping, tabs kept.
export function TalkProposalCode({ className, ...rest }: ComponentProps<typeof TextArea>) {
  return <TextArea className={cx(css.code, className)} spellCheck={false} {...rest} />;
}
