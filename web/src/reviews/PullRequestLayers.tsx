import { Link } from "react-router";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import { Keys } from "../ui/Keys";
import { Spinner } from "../ui/Spinner";
import { Status, type StatusInfo } from "../ui/Status";
import { ToggleButton } from "../ui/ToggleButton";
import css from "./PullRequestLayers.module.css";

// PullRequestLayer is one of a stack's layers down the side: where it
// stands, its title and number, and where its review is. The one shown
// is here; one asked for and being read is going.
export interface PullRequestLayer {
  number: number;
  title: string;
  words: string;
  status: StatusInfo;
  to: string;
  here?: boolean;
  going?: boolean;
}

// PullRequestLayers are a stack's pull requests down the side of a
// review, bottom first: the order to read them in ([ and ]). Those merged
// below it are said first; the whole stack together (w) is under them.
export function PullRequestLayers({
  layers,
  merged,
  title,
  about,
  whole,
  goingWhole,
  onWhole,
}: {
  layers: PullRequestLayer[];
  merged?: number;
  title: string; // what it's called, on hover
  about: string; // how many layers, and whether ws worked it out
  whole: boolean;
  goingWhole?: boolean;
  onWhole: () => void;
}) {
  return (
    <div className={css.layers}>
      <div className={css.head}>
        <span title={title}>
          Stack <span className="dim">· {about}</span>
        </span>
        <span className={`dim ${css.keys}`}>
          <Keys k="[" /> <Keys k="]" />
        </span>
      </div>
      <ol>
        {merged ? <li className={`dim ${css.merged}`}>{merged} merged below</li> : null}
        {layers.map((l, i) => (
          <li key={l.number} className={cx(l.here && css.here)} aria-busy={l.going || undefined}>
            <Link to={l.to} replace title={`${l.title} (#${l.number}): ${l.words}`}>
              <span className={css.n}>{i + 1}</span>
              {l.going ? <Spinner size={13} /> : <Status s={l.status} iconOnly size={13} className={css.stands} />}
              <span className={css.title}>{l.title}</span>
              <span className="mono dim">#{l.number}</span>
            </Link>
          </li>
        ))}
      </ol>
      <ToggleButton variant="ghost" size="small" className={css.whole} pressed={whole} aria-busy={goingWhole || undefined} onChange={onWhole} title="Every layer's changes together, to see where the stack ends up. It's for reading: comments are made on a layer." keys="w">
        {goingWhole && !whole ? <Spinner size={12} /> : <Icon name="layers" size={12} />} Whole stack <Keys k="w" />
      </ToggleButton>
    </div>
  );
}
