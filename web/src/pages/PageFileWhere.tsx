import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import css from "./PageFileWhere.module.css";
import { Text } from "../ui/Text";

// PageFileWhere is where a page would be filed, or is saved: a file in a
// repo (at). taken: there's one there already, which children says.
export function PageFileWhere({ at, taken, children }: { at: string; taken?: boolean; children?: ReactNode }) {
  return (
    <p className={cx(css.where, taken && css.taken)}>
      <Icon name="file" size={12} /> <Text mono>{at.replace(":", " · ")}</Text>
      {children}
    </p>
  );
}
