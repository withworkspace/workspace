import type { ReactNode } from "react";
import css from "./PageField.module.css";

// PageField is one of PageFields: its name (label), then what's filled
// in (an Input, a Select).
export function PageField({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className={css.field}>
      <span>{label}</span>
      {children}
    </label>
  );
}
