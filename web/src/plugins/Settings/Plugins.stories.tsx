import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { Button } from "../../ui/Button";
import { Copyable } from "../../ui/Copyable";
import { GroupHead } from "../../ui/GroupHead";
import { Icon } from "../../ui/Icon";
import { Spinner } from "../../ui/Spinner";
import { PluginsAdd } from "./PluginsAdd";
import { PluginsFailed } from "./PluginsFailed";
import { PluginsList } from "./PluginsList";
import { PluginsNote } from "./PluginsNote";
import { PluginsRemove } from "./PluginsRemove";
import { PluginsRow, PluginsRowNote } from "./PluginsRow";
import { PluginsUpdates } from "./PluginsUpdates";
import { stateOf } from "../installed";
import { Text } from "../../ui/Text";

// Settings › Plugins, from its parts: the field a plugin's added from,
// what's out, what didn't work, and each plugin a row in every state it
// stands in (ready, a later release out, to be set up, failed, stopped,
// the team's and not installed here); and the dialog that asks before one
// is removed.

const meta: Meta = { title: "Plugins/Settings" };
export default meta;

const none = () => {};
const at = "2026-11-24T10:00:00Z";

const remove = (
  <Button variant="ghost" size="small" onClick={none} title="Stop it, and take it out of the workspace">
    <Icon name="x" size={11} /> Remove
  </Button>
);
const again = (
  <Button variant="ghost" size="small" onClick={none} title="Fetch its latest release again">
    <Icon name="rotate" size={11} /> Update
  </Button>
);

function Installed() {
  return (
    <PluginsList>
      <PluginsRow
        kind="ready"
        icon="pencil"
        name="sketch"
        to="/x/sketch/"
        version="v0.6.1"
        state={stateOf({ state: "ready" })}
        about="Sketches: drawings of a flow or a system, kept with the work."
        from={{ label: "withworkspace/plugin-excalidraw", href: "https://github.com/withworkspace/plugin-excalidraw", at }}
        adds={["a view, Sketch", "ws sketch", "a guide for agents", "SKETCH-…"]}
        actions={
          <>
            {again}
            {remove}
          </>
        }
      />
      <PluginsRow
        kind="ready"
        icon="box"
        name="jira-cr"
        to="/x/jira-cr/"
        version="v0.4.0"
        state={stateOf({ state: "ready" })}
        out={{ label: "v0.5.0 is out", href: "https://github.com/withworkspace/plugin-jira-cr/releases/tag/v0.5.0" }}
        from={{ label: "withworkspace/plugin-jira-cr", href: "https://github.com/withworkspace/plugin-jira-cr", at }}
        adds={["a view, Board", "ws jira"]}
        actions={
          <>
            <Button variant="primary" size="small" onClick={none} title="Update it to v0.5.0">
              <Icon name="arrow-up" size={11} /> Update to v0.5.0
            </Button>
            {remove}
          </>
        }
      />
      <PluginsRow
        icon="box"
        name="coverage"
        to="/x/coverage/"
        state={stateOf({ state: "setup" })}
        from={{ label: "~/Projects/plugin-coverage" }}
        adds={["a view, Coverage"]}
        notes={
          <PluginsRowNote tone="attention">
            It can't be used until these are set: <Text mono>token, project</Text>. In a terminal:{" "}
            <Copyable text="ws plugin config coverage --ask">ws plugin config coverage --ask</Copyable>
          </PluginsRowNote>
        }
        actions={remove}
      />
      <PluginsRow
        kind="failed"
        icon="alert"
        name="frame-fixture"
        state={stateOf({ state: "failed" })}
        from={{ label: "~/Projects/workspace/testdata/plugins/frame-fixture" }}
        notes={
          <>
            <PluginsRowNote tone="danger">
              It exited before it said what it adds. Its log: <Copyable text="ws plugin logs frame-fixture">ws plugin logs frame-fixture</Copyable>
            </PluginsRowNote>
            <PluginsRowNote>Left out: a view called Settings, which is ws's</PluginsRowNote>
          </>
        }
        actions={remove}
      />
      <PluginsRow
        icon="box"
        name="relman"
        version="v0.1.0"
        state={stateOf({ state: "stopped" })}
        from={{ label: "withworkspace/plugin-relman", href: "https://github.com/withworkspace/plugin-relman", at }}
        notes={
          <>
            <PluginsRowNote>It last logged: the release calendar didn't answer</PluginsRowNote>
            <PluginsRowNote>Couldn't look for a later release: GitHub said 404</PluginsRowNote>
          </>
        }
        actions={remove}
      />
    </PluginsList>
  );
}

