import { useState } from "react";
import { Link } from "react-router";
import { Markdown } from "../markdown/LazyMarkdown";
import { Button } from "../ui/Button";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import { Time } from "../ui/Time";
import { turnTimeClass } from "../ui/Turn";
import css from "./TalkSettled.module.css";

// TalkSettled is a card you settled, as the turn ws wrote for you: a
// muted line saying what became of it ("Accepted: mission token-cache"),
// what it names leading there (to), and when (at; none in an ask's card).
// said is the whole of the turn's first line, as the agent read it, said
// when it's pointed at; edits, what you changed first, opens under it.
export function TalkSettled({ accepted, what, to, edits, said, at }: { accepted: boolean; what: string; to?: string; edits?: string; said?: string; at?: string }) {
  const [open, setOpen] = useState(false);
  return (
    <div className={cx(css.marker, accepted && css.accepted)}>
      <div className={css.line} title={said}>
        <Icon name={accepted ? "check-circle" : "x-circle"} size={13} className={css.mark} />
        <span className={css.text}>
          {accepted ? "Accepted" : "Declined"}: {to ? <Link to={to}>{what}</Link> : what}
        </span>
        {edits && (
          <Button variant="link" type="button" className={css.more} aria-expanded={open} onClick={() => setOpen(!open)}>
            edited <Icon name={open ? "chevron-down" : "chevron-right"} size={11} />
          </Button>
        )}
        {at && <Time iso={at} className={turnTimeClass(css.time)} />}
      </div>
      {open && edits && (
        <div className={css.edits}>
          <Markdown text={edits} />
        </div>
      )}
    </div>
  );
}
