import type { ReactNode } from "react";
import css from "./ToastPill.module.css";
import { Button } from "./Button";
import { cx } from "./cx";
import { Icon } from "./Icon";

// ToastPill is one thing said in passing: a dark pill with its tone's icon,
// the words, and something to press where there is (Undo, Open). Toasts
// draws the ones being said; toast() (ui/toast.ts) says one.
export function ToastPill({ tone = "info", children, action }: { tone?: "info" | "danger" | "done"; children: ReactNode; action?: { label: string; run: () => void } }) {
  return (
    <div className={cx(css.toast, tone === "danger" && css.danger, tone === "done" && css.done)}>
      <Icon name={tone === "danger" ? "x-circle" : tone === "done" ? "check-circle" : "info"} size={14} />
      <span>{children}</span>
      {action && (
        <Button variant="link" className={css.action} onClick={action.run}>
          {action.label}
        </Button>
      )}
    </div>
  );
}
