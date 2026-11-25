import { Button } from "../ui/Button";
import { Icon } from "../ui/Icon";
import css from "./PagePanelHead.module.css";

// PagePanelHead is a cell's panel's head: what was asked for, and Close.
export function PagePanelHead({ prompt, onClose }: { prompt?: string; onClose: () => void }) {
  return (
    <div className={css.head}>
      <span className={css.glyph} aria-hidden>
        »
      </span>
      <span className={css.prompt}>{prompt}</span>
      <Button variant="ghost" size="small" square onClick={onClose} aria-label="Close" title="Close" keys="esc">
        <Icon name="x" size={13} />
      </Button>
    </div>
  );
}
