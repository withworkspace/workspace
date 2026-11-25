import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import { Icon, type IconName } from "../ui/Icon";
import { Spinner } from "../ui/Spinner";
import { AnnotateActions } from "./AnnotateActions";
import css from "./AnnotateSuggestion.module.css";
import { Text } from "../ui/Text";

// AnnotateSuggestion is a box in a thread for something to settle: a
// suggested edit (kind "edit") or what was decided, to record ("decision",
// in the accent). Its head says what it is (icon, title) and who it's
// by; then what it holds, and what's pressed (actionsStart at the start
// of that line, actions at its end).
export function AnnotateSuggestion({
  kind,
  icon,
  title,
  by,
  actionsStart,
  actions,
  children,
}: {
  kind: "edit" | "decision";
  icon: IconName;
  title: ReactNode;
  by?: ReactNode;
  actionsStart?: ReactNode;
  actions: ReactNode;
  children?: ReactNode;
}) {
  return (
    <div className={cx(css.suggestion, kind === "edit" ? css.edit : css.decision)} onClick={(e) => e.stopPropagation()}>
      <div className={css.head}>
        <Icon name={icon} size={12} /> {title} {by && <Text dim className={css.by}>{by}</Text>}
      </div>
      {children}
      <AnnotateActions className={css.actions} start={actionsStart}>
        {actions}
      </AnnotateActions>
    </div>
  );
}

// AnnotateSuggestionBusy says it's being done, in place of what's pressed.
export function AnnotateSuggestionBusy({ children }: { children: ReactNode }) {
  return (
    <Text dim className={css.busy}>
      <Spinner /> {children}
    </Text>
  );
}
