import type { ReactNode, Ref } from "react";
import { Button } from "../ui/Button";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import { Keys } from "../ui/Keys";
import css from "./CodeNavPanel.module.css";

// CodeNavPanel is the panel beside the diff in code navigation: what it
// shows (title) and of which side (said), how it was found (how), what
// opens it elsewhere (tools), back (b) and close; then its body, which
// scrolls, or a note while it's read. Who uses a name (uses) is a list, set in from the edges.
export function CodeNavPanel({
  label,
  title,
  said,
  how,
  tools,
  uses,
  note,
  body,
  onBack,
  onClose,
  children,
}: {
  label: string;
  title: ReactNode;
  said: ReactNode;
  how?: string;
  tools?: ReactNode;
  uses?: boolean;
  note?: string; // said where its body would be (it's being read)
  body?: Ref<HTMLDivElement>;
  onBack: () => void;
  onClose: () => void;
  children: ReactNode;
}) {
  return (
    <aside className={css.panel} aria-label={label} data-nav-panel="">
      <header className={css.head}>
        {title}
        <span className="dim">{said}</span>
        {how && <span className={`dim ${css.how}`}>{how}</span>}
        <span className={css.spacer} />
        {tools}
        <Button variant="ghost" size="small" onClick={onBack} title="Back">
          <Icon name="arrow-left" size={12} /> <Keys k="b" />
        </Button>
        <Button variant="ghost" size="small" onClick={onClose} title="Close">
          <Icon name="x" size={13} />
        </Button>
      </header>
      <div className={cx(css.body, uses && css.uses)} ref={body}>
        {note && <div className={`dim ${css.note}`}>{note}</div>}
        {children}
      </div>
    </aside>
  );
}
