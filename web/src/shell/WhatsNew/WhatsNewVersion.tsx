import type { ReactNode } from "react";
import { cx } from "../../ui/cx";
import css from "./WhatsNewVersion.module.css";

// WhatsNewVersion is a release, as What's new says it: which ws this is,
// in its head, and which release an entry's from, where there are several.
export function WhatsNewVersion({ children }: { children: ReactNode }) {
  return <span className={cx(css.version, "mono")}>{children}</span>;
}
