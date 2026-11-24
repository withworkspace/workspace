import { type ReactNode, useId, useState } from "react";
import { Link, type To } from "react-router";
import type { Mission, RecipeInfo, RunDetail, Talk } from "../api";
import { Button, buttonClass } from "../ui/Button";
import { Chip } from "../ui/Chip";
import { Copyable } from "../ui/Copyable";
import { cx } from "../ui/cx";
import { Empty } from "../ui/Empty";
import { Icon } from "../ui/Icon";
import { Spinner } from "../ui/Spinner";
import { Status, type StatusInfo } from "../ui/Status";
import { Tab, Tabs } from "../ui/Tabs";
import { capital } from "../ui/text";
import { Time } from "../ui/Time";
import { RunOutput } from "./RunOutput";
import { RunPreview } from "./RunPreview";
import { bare, came, forWhom, listed, nameOfRef, runId, runStatus, type Said, startedBy } from "./runRows";
import css from "./RunPanel.module.css";

// RunPanel is one run of a place's, as its Runs tab shows the one picked:
// where it stands and what's done to it, then how and why it was run
// (who started it, from what, who it's for and so when it ends, what it
// needs, the step's check it was run for, the commit it ran at), then
// the service in a frame (Preview, while it runs) or all it's printed
// (Output). recipe is what its recipe says, if it has one here: what
// it's called, whether it's an app or a tool, and whether it's framed or
// opened in a browser tab of its own (render = "tab": Open, in place of
// the frame).
//
// What's done to it is the page's (onStop, onRunAgain, onClear: each
// left out where it can't be done), busy saying which is under way.
// back is the way to the list, where there's no room for both.

export type RunView = "preview" | "output";

