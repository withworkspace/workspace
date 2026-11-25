import { Link } from "react-router";
import type { Names } from "../api";
import { Callout } from "../ui/Callout";
import { Icon } from "../ui/Icon";
import { bare } from "./runRows";
import { RunsPageNames } from "./RunsPageNames";
import { Text, textClass } from "../ui/Text";

// RunsNames is where the workspace's names stand, or how to have them.
export function RunsNames({ names: n }: { names: Names }) {
  const example = `web.<capsule>.<repo>.${n.want}.localhost`;
  if (!n.on) {
    return (
      <Callout tone="quiet" icon="link" title="Runs are reached at 127.0.0.1 and a port">
        Turn on <Link to="/settings/runs">local names</Link> and each gets a name that stays put, <code>{example}</code>, with this page at <code>{n.want}.localhost</code>.
        Browsers send any <code>.localhost</code> to this machine by themselves.
      </Callout>
    );
  }
  let state: React.ReactNode;
  if (!n.port) state = <>No router is serving yet: this workspace's daemon becomes it within a few seconds.</>;
  else if (!n.name) state = <>The daemon hasn't written this workspace's routes yet: it does within a few seconds.</>;
  else
    state = (
      <>
        <a href={n.web} target="_blank" rel="noopener noreferrer" className={textClass("mono")}>
          {bare(n.web ?? "")}
          <Icon name="arrow-up-right" size={11} />
        </a>
        <Text dim> and each run below it, as {example.replace(`.${n.want}.`, `.${n.name}.`)}</Text>
      </>
    );
  return (
    <RunsPageNames
      names={state}
      note={n.name && n.name !== n.want ? <>{n.want} was another workspace's first: this one is {n.name} while that one's daemon runs.</> : undefined}
      router={
        n.port ? (
          <>
            On :{n.port}
            {n.fallback && <Text dim>, as 80 is another program's or kept by the system</Text>}
            {n.heldBy ? <Text dim>, held by the daemon at pid {n.heldBy}</Text> : null}
            <Text dim>. One serves every workspace on this machine.</Text>
          </>
        ) : (
          <Text dim>None running</Text>
        )
      }
    />
  );
}
