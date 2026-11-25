import type { ReactNode } from "react";
import { Button } from "../ui/Button";
import { Count } from "../ui/Count";
import { Icon } from "../ui/Icon";
import { Popover } from "../ui/Popover";
import css from "./ArtifactTweaksButton.module.css";

type ArtifactTweaksButtonProps = {
  // Whose tweaks they are: the button's name, and what it opens.
  label: string;
  // How many aren't as the page opens them, said on the button.
  changed: number;
  // The tweaks: their head and their body.
  children: ReactNode;
  // Open from the first, for a picture of it.
  defaultOpen?: boolean;
};

// ArtifactTweaksButton is a frame's tweaks behind a button, the width of
// the tray. Keys typed in what it opens are its own, not the page's
// (Popover says data-field there, not on the button, which has the focus
// again once it's shut, when the page's keys should work).
export function ArtifactTweaksButton({ label, changed, children, defaultOpen }: ArtifactTweaksButtonProps) {
  return (
    <Popover
      className={css.popover}
      placement="bottom end"
      aria-label={label}
      defaultOpen={defaultOpen}
      trigger={
        <Button variant="ghost" size="small" className={css.button} aria-label={label}>
          <Icon name="sliders" size={13} /> Tweaks
          {changed ? <Count>{changed}</Count> : null}
        </Button>
      }
    >
      <div className={css.dialog}>{children}</div>
    </Popover>
  );
}
