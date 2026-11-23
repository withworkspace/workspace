import { Fragment } from "react";
import { headed, type Place } from "../shell/places";
import { cx } from "../ui/cx";
import { PlaceLink } from "./PlaceLink";
import css from "./Places.module.css";

type PlacesProps = {
  // The sidebar's places, as arranged, and the page you're on.
  places: Place[];
  path: string;
  // What's new at a place since you last looked, by the place's id: said
  // by a mark on it.
  news?: Record<string, string>;
  className?: string;
};

// Places is the sidebar's places, a PlaceLink each, with a plugin's
// group's title above where its group starts.
export function Places({ places, path, news, className }: PlacesProps) {
  const titles = headed(places);
  return (
    <nav aria-label="Places" className={cx(css.places, className)}>
      {places.map((p, i) => (
        <Fragment key={p.id}>
          {titles[i] && <div className={css.group}>{titles[i]}</div>}
          <PlaceLink place={p} on={p.match(path)} news={news?.[p.id]} />
        </Fragment>
      ))}
    </nav>
  );
}
