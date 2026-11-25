import type { ReactNode } from "react";
import { Banner } from "../ui/Banner";
import css from "./WindowError.module.css";

// WindowError is why the workspace didn't load, said under the top of the
// page.
export function WindowError({ children }: { children: ReactNode }) {
  return <Banner className={css.error}>{children}</Banner>;
}
