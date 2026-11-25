import { Icon } from "../ui/Icon";
import { Mark } from "../ui/Mark";
import { FootItem } from "./FootItem";
import css from "./FootNotifications.module.css";

type FootNotificationsProps = {
  // How many you haven't read: the bell is marked while there are any.
  unread: number;
  // Its feed is open over the page.
  open?: boolean;
  onPress?: () => void;
};

// FootNotifications is the bell, at the foot's end: what happened that
// you'd want to know (the feed of notifications), opened over the page
// from it. It's marked while something's unread, and never a count in
// amber: that's for what needs you.
export function FootNotifications({ unread, open, onPress }: FootNotificationsProps) {
  const says = unread > 0 ? `Notifications: ${unread} unread` : "Notifications";
  return (
    <FootItem on={open} label={says} title={says} onPress={onPress}>
      <span className={css.icon}>
        <Icon name="bell" size={13} />
        {unread > 0 && <Mark ringed className={css.mark} />}
      </span>
      {unread > 0 && <span className={css.count}>{unread}</span>}
    </FootItem>
  );
}
