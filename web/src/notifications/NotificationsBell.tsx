import { useRef } from "react";
import { dismissNotifications, getNotifications, type Notification, type Notifications, readAllNotifications, readNotifications } from "../api";
import { FootNotifications } from "../chrome/FootNotifications";
import { useChromeStore } from "../chrome/store";
import { useLoaded } from "../load";
import { HeldPopover } from "../ui/Popover";
import { toast } from "../ui/toast";
import { useWorkspace } from "../workspace/store";
import css from "./NotificationsBell.module.css";
import { NotificationsPanel } from "./NotificationsPanel";

// NotificationsBell is the foot's bell and the feed it opens: it reads
// the feed while it's open (and again when the workspace says it moved),
// and says what you do with each notification. How many are unread is the
// workspace's view's, so the bell is marked without the feed being read.
export function NotificationsBell() {
  // Open here, or from a toast that told of what's new (Show).
  const open = useChromeStore((s) => s.bell);
  const setOpen = useChromeStore((s) => s.setBell);
  const at = useRef<HTMLSpanElement>(null);
  const count = useWorkspace((w) => w.notifications);
  const feed = useLoaded(open ? getNotifications : null, [open, count.latest, count.unread], { keep: true });
  const write = (doing: Promise<Notifications>) => doing.then(feed.set, (err: unknown) => toast(err instanceof Error ? err.message : String(err), { tone: "danger" }));
  const read = (n: Notification) => {
    if (!n.read) void write(readNotifications([n.n]));
  };
  return (
    <>
      <span ref={at} className={css.at}>
        <FootNotifications unread={count.unread} open={open} onPress={() => setOpen((now) => !now)} />
      </span>
      <HeldPopover open={open} onOpenChange={setOpen} at={at} placement="top end" offset={8} maxHeight={560} aria-label="Notifications">
        <NotificationsPanel
          notifications={feed.value?.notifications}
          unread={feed.value?.unread ?? count.unread}
          error={feed.error}
          onOpen={(n) => {
            read(n);
            // One with a page goes there, and the feed closes behind it.
            if (n.to) setOpen(false);
          }}
          onRead={read}
          onDismiss={(n) => void write(dismissNotifications([n.n]))}
          onReadAll={() => void write(readAllNotifications())}
          onLeave={() => setOpen(false)}
        />
      </HeldPopover>
    </>
  );
}
