import type { ReactNode } from "react";
import css from "./PagePassSummary.module.css";
import { Prose } from "../ui/Prose";

// PagePassSummary is what a pass said of the page as a whole.
export function PagePassSummary({ children }: { children: ReactNode }) {
  return <Prose className={css.summary}>{children}</Prose>;
}
