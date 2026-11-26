import type { ReactNode } from "react";
import { Button } from "../ui/Button";
import { Icon } from "../ui/Icon";
import { Menu } from "../ui/Menu";
import css from "./ReviewOpenMenu.module.css";

// ReviewOpenMenu is where else a review's change can be opened, in its
// bar: on GitHub, docked in the workspace to run, its files (children,
// MenuItems).
export function ReviewOpenMenu({ children }: { children: ReactNode }) {
  return (
    <Menu
      aria-label="Open"
      placement="bottom end"
      trigger={
        <Button variant="ghost" size="small" className={css.open} title="Open on GitHub, or here">
          Open <Icon name="chevron-down" size={12} />
        </Button>
      }
    >
      {children}
    </Menu>
  );
}
