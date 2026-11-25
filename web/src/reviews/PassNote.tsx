import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import css from "./PassNote.module.css";

// PassNote is something said of a pass (or a guide) under its head: why
// it can't run, the step it's at while it runs (steps, one line), or that
// it's been pushed to since (moved, with its icon).
export function PassNote({ kind, children }: { kind?: "steps" | "moved"; children: ReactNode }) {
  return (
    <p className={cx(css.note, kind !== "moved" && "dim", kind === "steps" && css.steps, kind === "moved" && css.moved)}>
      {kind === "moved" && <Icon name="rotate" size={12} />}
      {children}
    </p>
  );
}
