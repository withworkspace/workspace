import type { ReactNode } from "react";
import { Checkbox } from "../ui/Checkbox";
import css from "./PassShow.module.css";

// PassShow is a small checkbox that has the diff follow something of a
// pass's (its findings on their lines) or a guide's (its order).
export function PassShow({ checked, dim, title, onChange, children }: { checked: boolean; dim?: boolean; title: string; onChange: (on: boolean) => void; children: ReactNode }) {
  return (
    <Checkbox className={`${css.show}${dim ? " dim" : ""}`} checked={checked} onChange={onChange} title={title}>
      {children}
    </Checkbox>
  );
}
