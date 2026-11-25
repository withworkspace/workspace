import type { ReactNode } from "react";
import { Button } from "../ui/Button";
import { Icon } from "../ui/Icon";
import css from "./PageRetry.module.css";

// PageRetry says something couldn't be done (saved, planned), and is
// pressed to try again.
export function PageRetry({ title, onClick, children }: { title?: string; onClick: () => void; children: ReactNode }) {
  return (
    <Button variant="link" className={css.retry} title={title} onClick={onClick}>
      <Icon name="alert" size={12} /> {children}
    </Button>
  );
}
