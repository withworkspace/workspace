import { Count } from "../ui/Count";
import { Segment, Segmented } from "../ui/Segmented";
import css from "./SpaceTabs.module.css";

export type SpaceTab = { id: string; label: string; count: string; empty: boolean };

// SpaceTabs is a tab for each of a space's lists (a kind's, Other, or a
// folder's), with how many are in it: the one the list's scrolled to is
// on, and one pressed scrolls there. One with none shown (a search's) is
// off.
export function SpaceTabs({ tabs, value, onChange, label }: { tabs: SpaceTab[]; value: string; onChange: (id: string) => void; label: string }) {
  return (
    <Segmented className={css.tabs} value={value} onChange={onChange} aria-label={label}>
      {tabs.map((t) => (
        <Segment key={t.id} value={t.id} className={css.tab} disabled={t.empty}>
          {t.label} <Count className={css.count}>{t.count}</Count>
        </Segment>
      ))}
    </Segmented>
  );
}
