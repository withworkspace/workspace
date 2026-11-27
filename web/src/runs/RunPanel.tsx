import { type ReactNode, useId, useState } from "react";
import { Link } from "react-router";
import type { Mission, RecipeInfo, RunDetail, Talk } from "../api";
import { Button, buttonClass } from "../ui/Button";
import { Chip } from "../ui/Chip";
import { Icon } from "../ui/Icon";
import { Spinner } from "../ui/Spinner";
import { capital } from "../ui/strings";
import { Time } from "../ui/Time";
import { checkSaid, shownAs, standing } from "./panelWords";
import { RunOutput } from "./RunOutput";
import { RunPanelBody, runPanelLogClass, runPanelWaitingLogClass } from "./RunPanelBody";
import { RunPanelAside, RunPanelCommand, RunPanelFact, RunPanelFacts, RunPanelReports } from "./RunPanelFacts";
import { RunPanelFrame } from "./RunPanelFrame";
import { RunPanelHead } from "./RunPanelHead";
import { RunPanelNeed, RunPanelNeeds } from "./RunPanelNeeds";
import { RunPanelViews } from "./RunPanelViews";
import { RunPreview } from "./RunPreview";
import { bare, forWhom, listed, nameOfRef, runId, runStatus, startedBy, took } from "./runRows";
import type { RunView } from "./runView";
import { SaidTo } from "./SaidTo";
import { Text, textClass } from "../ui/Text";

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
// left out where it can't be done), busy saying which is under way. A
// reused task's Run again is Run fresh: asked again, it would only be
// reused again, so onRunAgain is given whether to run it fresh.
// back is the way to the list, where there's no room for both.
//
// It knows which view is shown, and maps the run onto the RunPanel…
// parts (stories: Runs/RunPanel).
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
  onRunAgain?: (fresh?: boolean) => void;
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
    <RunPanelFrame label={`${recipe?.title || r.name}, ${s.words}`}>
      <RunPanelHead
        back={back}
        title={recipe?.title}
        name={r.name}
        n={task ? r.n : undefined}
        status={{ ...s, words: capital(standing(r)) }}
        address={live && !task && r.link ? { href: r.link, host: bare(r.link), title: `Open it in a tab of its own${r.link !== r.local ? `: also ${bare(r.local)}` : ""}` } : undefined}
      >
        {live && inTab && r.link && (
          <a
            href={r.link}
            target="_blank"
            rel="noopener noreferrer"
            className={buttonClass({ variant: "primary", size: "small" })}
            title={r.up ? "Open in new tab" : "Not ready yet"}
          >
            <Icon name="external" size={11} /> Open
          </a>
        )}
        {onRunAgain && (
          <Button
            size="small"
            disabled={!!busy}
            onClick={() => onRunAgain(!!r.reused)}
            title={r.reused ? "Run it, though a pass with the same inputs is kept" : r.recipe ? `Run ${r.recipe} again` : "Run again"}
          >
            {busy === "again" ? <Spinner size={11} /> : <Icon name="play" size={11} />} {r.reused ? "Run fresh" : "Run again"}
          </Button>
        )}
        {live && onStop && (
          <Button variant="ghost" size="small" disabled={!!busy} onClick={onStop} title={task ? "Stop now" : "Stop, and run recipe's stop"}>
            {busy === "stop" ? <Spinner size={11} /> : <Icon name="stop" size={11} />} Stop
          </Button>
        )}
        {!live && onClear && (
          <Button
            variant="ghost"
            size="small"
            disabled={!!busy}
            onClick={onClear}
            title={task ? "Clear, with output, reports and earlier runs" : "Clear, with output"}
          >
            {busy === "clear" ? <Spinner size={11} /> : <Icon name="x" size={11} />} Clear
          </Button>
        )}
      </RunPanelHead>

      <RunPanelFacts>
        <RunPanelFact label="Started">
          <Time iso={r.started} as="relative" />
          <RunPanelAside>by {startedBy(r)}</RunPanelAside>
        </RunPanelFact>
        {r.check ? (
          <RunPanelFact label="For">
            <SaidTo s={checkSaid(r, missions)} span />
          </RunPanelFact>
        ) : (
          live &&
          !task && (
            <RunPanelFact label="For">
              <SaidTo s={forWhom(r, missions, talks)} span />
              {r.ends && <RunPanelAside>ends {r.ends}</RunPanelAside>}
            </RunPanelFact>
          )
        )}
        {live && task && r.ends && <RunPanelFact label="Ends">{capital(r.ends)}</RunPanelFact>}
        {!live && r.ended && (
          <RunPanelFact label="Ended">
            <Time iso={r.ended} as="relative" />
            {!task && <RunPanelAside>{r.state === "exited" ? `exit status ${r.status ?? 0}` : "outside Workspace"}</RunPanelAside>}
          </RunPanelFact>
        )}
        {task && r.reused && (
          <RunPanelFact label="Reused">
            <Text mono>{r.reused.from}</Text>
            <RunPanelAside>which took {took(r.reused.took) || "under 0.1s"}: its inputs are the same</RunPanelAside>
          </RunPanelFact>
        )}
        {task && r.commit && (
          <RunPanelFact label="Commit">
            <Text mono title={r.commit}>
              {r.commit.slice(0, 9)}
            </Text>
            {r.dirty && <RunPanelAside>with uncommitted changes</RunPanelAside>}
          </RunPanelFact>
        )}
        <RunPanelFact label="Recipe">
          {r.recipe ? (
            <>
              <Text mono title="Recipe in ws.repo.toml">
                [run.{task ? "task" : "service"}.{r.recipe}]
              </Text>
              {recipe && !task && <RunPanelAside>{shownAs(recipe)}</RunPanelAside>}
            </>
          ) : (
            <span>None: plain command</span>
          )}
        </RunPanelFact>
        {(r.needs?.length ?? 0) > 0 && (
          <RunPanelFact label="Needs" wide>
            {/* Each with what meets it: another run (its name and
                address), an address outside ws, or a repo's folder. */}
            <RunPanelNeeds>
              {r.needs?.map((n) => (
                <RunPanelNeed key={n.name} name={n.name}>
                  {n.folder ? (
                    <Link to={n.worktree === "ground" ? `/files/${n.repo}` : `/files/${n.repo}/${n.worktree}`} className={textClass("mono")} title={n.env ? `Passed as $${n.env}: ${n.dir}` : n.dir}>
                      {n.repo} / {n.worktree}
                    </Link>
                  ) : (
                    <a href={n.address} target="_blank" rel="noopener noreferrer" className={textClass("mono")} title={n.env ? `Passed as $${n.env}: ${n.address}` : n.address}>
                      {n.run ?? bare(n.address)}
                    </a>
                  )}
                  {n.env && <RunPanelAside>as ${n.env}</RunPanelAside>}
                  {n.mount && r.link !== r.local && <RunPanelAside>at {n.mount}</RunPanelAside>}
                </RunPanelNeed>
              ))}
            </RunPanelNeeds>
          </RunPanelFact>
        )}
        {live && (r.neededBy?.length ?? 0) > 0 && <RunPanelFact label="Needed by">{listed((r.neededBy ?? []).map(nameOfRef))}</RunPanelFact>}
        {(r.reports?.length ?? 0) > 0 && (
          <RunPanelFact label="Reports">
            <RunPanelReports>
              {r.reports?.map((rp) => (
                <Chip key={rp.name} tone={rp.files > 0 || live ? "quiet" : "waiting"} title={`${rp.path}${rp.format ? `, ${rp.format}` : ""}`}>
                  <Icon name="file" size={10} /> {rp.name}
                  {rp.files === 0 && !live ? " not written" : ""}
                </Chip>
              ))}
            </RunPanelReports>
          </RunPanelFact>
        )}
        <RunPanelFact label="Command" wide>
          <RunPanelCommand command={r.command} />
        </RunPanelFact>
      </RunPanelFacts>

      <RunPanelViews framed={framed} shown={shown} onChange={setView} controls={panel} name={r.name} />
      <RunPanelBody id={panel} tabpanel={framed} label={shown === "preview" ? `${r.name} preview` : `${r.name}'s output`}>
        {shown === "preview" ? (
          <RunPreview
            link={r.link}
            title={recipe?.title || r.name}
            up={!!r.up}
            port={r.port}
            frameKey={`${r.pid}`}
            waiting={<RunOutput runRef={r.ref} live className={runPanelWaitingLogClass} />}
          />
        ) : (
          <RunOutput runRef={runId(r)} live={live} className={runPanelLogClass} />
        )}
      </RunPanelBody>
    </RunPanelFrame>
  );
}
