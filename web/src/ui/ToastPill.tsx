import type { ReactNode } from "react";
import { Text, UNSTABLE_ToastContent as ToastContent } from "react-aria-components";
import { Button } from "./Button";
import { cx } from "./cx";
import { Icon } from "./Icon";
import { Keys } from "./Keys";
import css from "./ToastPill.module.css";

// ToastPill is one thing said in passing: a dark pill with its tone's icon,
// the words (and a key after them, for a key hint), something to press
// where there is (Undo, Open), and, closable, an × that shows when it's
// pointed at or focused. Toasts draws the ones being said, each in
// react-aria's Toast, which the words name and the × closes (their
// slots); toast() (ui/toast.ts) says one. Alone, as in a story, it's the
// pill and nothing more.
export function ToastPill({
  tone = "info",
  keys,
  action,
  closable,
  children,
}: {
  tone?: "info" | "danger" | "done";
  keys?: string;
  action?: { label: string; run: () => void };
  closable?: boolean;
  children: ReactNode;
}) {
  return (
    <div className={cx(css.toast, tone === "danger" && css.danger, tone === "done" && css.done)}>
      <Icon name={tone === "danger" ? "x-circle" : tone === "done" ? "check-circle" : "info"} size={14} />
      <ToastContent className={css.content}>
        <Text slot="title">
          {children}
          {keys && (
            <>
              {" "}
              <Keys k={keys} />
            </>
          )}
        </Text>
      </ToastContent>
      {action && (
        <Button variant="link" className={css.action} onClick={action.run}>
          {action.label}
        </Button>
      )}
      {closable && (
        <Button variant="bare" slot="close" className={css.close} aria-label="Close">
          <Icon name="x" size={12} />
        </Button>
      )}
    </div>
  );
}
