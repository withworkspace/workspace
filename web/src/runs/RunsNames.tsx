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
      <Callout tone="quiet" icon="link" title="Runs use 127.0.0.1 and a port">
        Turn on <Link to="/settings/runs">local names</Link> to give each a stable name, like{" "}
        <code>{example}</code>, and this page <code>{n.want}.localhost</code>. No setup needed: browsers send{" "}
        <code>.localhost</code> to this machine.
      </Callout>
    );
  }
  let state: React.ReactNode;
  if (!n.port) state = <>Router starting: ready in a few seconds.</>;
  else if (!n.name) state = <>Setting up routes: ready in a few seconds.</>;
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
      note={n.name && n.name !== n.want ? <>{n.want} is taken by another workspace: this one is {n.name} while that one runs.</> : undefined}
      router={
        n.port ? (
          <>
            On :{n.port}{n.fallback && <Text dim>, as port 80 is taken</Text>}
            {n.heldBy ? <Text dim>, held by Workspace at pid {n.heldBy}</Text> : null}
            <Text dim>. Shared by every workspace on this machine.</Text>
          </>
        ) : (
          <Text dim>None running</Text>
        )
      }
    />
  );
}
