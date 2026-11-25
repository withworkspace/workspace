import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { PageHead } from "../ui/PageHead";
import { Switch } from "../ui/Switch";
import { Agent, Field, none, Number, team, yours } from "./samples";
import { SettingsBlock } from "./SettingsBlock";
import { SettingsFind } from "./SettingsFind";
import { SettingsLayout } from "./SettingsLayout";
import { SettingsMarked } from "./SettingsMarked";
import { SettingsNav, SettingsNavHeading } from "./SettingsNav";
import { SettingsNavLink } from "./SettingsNavLink";
import { SettingsRow } from "./SettingsRow";
import { SettingsRows } from "./SettingsRows";
import { SettingsSide } from "./SettingsSide";

// The Settings page from its parts: the list of sections beside the one
// you're in, with how many your file sets and the dot that says it has
// keys ws doesn't read; and finding, where each section says how many it
// has that say what's typed, those with none stand back, and the rows
// found are under their sections' names. Narrow the window and the list
// runs along the top.

const meta: Meta = { title: "Settings/Settings" };
export default meta;

function Side({ query, setQuery, at }: { query: string; setQuery: (q: string) => void; at?: string }) {
  const finding = query.trim() !== "";
  const link = (id: string, icon: Parameters<typeof SettingsNavLink>[0]["icon"], title: string, hits = 0, more: Partial<Parameters<typeof SettingsNavLink>[0]> = {}) => (
    <SettingsNavLink to={`/settings/${id}`} icon={icon} current={!finding && id === at} faded={finding && hits === 0} count={finding && hits > 0 ? hits : undefined} {...(finding ? {} : more)}>
      {title}
    </SettingsNavLink>
  );
  return (
    <SettingsSide title="Settings" find={<SettingsFind value={query} onChange={setQuery} onClear={() => setQuery("")} />}>
      <SettingsNav>
        <SettingsNavHeading>Interface</SettingsNavHeading>
        {link("appearance", "sun", "Appearance")}
        {link("notifications", "bell", "Notifications")}
        <SettingsNavHeading>How work goes</SettingsNavHeading>
        {link("agents", "terminal", "Agents", 2)}
        {link("missions", "flag", "Missions", 1)}
        {link("repos", "branch", "Repos")}
        <SettingsNavHeading>This workspace</SettingsNavHeading>
        {link("yours", "pencil", "Set by you", 0, { count: 7, countTitle: "7 set by you", unread: "A key ws doesn't read" })}
        {link("about", "info", "About")}
      </SettingsNav>
    </SettingsSide>
  );
}

function Toggle({ on: was }: { on: boolean }) {
  const [on, setOn] = useState(was);
  return <Switch on={on} onChange={setOn} aria-label="A switch" />;
}

function Page({ typed = "" }: { typed?: string }) {
  const [query, setQuery] = useState(typed);
  const words = query.toLowerCase().split(/\s+/).filter(Boolean);
  const m = (text: string) => <SettingsMarked text={text} words={words} />;
  return (
    <div style={{ height: "100vh", overflow: "auto" }}>
      <SettingsLayout side={<Side query={query} setQuery={setQuery} at="missions" />}>
        {words.length > 0 ? (
          <>
            <PageHead title={<>“{query.trim()}”</>}>
              <span className="dim">3 settings</span>
            </PageHead>
            <SettingsBlock title={<a href="#agents">Agents</a>}>
              <SettingsRows>
                <SettingsRow label={m("Agent")} tomlKey={m("[crew] harness")} crumb="Crew" about={m("The agent CLI crew run in.")} source={yours("default Cursor")}>
                  <Agent />
                </SettingsRow>
                <SettingsRow label={m("Model")} tomlKey={m("[crew] model")} crumb="Crew" about={m("The model crew are given, in their agent's own naming.")}>
                  <Field value="haiku" mono />
                </SettingsRow>
              </SettingsRows>
            </SettingsBlock>
            <SettingsBlock title={<a href="#missions">Missions</a>}>
              <SettingsRows>
                <SettingsRow label={m("Rounds of review")} tomlKey={m("[missions] max_rounds")} about={m("How many times a change goes back to its crew before it's yours to decide.")}>
                  <Number value={3} />
                </SettingsRow>
              </SettingsRows>
            </SettingsBlock>
          </>
        ) : (
          <>
            <PageHead title="Missions" />
            <p className="lede">What ws holds on every mission, whatever an agent reads, and how the orchestrator moves one along.</p>
            <SettingsBlock id="missions" title="Guarantees" about="The process says the same in prose, for the planner and the orchestrator.">
              <SettingsRows>
                <SettingsRow label="Review before a pull request" tomlKey="[missions] review_before_pr" about="A mission's change is yours to read before ws opens its pull request." source={team}>
                  <Toggle on />
                </SettingsRow>
                <SettingsRow label="Replies need you" tomlKey="[missions] replies_need_me" about="A reply to a review thread is yours to send.">
                  <Toggle on />
                </SettingsRow>
                <SettingsRow label="Rounds of review" tomlKey="[missions] max_rounds" about="How many times a change goes back to its crew before it's yours to decide." source={yours("default 3")}>
                  <Number value={5} />
                </SettingsRow>
              </SettingsRows>
            </SettingsBlock>
            <SettingsBlock id="orchestrator" title="Orchestrator">
              <SettingsRows>
                <SettingsRow label="Moves missions along" tomlKey="[orchestrator] enabled" about="Off, a mission waits for you between its steps.">
                  <Switch on onChange={none} aria-label="Moves missions along" />
                </SettingsRow>
              </SettingsRows>
            </SettingsBlock>
          </>
        )}
      </SettingsLayout>
    </div>
  );
}

export const Section: StoryObj = { render: () => <Page /> };

export const Finding: StoryObj = { render: () => <Page typed="rounds model agent" /> };

export const NothingFound: StoryObj = {
  render: () => {
    function None() {
      const [query, setQuery] = useState("zebra");
      return (
        <div style={{ height: "100vh", overflow: "auto" }}>
          <SettingsLayout side={<Side query={query} setQuery={setQuery} />}>
            <PageHead title={<>“{query.trim()}”</>}>
              <span className="dim">0 settings</span>
            </PageHead>
            <div className="quiet-line">No setting says that. What only a file sets (a repo's commands, paths) isn't here.</div>
          </SettingsLayout>
        </div>
      );
    }
    return <None />;
  },
};
