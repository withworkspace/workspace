import type { ReactNode } from "react";
import css from "./DocumentsChanges.module.css";
import { Text } from "../ui/Text";

// DocumentsChanges is changes to documents, each a DocumentsChangeLine;
// more says how many earlier ones aren't shown.
export function DocumentsChanges({ more, children }: { more?: ReactNode; children: ReactNode }) {
  return (
    <ul className={css.changes}>
      {children}
      {more && <Text as="li" dim className={css.more}>{more}</Text>}
    </ul>
  );
}
