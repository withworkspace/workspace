import type { ReactNode } from "react";
import { Copyable } from "../ui/Copyable";
import { cx } from "../ui/cx";
import css from "./RunPanelFacts.module.css";
import { textClass } from "../ui/Text";

// RunPanelFacts is how and why a run was run, in its panel: facts side by
// side, a RunPanelFact each, as many to a row as there's room for.
export function RunPanelFacts({ children }: { children: ReactNode }) {
  return <dl className={css.facts}>{children}</dl>;
}

// RunPanelFact is one of what's said of a run: a quiet label over it.
// wide takes the width of the row.
export function RunPanelFact({ label, wide, children }: { label: string; wide?: boolean; children: ReactNode }) {
  return (
    <div className={cx(css.fact, wide && css.wide)}>
      <dt>{label}</dt>
      <dd>{children}</dd>
    </div>
  );
}

// RunPanelAside is what's said under a fact, quieter: who started it,
// what a recipe's for, how a need's handed over.
export function RunPanelAside({ children }: { children: ReactNode }) {
  return <span className={css.aside}>{children}</span>;
}

// RunPanelCommand is the command a run ran, which copies itself.
export function RunPanelCommand({ command }: { command: string }) {
  return (
    <Copyable text={command} className={cx(textClass("mono"), css.command)}>
      {command}
    </Copyable>
  );
}

// RunPanelReports holds a task's reports, a Chip each.
export function RunPanelReports({ children }: { children: ReactNode }) {
  return <span className={css.reports}>{children}</span>;
}
