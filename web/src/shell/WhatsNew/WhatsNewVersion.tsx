import type { ReactNode } from "react";
import css from "./WhatsNewVersion.module.css";
import { Text } from "../../ui/Text";

// WhatsNewVersion is a release, as What's new says it: which ws this is,
// in its head, and which release an entry's from, where there are several.
export function WhatsNewVersion({ children }: { children: ReactNode }) {
  return <Text mono className={css.version}>{children}</Text>;
}
