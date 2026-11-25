import type { Meta, StoryObj } from "@storybook/react-vite";
import { Agent, Field, Model, yours } from "./samples";
import { SettingsBlock } from "./SettingsBlock";
import { SettingsControl } from "./SettingsControl";
import { SettingsNote } from "./SettingsNote";
import { SettingsRole } from "./SettingsRole";
import { SettingsRoles } from "./SettingsRoles";

// The roles' table on Settings' Agents: a role with its agent and model
// as ws's defaults, one with both yours, one that follows another and
// says so, one that can't run and says why (its models couldn't be listed
// either, so its model's typed), and one with no model of its own. In a
// window 640px wide or less the head goes, and a role's cells go under it.

const meta: Meta = { title: "Settings/SettingsRoles" };
export default meta;

const cell = (control: React.ReactNode, mine?: boolean) => (
  <SettingsControl align="stretch">
    {control}
    {mine && yours("default sonnet", true)}
  </SettingsControl>
);

export const Roles: StoryObj = {
  render: () => (
    <div style={{ maxWidth: 824, margin: "0 auto", padding: 32 }}>
      <SettingsBlock>
        <SettingsRoles>
          <SettingsRole title="Crew" id="crew" about="Who does a mission's steps, each in a capsule." agent={cell(<Agent stretch />)} model={cell(<Model stretch />)} />
          <SettingsRole title="Briefing" id="briefing" about="The planner, which turns what you ask into a brief and a plan." agent={cell(<Agent stretch />, true)} model={cell(<Model stretch value="sonnet" />, true)} />
          <SettingsRole title="Team" id="team" about="The Team page's gists." note="It follows conversations' harness, on its small model, as none is set here." agent={cell(<Agent stretch />)} model={cell(<Model stretch />)} />
          <SettingsRole
            title="Judge"
            id="judge"
            about="Reads a step's work against its checks."
            problem="codex isn't on this machine: install it, or pick another agent."
            agent={cell(<Agent stretch value="codex" />, true)}
            model={cell(
              <>
                <Field value="" mono stretch placeholder="the agent's own default" />
                <SettingsNote left>Couldn't list codex's models: codex isn't on this machine</SettingsNote>
              </>,
            )}
          />
          <SettingsRole title="PR writer" id="pr_writer" agent={cell(<Agent stretch />)} />
        </SettingsRoles>
      </SettingsBlock>
    </div>
  ),
};
