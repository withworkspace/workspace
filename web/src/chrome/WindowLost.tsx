import { Spinner } from "../ui/Spinner";
import css from "./WindowLost.module.css";

// WindowLost says, across the top of the page, that the workspace's daemon
// has gone and for how long (for, in words), while it's reconnected.
// What's on the page under it is dimmed by windowStaleClass, given to
// the page it's in.
export function WindowLost({ for: lost }: { for: string }) {
  return (
    <div className={css.lost} role="status">
      <Spinner />
      <span>
        Lost the workspace's daemon {lost} ago. Reconnecting… What's here may be out of date; <code>ws web</code> starts it again.
      </span>
    </div>
  );
}

// windowStaleClass is the page's (Shell's pageClassName) while the daemon's
// lost: all in it but the WindowLost is dimmed, as it may be stale.
export const windowStaleClass = css.stale;
