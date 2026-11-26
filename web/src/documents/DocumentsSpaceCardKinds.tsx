import type { ReactNode } from "react";
import css from "./DocumentsSpaceCardKinds.module.css";
import { Text } from "../ui/Text";

// DocumentsSpaceCardKinds is a space's card's list: a
// DocumentsSpaceCardKind for each of its kinds, or its folders; while
// its documents are being read, what it's said of its folders (reading).
export function DocumentsSpaceCardKinds({ reading, children }: { reading?: string; children?: ReactNode }) {
  return (
    <ul className={css.kinds}>
      {children}
      {reading !== undefined && <Text as="li" dim className={css.reading}>{reading}</Text>}
    </ul>
  );
}
