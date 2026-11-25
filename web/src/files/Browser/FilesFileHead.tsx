import type { ReactNode } from "react";
import { Button } from "../../ui/Button";
import { cx } from "../../ui/cx";
import { Icon } from "../../ui/Icon";
import css from "./FilesFileHead.module.css";

// FilesFileHead is the open file's head, stuck to the top as it scrolls:
// the way back to the files (narrow), where it is, how much there is of
// it, and what's done with it (tools).
export function FilesFileHead({ onBack, path, size, tools }: { onBack?: () => void; path: ReactNode; size?: ReactNode; tools?: ReactNode }) {
  return (
    <header className={css.head}>
      {onBack && (
        <Button variant="ghost" square onClick={onBack} aria-label="Back to the files">
          <Icon name="arrow-left" size={15} />
        </Button>
      )}
      {path}
      {size && <span className={cx("dim", css.size)}>{size}</span>}
      <div className={css.tools}>{tools}</div>
    </header>
  );
}