export function RunPanel({
  r,
  recipe,
  missions,
  talks,
  busy,
  onStop,
  onRunAgain,
  onClear,
  back,
}: {
  r: RunDetail;
  recipe?: RecipeInfo;
  missions: Mission[];
  talks: Talk[];
  busy?: "stop" | "again" | "clear";
  onStop?: () => void;
  onRunAgain?: () => void;
  onClear?: () => void;
  back?: ReactNode;
}) {
  const live = r.state === "running";
  const task = r.type === "task";
  const inTab = recipe?.render === "tab";
  const framed = live && !task && !!r.link && !inTab;
  const [view, setView] = useState<RunView>(framed ? "preview" : "output");
  // A service that ends while it's framed has only its output left.
  const shown: RunView = view === "preview" && framed ? "preview" : "output";
  const panel = useId();
  const s = runStatus(r);
  return (
    <section className={css.panel} aria-label={`${recipe?.title || r.name}, ${s.words}`}>
      <Head
        back={back}
        title={recipe?.title}
        name={r.name}
        n={task ? r.n : undefined}
        status={{ ...s, words: capital(standing(r)) }}
        address={
          live &&
          !task &&
          r.link && (
            <a href={r.link} target="_blank" rel="noopener noreferrer" className={cx("mono", css.address)} title={`Open it in a tab of its own${r.link !== r.local ? `: also ${bare(r.local)}` : ""}`}>
              <span className={css.host}>{bare(r.link)}</span>
              <Icon name="arrow-up-right" size={11} />
            </a>
          )
        }
      >
          {live && inTab && r.link && (
            <a
              href={r.link}
              target="_blank"
              rel="noopener noreferrer"
              className={buttonClass({ variant: "primary", size: "small" })}
              title={r.up ? "Open it in a tab of its own, as its recipe says it's shown" : "Nothing's answering on its port yet: it opens once it does"}
            >
              <Icon name="external" size={11} /> Open
            </a>
          )}
          {onRunAgain && (
            <Button size="small" disabled={!!busy} onClick={onRunAgain} title={r.recipe ? `Run its ${r.recipe} recipe again, here` : "Run it again"}>
              {busy === "again" ? <Spinner size={11} /> : <Icon name="play" size={11} />} Run again
            </Button>
          )}
          {live && onStop && (
            <Button variant="ghost" size="small" disabled={!!busy} onClick={onStop} title={task ? "Stop it now: it's kept as stopped" : "End it now, and run its recipe's stop"}>
              {busy === "stop" ? <Spinner size={11} /> : <Icon name="stop" size={11} />} Stop
            </Button>
          )}
          {!live && onClear && (
            <Button
              variant="ghost"
              size="small"
              disabled={!!busy}
              onClick={onClear}
              title={task ? "Clear it away, with its output, its reports and the runs before it" : "Clear it away, its output with it"}
            >
              {busy === "clear" ? <Spinner size={11} /> : <Icon name="x" size={11} />} Clear
            </Button>
          )}
      </Head>

      <dl className={css.facts}>
        <Fact label="Started">
          <Time iso={r.started} as="relative" />
          <span className={css.quiet}>by {startedBy(r)}</span>
        </Fact>
        {r.check ? (
          <Fact label="For">
            <To s={checkSaid(r, missions)} />
          </Fact>
        ) : (
          live &&
          !task && (
            <Fact label="For">
              <To s={forWhom(r, missions, talks)} />
              {r.ends && <span className={css.quiet}>so it ends {r.ends}</span>}
            </Fact>
          )
        )}
        {live && task && r.ends && <Fact label="Ends">{capital(r.ends)}</Fact>}
        {!live && r.ended && (
          <Fact label="Ended">
            <Time iso={r.ended} as="relative" />
            {!task && <span className={css.quiet}>{r.state === "exited" ? `exit status ${r.status ?? 0}` : "outside ws"}</span>}
          </Fact>
        )}
        {task && r.commit && (
          <Fact label="Commit">
            <span className="mono" title={r.commit}>
              {r.commit.slice(0, 9)}
            </span>
            {r.dirty && <span className={css.quiet}>with changes not committed</span>}
          </Fact>
        )}
        <Fact label="Recipe">
          {r.recipe ? (
            <>
              <span className="mono" title="Its ws.repo.toml's recipe">
                [run.{task ? "task" : "service"}.{r.recipe}]
              </span>
              {recipe && !task && <span className={css.quiet}>{shownAs(recipe)}</span>}
            </>
          ) : (
            <span>None: a command, run as it is</span>
          )}
        </Fact>
        {(r.needs?.length ?? 0) > 0 && (
          <Fact label="Needs" wide>
            <Needs r={r} />
          </Fact>
        )}
        {live && (r.neededBy?.length ?? 0) > 0 && <Fact label="Needed by">{listed((r.neededBy ?? []).map(nameOfRef))}</Fact>}
        {(r.reports?.length ?? 0) > 0 && (
          <Fact label="Reports">
            <span className={css.reports}>
              {r.reports?.map((rp) => (
                <Chip key={rp.name} tone={rp.files > 0 || live ? "quiet" : "waiting"} title={`${rp.path}${rp.format ? `, ${rp.format}` : ""}`}>
                  <Icon name="file" size={10} /> {rp.name}
                  {rp.files === 0 && !live ? " not written" : ""}
                </Chip>
              ))}
            </span>
          </Fact>
        )}
        <Fact label="Command" wide>
          <Copyable text={r.command} className={cx("mono", css.command)}>
            {r.command}
          </Copyable>
        </Fact>
      </dl>

      <div className={css.views}>
        {framed ? (
          <Tabs value={shown} onChange={setView} controls={panel} className={css.tabs} aria-label={`What's shown of ${r.name}`}>
            <Tab value="preview" title="It, in a frame">
              <Icon name="eye" size={13} /> Preview
            </Tab>
            <Tab value="output" title="What it's printing, as it does">
              <Icon name="terminal" size={13} /> Output
            </Tab>
          </Tabs>
        ) : (
          // Nothing to frame (a task, a service ended or opened in a tab of
          // its own): its output is all there is to show.
          <h3 className={css.view}>
            <Icon name="terminal" size={13} /> Output
          </h3>
        )}
      </div>
      <div id={panel} role={framed ? "tabpanel" : undefined} aria-label={shown === "preview" ? `${r.name}, framed` : `${r.name}'s output`} className={css.body}>
        {shown === "preview" ? (
          <RunPreview link={r.link} title={recipe?.title || r.name} up={!!r.up} port={r.port} frameKey={`${r.pid}`} waiting={<RunOutput runRef={r.ref} live className={css.waitingLog} />} />
        ) : (
          <RunOutput runRef={runId(r)} live={live} className={css.log} />
        )}
      </div>
    </section>
  );
}

