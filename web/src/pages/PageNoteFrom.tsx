import { Button } from "../ui/Button";
import { Icon } from "../ui/Icon";
import css from "./PageNoteFrom.module.css";
import { Spacer } from "../ui/Spacer";

// PageNoteFrom is the head of a finding opened from its words: the pass
// it's from (to open it), and Close.
export function PageNoteFrom({ pass, onPass, onClose }: { pass: string; onPass: () => void; onClose: () => void }) {
  return (
    <div className={css.from}>
      <Button variant="link" onClick={onPass} title="Open pass">
        {pass}
      </Button>
      <Spacer />
      <Button variant="ghost" size="small" square onClick={onClose} aria-label="Close" title="Close" keys="esc">
        <Icon name="x" size={13} />
      </Button>
    </div>
  );
}
