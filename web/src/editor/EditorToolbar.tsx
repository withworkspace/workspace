import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import { Icon, type IconName } from "../ui/Icon";
import { ToggleButton } from "../ui/ToggleButton";
import css from "./EditorToolbar.module.css";

type EditorToolbarProps = {
  // In a card: part of it, not sticking to the top.
  compact?: boolean;
  // In a page's own bar (the brief's), where the page put it: part of the
  // bar.
  placed?: boolean;
  // Showing the markdown as text, not the document as it reads.
  source?: boolean;
  onSource: () => void;
  // What's at its start: EditorTools and EditorToolSeps, or words.
  children?: ReactNode;
};

// EditorToolbar is the editor's row of what the keys and the typed
// markdown do, as buttons, and at its end Markdown, which shows the text
// itself. It says doc-toolbar too: a page's bar still finds it by that
// (the brief's hides itself until it holds one).
export function EditorToolbar({ compact, placed, source = false, onSource, children }: EditorToolbarProps) {
  return (
    <div className={cx("doc-toolbar", css.toolbar, compact && css.compact, placed && css.placed)} role="toolbar" aria-label="Formatting">
      {children}
      <span className={css.spacer} />
      <ToggleButton variant="ghost" size="small" pressed={source} onChange={onSource} title={source ? "Back to the document as it reads" : "Edit the markdown itself"}>
        <Icon name="hash" size={12} /> Markdown
      </ToggleButton>
    </div>
  );
}

type EditorToolProps = {
  // An icon, or words of two letters at most (H1).
  icon: IconName | string;
  title: string;
  // Lit: the selection's in what it makes.
  on: boolean;
  disabled?: boolean;
  onPress: () => void;
};

// EditorTool is one of the toolbar's buttons. It keeps the editor's
// selection when it's pressed: it acts on it.
export function EditorTool({ icon, title, on, disabled, onPress }: EditorToolProps) {
  return (
    <ToggleButton
      variant="ghost"
      square
      className={css.tool}
      pressed={on}
      title={title}
      aria-label={title}
      disabled={disabled}
      onMouseDown={(e) => e.preventDefault()}
      onChange={onPress}
    >
      {icon.length <= 2 ? <span className={css.level}>{icon}</span> : <Icon name={icon as IconName} />}
    </ToggleButton>
  );
}

// EditorToolSep is a rule between the toolbar's groups.
export function EditorToolSep() {
  return <span className={css.sep} />;
}
