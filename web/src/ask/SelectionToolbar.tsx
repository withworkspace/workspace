import type { Ref } from "react";
import { Button } from "../ui/Button";
import { Icon } from "../ui/Icon";
import { Keys } from "../ui/Keys";
import css from "./SelectionToolbar.module.css";

type SelectionToolbarProps = {
  // Where it stands: over the middle of what's picked out, in the window.
  left: number;
  top: number;
  onAsk: () => void;
  // What asking is, said in full, where it isn't the page's own agent
  // that's asked (a mission's planner).
  askTitle?: string;
  // None where asking is all there is to offer (a mission's brief).
  onExplain?: () => void;
  // A link to where it is (its section), where there's one to copy.
  onLink?: () => void;
  ref?: Ref<HTMLDivElement>;
};

// SelectionToolbar is what's offered over a passage picked out of what's
// read: Ask about this (a), Explain, and Link. Pressing it keeps the
// selection (and the focus) where they are.
export function SelectionToolbar({ left, top, onAsk, askTitle = "Ask about this passage", onExplain, onLink, ref }: SelectionToolbarProps) {
  return (
    <div ref={ref} className={css.tool} role="toolbar" aria-label="The passage picked out" style={{ left, top }} onMouseDown={(e) => e.preventDefault()}>
      <Button type="button" onClick={onAsk} title={askTitle} keys="a">
        <Icon name="sparkle" size={12} /> Ask about this <Keys k="a" />
      </Button>
      {onExplain && (
        <Button type="button" onClick={onExplain} title="Ask for this passage to be explained">
          <Icon name="quote" size={12} /> Explain
        </Button>
      )}
      {onLink && (
        <Button type="button" onClick={onLink} title="Copy a link to where it is: its section">
          <Icon name="link" size={12} /> Link
        </Button>
      )}
    </div>
  );
}
