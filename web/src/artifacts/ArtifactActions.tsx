import type { ReactNode } from "react";
import { Link } from "react-router";
import { Button, buttonClass } from "../ui/Button";
import { Icon } from "../ui/Icon";
import { Keys } from "../ui/Keys";
import { ToggleButton } from "../ui/ToggleButton";
import { ArtifactToolsEnd } from "./ArtifactTools";

type ArtifactActionsProps = {
  // There's a version to run: with none, only what's done to the artifact
  // itself is offered.
  runnable: boolean;
  // Its source is shown, in place of its frame; it's stopped.
  source: boolean;
  stopped: boolean;
  // It has more files than its page: what Source says it shows.
  manyFiles?: boolean;
  onReload: () => void;
  onStop: () => void;
  onSource: (source: boolean) => void;
  // The button that shows its tweaks, where its page has some.
  tweaks?: ReactNode;
  // What Compare opens, and says of it; none, where there's nothing to
  // compare it with.
  compare?: { to: string; title: string };
  // Where it runs alone, in a tab of its own.
  alone?: string;
  // It can be kept or discarded: whether it's promoted already.
  keep?: { promoted: boolean; onPromote: () => void; onDiscard: () => void };
  // It's discarded, and can be put back.
  onRestore?: () => void;
};

// ArtifactActions is what's pressed on an artifact's page, at the end of
// its tools: run it again, stop it, read its source, its tweaks, compare
// it, open it alone, and keep it or discard it.
export function ArtifactActions({ runnable, source, stopped, manyFiles, onReload, onStop, onSource, tweaks, compare, alone, keep, onRestore }: ArtifactActionsProps) {
  return (
    <ArtifactToolsEnd>
      <Button variant="ghost" size="small" onClick={onReload} disabled={!runnable || source} title="Run it again">
        <Icon name="rotate" size={13} /> Reload <Keys k="r" />
      </Button>
      <Button variant="ghost" size="small" onClick={onStop} disabled={!runnable || source} title={stopped ? "Run it again" : "Unload it: a script that loops stops"}>
        <Icon name={stopped ? "play" : "stop"} size={13} /> {stopped ? "Run" : "Stop"} <Keys k="x" />
      </Button>
      <ToggleButton variant="ghost" size="small" pressed={source} onChange={onSource} disabled={!runnable} title={manyFiles ? "Its files, as they were written" : "Its HTML"}>
        <Icon name="code" size={13} /> Source <Keys k="s" />
      </ToggleButton>
      {tweaks}
      {compare ? (
        <Link className={buttonClass({ variant: "ghost" })} to={compare.to} title={compare.title}>
          <Icon name="split" size={13} /> Compare <Keys k="c" />
        </Link>
      ) : runnable ? (
        <Button variant="ghost" size="small" unavailable="It has one version, and there's no other artifact to compare it with">
          <Icon name="split" size={13} /> Compare
        </Button>
      ) : null}
      {alone ? (
        <a className={buttonClass({ variant: "ghost" })} href={alone} target="_blank" rel="noopener noreferrer" title="In a tab of its own, still sandboxed, as it's set here">
          <Icon name="external" size={13} /> Open alone
        </a>
      ) : null}
      {keep && (
        <Button variant="ghost" size="small" onClick={keep.onPromote} title={keep.promoted ? "Choose another version, or say what it went into now" : "Keep it for good: it expires no more"}>
          <Icon name="check-circle" size={13} /> {keep.promoted ? "Promoted" : "Promote"} <Keys k="p" />
        </Button>
      )}
      {keep && (
        <Button variant="ghost" size="small" onClick={keep.onDiscard} title="Take it off the list, saying why: it can be restored">
          <Icon name="ban" size={13} /> Discard
        </Button>
      )}
      {onRestore && (
        <Button variant="ghost" size="small" onClick={onRestore} title="Put it back, as it was">
          <Icon name="rotate" size={13} /> Restore
        </Button>
      )}
    </ArtifactToolsEnd>
  );
}
