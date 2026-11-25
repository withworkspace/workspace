import { Link } from "react-router";
import type { Notification } from "../api";
import { Button } from "../ui/Button";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import { Mark } from "../ui/Mark";
import { Row, RowLine, RowMain, RowMeta, RowSub, rowLinkClass, rowStatusClass } from "../ui/Rows";
import { Time } from "../ui/Time";
import { sourceIcon } from "./feed";
import css from "./NotificationRow.module.css";

type NotificationRowProps = {
  notification: Notification;
  // Opened: it's read, and its page (if it has one) is where it goes.
  onOpen?: () => void;
  // Put away: out of the feed.
  onDismiss?: () => void;
};

// NotificationRow is one thing that happened, in the feed: where it's
// from as an icon, what happened as a sentence, when, and a mark while
// you haven't read it. It opens what it's about, where there's a page for
// that; one with none is read by pressing it.
export function NotificationRow({ notification: n, onOpen, onDismiss }: NotificationRowProps) {
  const inside = (
    <>
      <Icon name={sourceIcon(n)} size={15} className={rowStatusClass(cx(css.icon, n.attention && css.attention))} />
      <RowMain className={css.main}>
        <RowLine>
          <span className={cx(css.text, !n.read && css.unread)}>{n.text}</span>
        </RowLine>
        {n.refs && n.refs.length > 0 && <RowSub>{n.refs.join(" · ")}</RowSub>}
      </RowMain>
      {!n.read && <Mark title="Unread" className={css.mark} />}
      <RowMeta>
        <Time iso={n.at} as="clock" />
      </RowMeta>
    </>
  );
  return (
    <Row className={css.row}>
      {n.to ? (
        <Link to={n.to} className={rowLinkClass(css.link)} onClick={onOpen}>
          {inside}
        </Link>
      ) : (
        <button type="button" className={rowLinkClass(cx(css.link, css.plain))} onClick={onOpen}>
          {inside}
        </button>
      )}
      {onDismiss && (
        <Button variant="bare" size="small" title="Dismiss" aria-label={`Dismiss ${n.ref}`} className={css.dismiss} onClick={onDismiss}>
          <Icon name="x" size={13} />
        </Button>
      )}
    </Row>
  );
}
