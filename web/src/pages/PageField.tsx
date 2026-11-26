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

// pageFieldCodeClass is a field whose words are a path or a commit's
// message (an Input given textClass("mono")): the code's type, a size
// down, with room between its lines.
export const pageFieldCodeClass: string = css.code;
