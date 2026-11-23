import { Button } from "../../ui/Button";
import { Icon, type IconName } from "../../ui/Icon";
import css from "./FindAct.module.css";

// FindAct is one thing to do to the thing in Find's pane: a small quiet
// button, its icon before its words. Pressing it leaves the focus in
// Find's text box, for what it may ask next.
export function FindAct({ icon, disabled, onClick, children }: { icon: IconName; disabled?: boolean; onClick: () => void; children: string }) {
  return (
    <Button variant="ghost" size="small" className={css.act} type="button" disabled={disabled} onMouseDown={(e) => e.preventDefault()} onClick={onClick}>
      <Icon name={icon} size={12} /> {children}
    </Button>
  );
}
