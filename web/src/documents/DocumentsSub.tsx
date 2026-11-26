import type { ReactNode } from "react";
import css from "./DocumentsSub.module.css";
import { Text } from "../ui/Text";

// DocumentsSub is what a heading says after its count, quietly: across
// how many spaces, how many pages filling.
export function DocumentsSub({ children }: { children: ReactNode }) {
  return <Text dim className={css.sub}>{children}</Text>;
}
