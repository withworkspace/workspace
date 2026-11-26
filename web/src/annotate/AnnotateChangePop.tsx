import { Button } from "../ui/Button";
import { Icon } from "../ui/Icon";
import css from "./AnnotateChangePop.module.css";
import { Text } from "../ui/Text";

// AnnotateChangePop is what a changed passage said when you last commented
// (old, its lines), under its bar (top); added false: it's been removed
// since.
export function AnnotateChangePop({ top, old, added, onClose }: { top: number; old: string[]; added: boolean; onClose: () => void }) {
  const text = old.join("\n").trim();
  return (
    <div className={css.pop} style={{ top }} onClick={(e) => e.stopPropagation()}>
      <div className={css.head}>
        <Text dim>{text ? "When you last commented, this read:" : "Added since you last commented."}</Text>
        <span className={css.spacer} />
        <Button variant="ghost" size="small" square onClick={onClose} title="Close" keys="esc" aria-label="Close">
          <Icon name="x" size={12} />
        </Button>
      </div>
      {text && <pre className={css.was}>{text}</pre>}
      {!added && <Text as="div" dim>Removed since.</Text>}
    </div>
  );
}
