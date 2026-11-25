import type { ReactNode } from "react";
import css from "./DocumentsEmpty.module.css";
import { Text } from "../ui/Text";

// DocumentsEmpty is a line said where a list would be: there are none,
// it's being searched, nothing says that.
export function DocumentsEmpty({ children }: { children: ReactNode }) {
  return <Text as="p" dim className={css.empty}>{children}</Text>;
}
