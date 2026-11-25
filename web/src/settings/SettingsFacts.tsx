import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./SettingsFacts.module.css";

// SettingsFacts is a list of things to know, each named: which ws this
// is, where the workspace keeps its files. A SettingsFact each.
export function SettingsFacts({ children }: { children: ReactNode }) {
  return <dl className={css.facts}>{children}</dl>;
}

type SettingsFactProps = {
  term: ReactNode;
  // What an address's hash names it by.
  id?: string;
  // mono is for what's read as it's typed: a path, a version.
  mono?: boolean;
  // A class of what holds it, to lay out what it says.
  className?: string;
  children: ReactNode;
};

// SettingsFact is one of them: its name, and what it is.
export function SettingsFact({ term, id, mono, className, children }: SettingsFactProps) {
  return (
    <>
      <dt id={id}>{term}</dt>
      <dd className={cx(mono && "mono", className) || undefined}>{children}</dd>
    </>
  );
}
