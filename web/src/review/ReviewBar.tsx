import type { ReactNode } from "react";
import { Icon } from "../ui/Icon";
import { keyProps } from "../ui/keyHint";
import { TLink } from "../ui/TLink";
import css from "./ReviewBar.module.css";

type ReviewBarProps = {
  // The way back: drawn here only where the list down the side (which has
  // it, ReviewSideBack) isn't, on a narrow window.
  back: { to: string; label: string };
  // Where it stands, as an icon that says more when pointed at
  // (ReviewStand).
  stand?: ReactNode;
  // Its name: cut short before it pushes the rest off the row.
  name: ReactNode;
  // After its name: what it's in, who it's by, what it goes into, whether
  // it can go in. These give way to the name on a narrower window.
  about?: ReactNode;
  // At the bar's end: what's done to the whole review.
  actions: ReactNode;
};

// ReviewBar is a review's bar: one row over the pane, which stays at the
// top as the pane scrolls under it, its rule running the pane's width.
// What's done in one view (the changes' scope and layout) is that view's
// own bar, under this one (ReviewChangesBar).
export function ReviewBar({ back, stand, name, about, actions }: ReviewBarProps) {
  return (
    <div className={css.bar} data-review-bar="">
      <header className={css.head}>
        <TLink to={back.to} className={css.back} aria-label={`Back to ${back.label}`} {...keyProps("esc", `Back to ${back.label}`)}>
          <Icon name="arrow-left" size={15} />
        </TLink>
        {stand}
        <h1 className={css.name}>{name}</h1>
        {about && <span className={css.about}>{about}</span>}
      </header>
      <div className={css.actions}>{actions}</div>
    </div>
  );
}