function Wanted() {
  return (
    <PluginsList>
      <PluginsRow
        kind="wanted"
        icon="circle-dashed"
        name="standup"
        version="v1.2.0"
        source="withworkspace/plugin-standup@v1.2.0"
        actions={
          <Button size="small" onClick={none} title="Install it from withworkspace/plugin-standup@v1.2.0">
            <Icon name="plus" size={11} /> Add
          </Button>
        }
      />
      <PluginsRow
        kind="wanted"
        icon="circle-dashed"
        name="adding"
        source="withworkspace/plugin-adding"
        actions={
          <Button size="small" disabled onClick={none}>
            <Spinner size={11} /> Adding…
          </Button>
        }
      />
    </PluginsList>
  );
}

// The whole section, as Settings draws it.
export const Section: StoryObj = {
  render: function Render() {
    const [from, setFrom] = useState("");
    return (
      <div style={{ maxWidth: 900, padding: 24 }}>
        <PluginsAdd value={from} onChange={setFrom} busy={false} onAdd={none} />
        <PluginsUpdates out={1} checked={at} looking={false} onLook={none} />
        <section>
          <GroupHead count={5}>Installed</GroupHead>
          <Installed />
        </section>
        <section>
          <GroupHead count={2}>The team's, not installed here</GroupHead>
          <PluginsNote>
            In <code>ws.toml</code>, under <code>[plugins]</code>.
          </PluginsNote>
          <Wanted />
        </section>
        <PluginsNote>
          The daemon isn't looking for updates by itself: <a href="/settings/about#updates">Look for updates</a> is off.
        </PluginsNote>
      </div>
    );
  },
};

// The field, empty, typed in, and adding.
export const Add: StoryObj = {
  render: () => (
    <div style={{ display: "grid", gap: 8, maxWidth: 900, padding: 24 }}>
      <PluginsAdd value="" onChange={none} busy={false} onAdd={none} />
      <PluginsAdd value="withworkspace/plugin-excalidraw" onChange={none} busy={false} onAdd={none} />
      <PluginsAdd value="withworkspace/plugin-excalidraw" onChange={none} busy onAdd={none} />
    </div>
  ),
};

// What's out: updates, none, never looked, and looking now.
export const Updates: StoryObj = {
  render: () => (
    <div style={{ display: "grid", gap: 8, maxWidth: 900, padding: 24 }}>
      <PluginsUpdates out={2} checked={at} looking={false} onLook={none} />
      <PluginsUpdates out={1} checked={at} looking={false} onLook={none} />
      <PluginsUpdates out={0} checked={at} looking={false} onLook={none} />
      <PluginsUpdates out={0} looking={false} onLook={none} />
      <PluginsUpdates out={0} checked={at} looking onLook={none} />
    </div>
  ),
};

// What didn't work: with what a build printed, and without.
export const Failed: StoryObj = {
  render: () => (
    <div style={{ display: "grid", gap: 12, maxWidth: 900, padding: 24 }}>
      <PluginsFailed
        what="Couldn't add withworkspace/plugin-relman"
        error="Its build failed."
        output={"go: downloading github.com/withworkspace/plugin v0.12.0\n# github.com/withworkspace/plugin-relman\n./main.go:14:2: undefined: plugin.Board\n"}
        onDismiss={none}
      />
      <PluginsFailed what="Couldn't remove sketch" error="It's still running a command." onDismiss={none} />
    </div>
  ),
};

// The rows alone, in each state.
export const Rows: StoryObj = {
  render: () => (
    <div style={{ maxWidth: 900, padding: 24 }}>
      <Installed />
      <Wanted />
    </div>
  ),
};

// Asked before one's removed: one whose things have names, and one whose haven't.
export const Remove: StoryObj = {
  render: () => <PluginsRemove name="sketch" names={["SKETCH-…"]} onClose={none} onRemove={none} />,
};

export const RemoveNoNames: StoryObj = {
  render: () => <PluginsRemove name="relman" onClose={none} onRemove={none} />,
};
