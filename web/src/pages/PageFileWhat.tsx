import type { ReactNode } from "react";
import css from "./PageFileWhat.module.css";
import { Text } from "../ui/Text";

// PageFileWhat says what filing or saving a page does, or what starting one
// as a kind of document does, under its fields.
export function PageFileWhat({ children }: { children: ReactNode }) {
  return <Text as="p" dim className={css.what}>{children}</Text>;
}
