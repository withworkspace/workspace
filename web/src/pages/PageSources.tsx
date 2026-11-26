import type { ReactNode } from "react";
import css from "./PageSources.module.css";
import { Text } from "../ui/Text";

// PageSources are the files a cell was written from, under it in its
// panel: a PageSourceChip each.
export function PageSources({ children }: { children: ReactNode }) {
  return (
    <div className={css.sources}>
      <Text dim>From</Text>
      {children}
    </div>
  );
}
