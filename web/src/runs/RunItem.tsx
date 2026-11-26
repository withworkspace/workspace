import type { ReactNode } from "react";
import { Link, type To } from "react-router";
import type { RunDetail } from "../api";
import { cx } from "../ui/cx";
import { Row, RowLine, RowMain, RowMeta, RowSub, RowTitle, rowLinkClass, rowStatusClass } from "../ui/Rows";
import { Status, type StatusInfo } from "../ui/Status";
import { Time } from "../ui/Time";
import { came, runStatus, type ServiceRow, startedBy } from "./runRows";
import css from "./RunItem.module.css";
import { Text, textClass } from "../ui/Text";

// A place's runs, a row each in its list of them: a service (TaskItem's
// sibling, ServiceItem), running or not, or one run of a task. Each goes
// to what it is, shown beside the list (to); picked is the one shown.

// ServiceItem is one of a place's services: what it's called (its
// recipe's name beside, where that's another), where its run stands and
// who started it, or that it isn't running; and how long ago its run
// started or ended.
export function ServiceItem({ s, to, picked }: { s: ServiceRow; to: To; picked: boolean }) {
  const r = s.run;
  const title = s.recipe?.title;
  const tab = s.recipe?.render === "tab";
  return (
    <Item
      to={to}
      picked={picked}
      over={r?.state !== "running"}
      status={r ? runStatus(r) : { icon: "circle", tone: "quiet", words: "Not running" }}
      title={title || s.name}
      name={title && title !== s.name ? s.name : undefined}
      sub={r ? `${at(r)} · ${startedBy(r)}` : tab ? "not running · opens in new tab" : "not running"}
      when={r && (r.state === "running" ? r.started : (r.ended ?? r.started))}
    />
  );
}

// TaskItem is one run of a task: what it came to, who ran it, and when.
export function TaskItem({ r, title, to, picked }: { r: RunDetail; title?: string; to: To; picked: boolean }) {
  const live = r.state === "running";
  return (
    <Item
      to={to}
      picked={picked}
      over={!live}
      status={runStatus(r)}
      title={title || r.name}
      name={title && title !== r.name ? r.name : undefined}
      n={r.n}
      sub={`${came(r)} · ${startedBy(r)}`}
      when={live ? r.started : (r.ended ?? r.started)}
    />
  );
}

function Item({ to, picked, over, status, title, name, n, sub, when }: { to: To; picked: boolean; over: boolean; status: StatusInfo; title: string; name?: string; n?: number; sub: ReactNode; when?: string }) {
  return (
    <Row picked={picked} className={cx(over && css.over)}>
      <Link to={to} replace className={rowLinkClass(css.link)} aria-current={picked ? "true" : undefined}>
        <Status s={status} iconOnly className={rowStatusClass()} />
        <RowMain>
          <RowLine>
            <RowTitle className={cx(!name && textClass("mono"), css.title)}>{title}</RowTitle>
            {name && <Text mono className={css.aside}>{name}</Text>}
            {!!n && <span className={css.aside}>#{n}</span>}
          </RowLine>
          <RowSub className={css.sub}>{sub}</RowSub>
        </RowMain>
        {when && (
          <RowMeta className={css.when}>
            <Time iso={when} as="age" />
          </RowMeta>
        )}
      </Link>
    </Row>
  );
}

// at is where a service's run is, or how it ended, in a few words.
function at(r: RunDetail): string {
  if (r.state === "running") return r.up ? `up on :${r.port}` : `starting on :${r.port}`;
  return r.state === "exited" ? `exited ${r.status ?? 0}` : "gone";
}
