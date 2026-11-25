import type { ComponentProps } from "react";
import { Composer } from "../ui/Composer";
import { cx } from "../ui/cx";
import css from "./TalkNew.module.css";

// TalkNew is where a conversation's started, over the list: a Composer,
// fresh when it's come to from "New mission", to be written in at once.
export function TalkNew({ fresh, className, ...rest }: ComponentProps<typeof Composer> & { fresh?: boolean }) {
  return <Composer {...rest} className={cx(css.new, fresh && css.fresh, className)} />;
}
