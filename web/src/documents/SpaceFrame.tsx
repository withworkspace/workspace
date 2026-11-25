import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./SpaceFrame.module.css";

// SpaceFrame is a space's page: it takes the window's width and, wide,
// its height, its SpaceSplit filling what's under its head.
export function SpaceFrame({ children }: { children: ReactNode }) {
  return <div className={cx("page", css.frame)}>{children}</div>;
}
