import type { ComponentProps } from "react";
import { Composer } from "../ui/Composer";
import { cx } from "../ui/cx";
import css from "./PageComposer.module.css";

// PageComposer is a Composer (beside its buttons) placed in a cell's
// panel, or (flush) under a finding.
export function PageComposer({ flush, ...props }: Omit<ComponentProps<typeof Composer>, "layout" | "className"> & { flush?: boolean }) {
  return <Composer layout="beside" className={cx(css.composer, flush && css.flush)} {...props} />;
}
