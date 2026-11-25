import type { ReactNode } from "react";
import { Icon } from "../ui/Icon";
import css from "./ReviewStatus.module.css";

// ReviewStatus says, over the changes, that they're what's changed since
// your last review, or each file's change since a rebase.
export function ReviewStatus({ children }: { children: ReactNode }) {
  return (
    <div className={css.status}>
      <Icon name="rotate" size={13} className={css.icon} />
      <span>{children}</span>
    </div>
  );
}
