import type { Notification } from "../api";
import { GroupHead } from "../ui/GroupHead";
import { Rows } from "../ui/Rows";
import { dayLabel } from "../ui/when";
import { byDay } from "./feed";
import { NotificationRow } from "./NotificationRow";

type NotificationFeedProps = {
  notifications: Notification[];
  onOpen?: (n: Notification) => void;
  onDismiss?: (n: Notification) => void;
};

// NotificationFeed is the feed, by day, newest first: a NotificationRow
// each.
export function NotificationFeed({ notifications, onOpen, onDismiss }: NotificationFeedProps) {
  return (
    <>
      {byDay(notifications).map((day) => (
        <section key={day.key}>
          <GroupHead>{dayLabel(day.key)}</GroupHead>
          <Rows>
            {day.notifications.map((n) => (
              <NotificationRow key={n.n} notification={n} onOpen={onOpen && (() => onOpen(n))} onDismiss={onDismiss && (() => onDismiss(n))} />
            ))}
          </Rows>
        </section>
      ))}
    </>
  );
}
