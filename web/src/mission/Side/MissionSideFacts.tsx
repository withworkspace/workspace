import type { ReactNode } from "react";
import { cx } from "../../ui/cx";
import css from "./MissionSideFacts.module.css";
import { textClass } from "../../ui/Text";
import { srOnlyClass } from "../../ui/SrOnly";

// MissionSideFacts is the top of a mission's details: what's known of it,
// each a MissionSideFact, its name beside it.
export function MissionSideFacts({ children }: { children: ReactNode }) {
  return <dl className={css.facts}>{children}</dl>;
}

// MissionSideFact is one thing known of a mission: its name (hidden where
// what it is says it: its state), and the thing. A state takes the row,
// with what's done to it at its end; where it's at is quiet; what it's
// about is names in a row; mono, it's a name to copy (a slug, a branch).
export function MissionSideFact({
  label,
  hideLabel,
  kind,
  mono,
  children,
}: {
  label: string;
  hideLabel?: boolean;
  kind?: "state" | "at" | "about";
  mono?: boolean;
  children: ReactNode;
}) {
  return (
    <>
      <dt className={hideLabel ? srOnlyClass : undefined}>{label}</dt>
      <dd className={cx(kind && kinds[kind], mono && textClass("mono")) || undefined}>{children}</dd>
    </>
  );
}

const kinds = { state: css.state, at: css.at, about: css.about };
