import type { ReactNode } from "react";
import css from "./CodeNavSig.module.css";

// CodeNavSig is a definition's signature, or the line that uses it, as
// code: its own line, wrapping where it must.
export function CodeNavSig({ as = "code", children }: { as?: "code" | "div"; children: ReactNode }) {
  const Tag = as;
  return <Tag className={css.sig}>{children}</Tag>;
}
