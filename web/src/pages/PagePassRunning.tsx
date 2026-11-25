import type { ReactNode } from "react";
import { Spinner } from "../ui/Spinner";
import css from "./PagePassRunning.module.css";
import { PageVersion } from "./PageVersion";

// PagePassRunning is a pass reading the page: what it's doing (words),
// and its steps as they go (children).
export function PagePassRunning({ words, children }: { words: ReactNode; children: ReactNode }) {
  return (
    <div className={css.running}>
      <PageVersion>
        <Spinner size={12} />
        <span>{words}</span>
      </PageVersion>
      {children}
    </div>
  );
}
