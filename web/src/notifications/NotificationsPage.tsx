import { Link } from "react-router";
import { dismissNotifications, getNotifications, readAllNotifications, readNotifications, type Notification } from "../api";
import { useLoaded } from "../load";
import { Button, buttonClass } from "../ui/Button";
import { Count } from "../ui/Count";
import { Empty } from "../ui/Empty";
import { ErrorCard } from "../ui/ErrorCard";
import { PageHead } from "../ui/PageHead";
import { toast } from "../ui/toast";
import { useTitle } from "../ui/useTitle";
import { useWorkspace } from "../workspace/store";
import { NotificationFeed } from "./NotificationFeed";

// NotificationsPage is the feed of what happened: a mission that ended by
// itself, a pull request this ws opened merging, a plugin's update out.
// It's kept, each read or not, until you put it away. What needs you
// isn't here: that's the sidebar's Needs you, worked out from where
// things stand.
export function NotificationsPage() {
  useTitle("Notifications");
  // The workspace's view says when the feed moves (one added, one read
  // from another page): it's read again then.
  const moved = useWorkspace((w) => `${w.notifications.latest} ${w.notifications.unread}`);
  const feed = useLoaded(getNotifications, [moved], { keep: true });
  const write = (doing: Promise<Awaited<ReturnType<typeof getNotifications>>>) =>
    doing.then(feed.set, (err: unknown) => toast(err instanceof Error ? err.message : String(err), { tone: "danger" }));
  const open = (n: Notification) => {
    if (!n.read) void write(readNotifications([n.n]));
  };
  if (feed.error) return <ErrorCard error={feed.error} />;
  const list = feed.value?.notifications;
  const unread = feed.value?.unread ?? 0;
  return (
    <div className="page narrow">
      <PageHead
        title="Notifications"
        actions={
          <>
            {unread > 0 && (
              <Button variant="ghost" size="small" onClick={() => void write(readAllNotifications())}>
                Mark all read
              </Button>
            )}
            <Link to="/settings/notifications" className={buttonClass({ variant: "ghost", size: "small" })}>
              Settings
            </Link>
          </>
        }
      >
        {unread > 0 && <Count title={`${unread} unread`}>{unread}</Count>}
      </PageHead>
      {list && list.length === 0 && (
        <Empty icon="bell" title="No notifications yet">
          What Workspace does for you shows here: a mission ending by itself, a pull request merging, a plugin
          update. What needs you is in the sidebar.
        </Empty>
      )}
      {list && list.length > 0 && <NotificationFeed notifications={list} onOpen={open} onRead={open} onDismiss={(n) => void write(dismissNotifications([n.n]))} />}
    </div>
  );
}
