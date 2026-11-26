import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import { Time } from "../ui/Time";
import css from "./RunsPagePast.module.css";
import { textClass } from "../ui/Text";

// RunsPagePast is the runs of a task that are kept, as a row of small
// tabs over the one that's open: a RunsPagePastRun each.
export function RunsPagePast({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className={css.past} role="tablist" aria-label={label}>
      {children}
    </div>
  );
}

// RunsPagePastRun is one run of a task, its tab: what it came to as its
// icon (result), its number, and when it ended (at). on: it's the one
// open.
export function RunsPagePastRun({ n, result, at, on, title, onPick }: { n?: number; result?: string; at: string; on: boolean; title: string; onPick: () => void }) {
  return (
    <button role="tab" aria-selected={on} className={cx(css.run, result === "passed" && css.passed, result === "failed" && css.failed, on && css.on)} onClick={onPick} title={title}>
      <Icon name={result === "passed" ? "check-circle" : result === "failed" ? "x-circle" : result === "running" ? "circle-dot" : "circle-dashed"} size={11} />
      <span>#{n}</span>
      <Time iso={at} as="relative" className={textClass("dim")} />
    </button>
  );
}
