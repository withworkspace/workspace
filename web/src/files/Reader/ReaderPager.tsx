import { Link } from "react-router";
import { Icon } from "../../ui/Icon";
import { Keys } from "../../ui/Keys";
import type { Siblings } from "../shelves";
import css from "./ReaderPager.module.css";

// ReaderPager is the documents either side of it, and where it is among
// those kept with it (17 of 28 in its folder), at its foot: [ and ] go
// there.
export function ReaderPager({ sib, to }: { sib: Siblings; to: (path: string) => string }) {
  const where = sib.shelf.name;
  return (
    <nav className={css.pager} aria-label={`In ${where}`}>
      {sib.prev?.path ? (
        <Link to={to(sib.prev.path)} rel="prev">
          <small>
            <Icon name="arrow-left" size={12} /> Previous in {where} <Keys k="[" />
          </small>
          <b>{sib.prev.title}</b>
        </Link>
      ) : (
        <span className={css.none} />
      )}
      <span className={css.pos}>
        {sib.at} of {sib.of}
        <br />
        in {where}
      </span>
      {sib.next?.path ? (
        <Link className={css.next} to={to(sib.next.path)} rel="next">
          <small>
            <Keys k="]" /> Next in {where} <Icon name="arrow-right" size={12} />
          </small>
          <b>{sib.next.title}</b>
        </Link>
      ) : (
        <span className={css.none} />
      )}
    </nav>
  );
}
