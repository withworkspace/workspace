import { Link } from "react-router";
import { Button } from "../ui/Button";
import { Icon } from "../ui/Icon";
import { Mark } from "../ui/Mark";
import { Spinner } from "../ui/Spinner";
import { Time } from "../ui/Time";
import { cx } from "../ui/cx";
import css from "./BelowRunRow.module.css";
import { RunLink } from "./RunLink";
import { RunMark } from "./RunMark";

type BelowRunRowProps = {
  name: string;
  // Where it answers (link), as it's read (host); none until it has a port.
  link?: string;
  host?: string;
  // Something answers on its port.
  up: boolean;
  // It started since Below was last closed.
  fresh?: boolean;
  // Where it runs (repo / worktree), said where Below shows everywhere's.
  place?: string;
  // Where it's gone to: its worktree's Runs tab, with it picked.
  to: string;
  started: string;
  // Stopping it is under way.
  busy?: boolean;
  onStop?: () => void;
};

// BelowRunRow is one running service in Below's Runs: its mark, its name
// (going to it, in its worktree's Runs tab), marked where it started
// since you last looked, where it answers (opened in a tab of its own), where
// it runs, how long it's run, and Stop.
export function BelowRunRow({ name, link, host, up, fresh, place, to, started, busy, onStop }: BelowRunRowProps) {
  return (
    <li className={cx(css.row, fresh && css.fresh)}>
      <RunMark live up={up} title={up ? "Up" : "Not up yet"} />
      <span className={css.what}>
        <Link to={to} className={css.name} title={`Show ${name}: preview, details and output`}>
          {name}
        </Link>
        {fresh && <Mark title="Started since you last looked" className={css.mark} />}
        {link && host && (
          <RunLink href={link} title={up ? "Open in new tab" : "Not ready yet"}>
            {host}
          </RunLink>
        )}
        {!up && <span className={css.dim}>starting</span>}
      </span>
      {place && <span className={cx(css.dim, css.place)}>{place}</span>}
      <Time iso={started} as="age" className={cx(css.dim, css.age)} />
      {onStop && (
        <Button variant="ghost" size="small" square disabled={busy} onClick={onStop} title={`Stop ${name}`}>
          {busy ? <Spinner size={11} /> : <Icon name="stop" size={11} />}
        </Button>
      )}
    </li>
  );
}
