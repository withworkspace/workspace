import { Link } from "react-router";
import type { Notification } from "../api";
import { Button, buttonClass } from "../ui/Button";
import { Count } from "../ui/Count";
import { Empty } from "../ui/Empty";
import { Loading } from "../ui/Loading";
import { NotificationFeed } from "./NotificationFeed";
import css from "./NotificationsPanel.module.css";

type NotificationsPanelProps = {
  // The feed, newest first: undefined while it's being read.
  notifications?: Notification[];
  unread: number;
  // Why it couldn't be read, if it couldn't.
  error?: string;
  // A row opened (it's read, and its page is gone to), read where it is,
  // or put away.
  onOpen?: (n: Notification) => void;
  onRead?: (n: Notification) => void;
  onDismiss?: (n: Notification) => void;
  onReadAll?: () => void;
  // One of its links was followed: whatever holds it closes.
  onLeave?: () => void;
};

// NotificationsPanel is the feed as it opens from the foot's bell: its
// heading with how many are unread and Mark all read, the feed by day
// (each row read by its tick, without opening it), and the way to the
// whole of it and to where each source is sent.
export function NotificationsPanel({ notifications, unread, error, onOpen, onRead, onDismiss, onReadAll, onLeave }: NotificationsPanelProps) {
  return (
    <div className={css.panel}>
      <header className={css.head}>
        <h2 className={css.title}>Notifications</h2>
        {unread > 0 && <Count title={`${unread} unread`}>{unread}</Count>}
        <span className={css.space} />
        {unread > 0 && onReadAll && (
          <Button variant="ghost" size="small" onClick={onReadAll}>
            Mark all read
          </Button>
        )}
      </header>
      <div className={css.feed}>
        {error && <p className={css.error}>{error}</p>}
        {!error && !notifications && <Loading what="Reading the feed…" />}
        {notifications && notifications.length === 0 && (
          <Empty icon="bell" title="Nothing's happened yet">
            What ws does for you is said here as it happens.
          </Empty>
        )}
        {notifications && notifications.length > 0 && <NotificationFeed notifications={notifications} onOpen={onOpen} onRead={onRead} onDismiss={onDismiss} compact />}
      </div>
      <footer className={css.foot}>
        <Link to="/notifications" className={buttonClass({ variant: "ghost", size: "small" })} onClick={onLeave}>
          All notifications
        </Link>
        <Link to="/settings/notifications" className={buttonClass({ variant: "ghost", size: "small" })} onClick={onLeave}>
          Settings
        </Link>
      </footer>
    </div>
  );
}
