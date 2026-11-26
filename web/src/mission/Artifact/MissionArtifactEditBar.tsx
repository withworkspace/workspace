import type { KeyboardEvent, ReactNode } from "react";
import { Input } from "../../ui/Input";
import css from "./MissionArtifactEditBar.module.css";
import { Text } from "../../ui/Text";

type MissionArtifactEditBarProps = {
  // Edit or suggest, where it's a document to comment on.
  mode?: ReactNode;
  // What the commit says (or, suggesting, why), where there's a commit.
  message?: { value: string; placeholder: string; label: string; onChange: (value: string) => void; onKeyDown: (e: KeyboardEvent<HTMLInputElement>) => void };
  // That a draft's kept, and when.
  kept?: ReactNode;
  // What's pressed, at the bar's end: discard, close, save.
  actions: ReactNode;
};

// MissionArtifactEditBar is the bar under a document being edited, which
// stays at the window's foot as it scrolls: edit or suggest, what the
// commit says, that a draft's kept, and what's pressed.
export function MissionArtifactEditBar({ mode, message, kept, actions }: MissionArtifactEditBarProps) {
  return (
    <div className={css.bar}>
      {mode}
      {message && (
        <Input
          className={css.message}
          value={message.value}
          placeholder={message.placeholder}
          aria-label={message.label}
          onChange={(e) => message.onChange(e.target.value)}
          onKeyDown={message.onKeyDown}
        />
      )}
      <Text dim className={css.kept}>{kept}</Text>
      <span className={css.spacer} />
      {actions}
    </div>
  );
}
