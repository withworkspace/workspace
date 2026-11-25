import { Link } from "react-router";
import type { Place } from "../shell/places";
import { Count } from "../ui/Count";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import { keyProps } from "../ui/keyHint";
import { Mark } from "../ui/Mark";
import css from "./PlaceLink.module.css";

type PlaceLinkProps = {
  place: Place;
  // It's where you are.
  on?: boolean;
  // What's new there since you last looked: a mark, where it has no
  // count. A count is of what needs you, and says more.
  news?: string;
};

// PlaceLink is a place in the sidebar: its icon, its name, and how many
// there need you. In the rail it's its icon, the count a dot on it.
export function PlaceLink({ place, on, news }: PlaceLinkProps) {
  const counted = !!place.count;
  return (
    <Link
      to={place.to}
      aria-current={on ? "page" : undefined}
      className={cx(css.place, on && css.on)}
      {...(place.keys ? keyProps(place.keys, place.label) : {})}
    >
      <Icon name={place.icon} size={15} className={css.icon} />
      <span className={css.words}>{place.label}</span>
      {counted && (
        <Count
          tone="attention"
          className={css.count}
          title={`${place.count} ${place.countTitle ?? "need you"}`}
        >
          {place.count}
        </Count>
      )}
      {!counted && news && <Mark ringed title={news} className={css.news} />}
    </Link>
  );
}
