import { Fragment, type RefObject } from "react";
import { Link } from "react-router";
import { Crumbs, CrumbSep } from "../../ui/Crumbs";
import { cx } from "../../ui/cx";
import { Icon } from "../../ui/Icon";
import { keyProps } from "../../ui/keyHint";
import type { Shelf } from "../shelves";
import css from "./ReaderCrumbs.module.css";

// ReaderCrumbs is the reader's path: where it's from (Documents, or
// Files), each shelf it's on as a menu of what's there (open: the one
// that's open), and its own title.
// up is the folder u opens now (the one it's kept in; with one open, the
// one above): its crumb says the key.
export function ReaderCrumbs({ from, on, title, open, up, toggle, nav }: { from: { to: string; label: string }; on: Shelf[]; title: string; open?: string; up?: string; toggle: (id: string) => void; nav?: RefObject<HTMLElement | null> }) {
  return (
    <Crumbs className={css.path} ref={nav}>
      <Link to={from.to} className={css.crumb}>
        {from.label}
      </Link>
      <CrumbSep className={css.sep}>/</CrumbSep>
      {on.map((s) => (
        <Fragment key={s.id}>
          <button
            type="button"
            className={cx(css.crumb, open === s.id && css.open)}
            data-folder-crumb={s.id}
            aria-haspopup="dialog"
            aria-expanded={open === s.id}
            {...keyProps(s.id === up ? "u" : undefined, `Browse ${s.name}`, () => toggle(s.id))}
          >
            {s.label}
            <Icon name="chevron-down" size={11} />
          </button>
          <CrumbSep className={css.sep}>/</CrumbSep>
        </Fragment>
      ))}
      <span className={cx(css.crumb, css.here)} aria-current="page">
        {title}
      </span>
    </Crumbs>
  );
}
