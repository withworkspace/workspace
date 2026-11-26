import type { ReactNode } from "react";
import { Link, type To } from "react-router";
import type { RecipeInfo } from "../api";
import { Button } from "../ui/Button";
import { Icon } from "../ui/Icon";
import { Spinner } from "../ui/Spinner";
import { shownAs } from "./panelWords";
import { RunPanelAside, RunPanelCommand, RunPanelFact, RunPanelFacts } from "./RunPanelFacts";
import { RunPanelFrame } from "./RunPanelFrame";
import { RunPanelHead } from "./RunPanelHead";
import { RunPanelIdle } from "./RunPanelIdle";
import { Text } from "../ui/Text";

// RecipePanel is a service of a place's that isn't running, as its Runs
// tab shows it picked: what it's called, what it's for and how it's
// shown, its command, and Start (onStart, busy while it starts). pageTo
// is the worktree's tab of its own, for one with a page.
export function RecipePanel({ rc, busy, onStart, pageTo, back }: { rc: RecipeInfo; busy?: boolean; onStart?: () => void; pageTo?: To; back?: ReactNode }) {
  const called = rc.title || rc.name;
  const inTab = rc.render === "tab";
  return (
    <RunPanelFrame label={`${called}, not running`}>
      <RunPanelHead back={back} title={rc.title} name={rc.name} status={{ icon: "circle", tone: "quiet", words: "Not running" }}>
        {onStart && (
          <Button variant="primary" size="small" disabled={busy} onClick={onStart} title={`Start its ${rc.name} recipe here, for as long as you have it open here`}>
            {busy ? <Spinner size={11} /> : <Icon name="play" size={11} />} Start
          </Button>
        )}
      </RunPanelHead>
      <RunPanelFacts>
        <RunPanelFact label="Recipe">
          <Text mono>[run.service.{rc.name}]</Text>
          <RunPanelAside>{shownAs(rc)}</RunPanelAside>
        </RunPanelFact>
        <RunPanelFact label="Command" wide>
          <RunPanelCommand command={rc.command} />
        </RunPanelFact>
      </RunPanelFacts>
      <RunPanelIdle icon={inTab ? "external" : "play"} title={`${called} isn't running`} action={pageTo && <Link to={pageTo}>Its tab, {called}</Link>}>
        {inTab
          ? "Start it, and it opens in a tab of its own once it answers. It runs while you have this open, and 15 minutes after."
          : "Start it, and it's shown here, in a frame, once it answers. It runs while you have this open, and 15 minutes after."}
      </RunPanelIdle>
    </RunPanelFrame>
  );
}
