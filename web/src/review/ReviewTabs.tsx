import { Button } from "../ui/Button";
import { cx } from "../ui/cx";
import { Icon, type IconName } from "../ui/Icon";
import css from "./ReviewTabs.module.css";

// ReviewTabs are a review's views as a row of tabs, for a narrow window,
// where the list down the side isn't. Each has its key (1 for the first),
// and a dot while there's something new in it.
export function ReviewTabs({ views, here, onPick }: { views: { key: string; label: string; icon: IconName; n: number; news?: string }[]; here: string; onPick: (key: string) => void }) {
  return (
    <nav className={css.tabs} aria-label="Views">
      {views.map((v) => (
        <Button variant="ghost" size="small" key={v.key} className={cx(css.tab, here === v.key && css.on)} aria-pressed={here === v.key} onClick={() => onPick(v.key)} keys={`${v.n}`}>
          <Icon name={v.icon} size={12} /> {v.label}
          {v.news && <span className={css.news} aria-label={v.news} />}
        </Button>
      ))}
    </nav>
  );
}
