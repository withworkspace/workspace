import { artifactViewKeys } from "./keys";
import type { Command } from "../keymap/commands";
import { keyOf, useKeymap } from "../keymap/keymap";
import type { ReactNode } from "react";
import { Link } from "react-router";
import { Button, buttonClass } from "../ui/Button";
import { Icon } from "../ui/Icon";
import { keyProps } from "../ui/keyHint";
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
  // Each key as it is now, drawn after its words.
  const map = useKeymap();
  const kbd = (c: Command) => {
    const k = keyOf(c, map);
    return k ? <Keys k={k} /> : null;
  };
  return (
    <ArtifactToolsEnd>
      <Button variant="ghost" size="small" onClick={onReload} disabled={!runnable || source} title="Run again" keys={artifactViewKeys.reload}>
        <Icon name="rotate" size={13} /> Reload {kbd(artifactViewKeys.reload)}
      </Button>
      <Button variant="ghost" size="small" onClick={onStop} disabled={!runnable || source} title={stopped ? "Run again" : "Unload: a script that loops stops"} keys={artifactViewKeys.stop}>
        <Icon name={stopped ? "play" : "stop"} size={13} /> {stopped ? "Run" : "Stop"} {kbd(artifactViewKeys.stop)}
      </Button>
      <ToggleButton variant="ghost" size="small" pressed={source} onChange={onSource} disabled={!runnable} title={manyFiles ? "Source" : "HTML"} keys={artifactViewKeys.source}>
        <Icon name="code" size={13} /> Source {kbd(artifactViewKeys.source)}
      </ToggleButton>
      {tweaks}
      {compare ? (
        <Link className={buttonClass({ variant: "ghost" })} to={compare.to} {...keyProps(keyOf(artifactViewKeys.compare, map), compare.title)}>
          <Icon name="split" size={13} /> Compare {kbd(artifactViewKeys.compare)}
        </Link>
      ) : runnable ? (
        <Button variant="ghost" size="small" unavailable="There is only one version">
          <Icon name="split" size={13} /> Compare
        </Button>
      ) : null}
      {alone ? (
        <a className={buttonClass({ variant: "ghost" })} href={alone} target="_blank" rel="noopener noreferrer" {...keyProps(keyOf(artifactViewKeys.newTab, map), "In new tab, still sandboxed")}>
          <Icon name="external" size={13} /> Open in new tab
        </a>
      ) : null}
      {keep && (
        <Button variant="ghost" size="small" onClick={keep.onPromote} title={keep.promoted ? "Choose another version" : "Keep permanently"} keys={artifactViewKeys.promote}>
          <Icon name="check-circle" size={13} /> {keep.promoted ? "Promoted" : "Promote"} {kbd(artifactViewKeys.promote)}
        </Button>
      )}
      {keep && (
        <Button variant="ghost" size="small" onClick={keep.onDiscard} title="Remove from list: can be restored">
          <Icon name="ban" size={13} /> Discard
        </Button>
      )}
      {onRestore && (
        <Button variant="ghost" size="small" onClick={onRestore} title="Undo discard">
          <Icon name="rotate" size={13} /> Restore
        </Button>
      )}
    </ArtifactToolsEnd>
  );
}