// RecipePanel is a service of a place's that isn't running, as its Runs
// tab shows it picked: what it's called, what it's for and how it's
// shown, its command, and Start (onStart, busy while it starts). pageTo
// is the worktree's tab of its own, for one with a page.
export function RecipePanel({ rc, busy, onStart, pageTo, back }: { rc: RecipeInfo; busy?: boolean; onStart?: () => void; pageTo?: To; back?: ReactNode }) {
  const called = rc.title || rc.name;
  const inTab = rc.render === "tab";
  return (
    <section className={css.panel} aria-label={`${called}, not running`}>
      <Head back={back} title={rc.title} name={rc.name} status={{ icon: "circle", tone: "quiet", words: "Not running" }}>
        {onStart && (
          <Button variant="primary" size="small" disabled={busy} onClick={onStart} title={`Start its ${rc.name} recipe here, for as long as you have it open here`}>
            {busy ? <Spinner size={11} /> : <Icon name="play" size={11} />} Start
          </Button>
        )}
      </Head>
      <dl className={css.facts}>
        <Fact label="Recipe">
          <span className="mono">[run.service.{rc.name}]</span>
          <span className={css.quiet}>{shownAs(rc)}</span>
        </Fact>
        <Fact label="Command" wide>
          <Copyable text={rc.command} className={cx("mono", css.command)}>
            {rc.command}
          </Copyable>
        </Fact>
      </dl>
      <div className={css.idle}>
        <Empty icon={inTab ? "external" : "play"} title={`${called} isn't running`} action={pageTo && <Link to={pageTo}>Its tab, {called}</Link>}>
          {inTab
            ? "Start it, and it opens in a tab of its own once it answers. It runs while you have this open, and 15 minutes after."
            : "Start it, and it's shown here, in a frame, once it answers. It runs while you have this open, and 15 minutes after."}
        </Empty>
      </div>
    </section>
  );
}

// Head is a panel's head: the way back, what it's called (its recipe's
// name beside, where that's another), which run of a task it is, where
// it stands, where it's reached, and what's done to it (its children).
function Head({ back, title, name, n, status, address, children }: { back?: ReactNode; title?: string; name: string; n?: number; status: StatusInfo; address?: ReactNode; children?: ReactNode }) {
  const named = !!title && title !== name;
  return (
    <header className={css.head}>
      {back}
      <h2 className={cx(!named && "mono", css.name)}>{named ? title : name}</h2>
      {named && <span className={cx("mono", css.n)}>{name}</span>}
      {!!n && <span className={css.n}>run {n}</span>}
      <Status s={status} className={css.status} />
      {address}
      <span className={css.spacer} />
      <span className={css.actions}>{children}</span>
    </header>
  );
}

// shownAs is what a service's recipe is for, and how it's shown.
function shownAs(rc: RecipeInfo): string {
  return `${rc.kind === "tool" ? "A tool" : "An app"}, ${rc.render === "tab" ? "opened in a tab of its own" : "framed here"}`;
}

// standing is where a run stands, in words: up on its port, what a task
// came to, how a service ended.
function standing(r: RunDetail): string {
  if (r.type === "task") return came(r);
  if (r.state === "running") return r.up ? `up on :${r.port}` : `starting on :${r.port}`;
  return r.state === "exited" ? `exited ${r.status ?? 0}` : "gone";
}

// checkSaid is the check of a mission's step a task was run for, going to
// the mission.
function checkSaid(r: RunDetail, missions: Mission[]): Said {
  const c = r.check!;
  const m = missions.find((x) => x.slug === c.mission);
  return { text: `check ${c.n} of step ${c.step} of ${m?.title || c.mission}`, to: `/missions/${c.mission}` };
}

function To({ s }: { s: Said }) {
  return s.to ? <Link to={s.to}>{s.text}</Link> : <span>{s.text}</span>;
}

// Fact is one of what's said of a run: a quiet label over it. wide takes
// the width of the row.
function Fact({ label, wide, children }: { label: string; wide?: boolean; children: ReactNode }) {
  return (
    <div className={cx(css.fact, wide && css.wide)}>
      <dt>{label}</dt>
      <dd>{children}</dd>
    </div>
  );
}

// Needs is what a run needs, each with what meets it: another run (its
// name and address), an address outside ws, or a repo's folder.
function Needs({ r }: { r: RunDetail }) {
  return (
    <ul className={css.needs}>
      {r.needs?.map((n) => (
        <li key={n.name}>
          <span className={css.need}>{n.name}</span>
          {n.folder ? (
            <Link to={n.worktree === "ground" ? `/files/${n.repo}` : `/files/${n.repo}/${n.worktree}`} className="mono" title={n.env ? `Handed to it as $${n.env}: ${n.dir}` : n.dir}>
              {n.repo} / {n.worktree}
            </Link>
          ) : (
            <a href={n.address} target="_blank" rel="noopener noreferrer" className="mono" title={n.env ? `Handed to it as $${n.env}: ${n.address}` : n.address}>
              {n.run ?? bare(n.address)}
            </a>
          )}
          {n.env && <span className={css.quiet}>as ${n.env}</span>}
          {n.mount && r.link !== r.local && <span className={css.quiet}>at {n.mount}</span>}
        </li>
      ))}
    </ul>
  );
}
