import type { ReactNode } from "react";
import css from "./PagePanelFoot.module.css";
import { Text } from "../ui/Text";
import { Spacer } from "../ui/Spacer";

// PagePanelFoot is the foot of a cell's panel: what's done with the cell
// (children) and its name (id); or, under the box that asks for a
// change, what asking does (hint).
export function PagePanelFoot({ id, hint, children }: { id?: string; hint?: ReactNode; children?: ReactNode }) {
  return (
    <div className={css.foot}>
      {children}
      {id && (
        <>
          <Spacer />
          <Text dim className={css.id}>{id}</Text>
        </>
      )}
      {hint && <Text dim className={css.hint}>{hint}</Text>}
    </div>
  );
}
