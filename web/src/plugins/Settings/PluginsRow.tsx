import type { ReactNode } from "react";
import { Link } from "react-router";
import { Chip, chipClass } from "../../ui/Chip";
import { cx } from "../../ui/cx";
import { Icon, type IconName } from "../../ui/Icon";
import { Row, RowMain, RowSub } from "../../ui/Rows";
import { Time } from "../../ui/Time";
import { type Tone, toneClass } from "../../ui/tone";
import css from "./PluginsRow.module.css";

type PluginsRowProps = {
  // ready and failed colour its icon; one of the team's that isn't
  // installed here (wanted) has a quieter name.
  kind?: "ready" | "failed" | "wanted";
  icon: IconName;
  name: string;
  // Where its first view opens, to go there by its name.
  to?: string;
  version?: string;
  // Where it stands, as a chip.
  state?: { label: string; tone?: Tone; title?: string };
  // A later release that's out, and where it's said what's in it.
  out?: { label: string; href: string };
  about?: string;
  // Where it's from: a repo (linked), or a folder; and when it was fetched.
  from?: { label: string; href?: string; at?: string };
  // What it adds: its views, commands, guide and names.
  adds?: string[];
  // Where it's added from, for one that isn't yet.
  source?: string;
  // What's said of it under, each a PluginsRowNote.
  notes?: ReactNode;
  // What's done to it, at its end.
  actions: ReactNode;
  // What it opens (a dialog).
  children?: ReactNode;
};

// PluginsRow is a plugin, in Settings: its name, where it stands and
// what it adds, with what's to be done to it at its end.
export function PluginsRow({ kind, icon, name, to, version, state, out, about, from, adds, source, notes, actions, children }: PluginsRowProps) {
  return (
    <Row className={cx(css.row, kind === "ready" && css.ready, kind === "failed" && css.failed, kind === "wanted" && css.wanted)}>
      <span className={css.icon} aria-hidden>
        <Icon name={icon} size={16} />
      </span>
      <RowMain as="div" className={css.main}>
        <div className={css.top}>
          {to ? (
            <Link to={to} className={css.title} title={`Open ${name}`}>
              {name}
            </Link>
          ) : (
            <span className={css.title}>{name}</span>
          )}
          {version && <span className={cx("mono", "dim", css.version)}>{version}</span>}
          {state && (
            <Chip tone={state.tone} title={state.title}>
              {state.label}
            </Chip>
          )}
          {out && (
            <a className={chipClass("attention", css.out)} href={out.href} target="_blank" rel="noopener noreferrer" title="What's in it, on GitHub">
              {out.label}
              <Icon name="arrow-up-right" size={10} />
            </a>
          )}
        </div>
        {about && <div className={css.about}>{about}</div>}
        <RowSub as="div" className={css.sub}>
          {from && (
            <span>
              {from.href ? (
                <>
                  From{" "}
                  <a href={from.href} target="_blank" rel="noopener noreferrer">
                    {from.label}
                  </a>
                </>
              ) : (
                from.label
              )}
              {from.at && (
                <>
                  , fetched <Time iso={from.at} as="relative" />
                </>
              )}
            </span>
          )}
          {adds && adds.length > 0 && <span>Adds {adds.join(" · ")}</span>}
          {source !== undefined && <span className="mono">{source}</span>}
        </RowSub>
        {notes}
      </RowMain>
      <span className={css.actions}>{actions}</span>
      {children}
    </Row>
  );
}

// PluginsRowNote is something said of a plugin under its row, in a tone
// where it's what's wrong.
export function PluginsRowNote({ tone, children }: { tone?: Tone; children: ReactNode }) {
  return <div className={cx(css.note, tone && css.toned, toneClass(tone))}>{children}</div>;
}
