import type { Meta, StoryObj } from "@storybook/react-vite";
import type { ReactNode } from "react";
import type { CrewPicks } from "../../api";
import { Button } from "../../ui/Button";
import type { CalloutTone } from "../../ui/Callout";
import { Inline } from "../../ui/Inline";
import { Input } from "../../ui/Input";
import { Keys } from "../../ui/Keys";
import { MoreLink } from "../../ui/MoreLink";
import { MissionLead } from "./MissionLead";
import { MissionLeadAnswer, MissionLeadAnswerForm, MissionLeadButtons } from "./MissionLeadAnswer";
import { MissionLeadLatest, MissionLeadSaid } from "./MissionLeadLatest";
import { MissionLeadLaunch } from "./MissionLeadLaunch";
import { MissionLeadNotice } from "./MissionLeadNotice";
import { MissionLeadPrompt } from "./MissionLeadPrompt";
import { MissionLeadRequest } from "./MissionLeadRequest";
import { MissionSummarySaid } from "./MissionSummarySaid";

// Where a mission stands, as its Summary leads with it, drawn from props:
// the lead in each tone, with one thing to do beside it or choices and a
// field under it, crew's last word at its foot, a notice under it, and
// Launch crew alone and split. Every card ws makes, on the missions it
// gave, is Mission/Banners.

const meta: Meta = { title: "Mission/MissionLead" };
export default meta;

const none = () => {};
const column = (children: ReactNode) => <div style={{ maxWidth: 860, padding: "16px 32px 48px", background: "var(--bg)" }}>{children}</div>;
const step = (
  <span>
    Step 3 of 6: <strong>Review</strong>
  </span>
);

const tones: [CalloutTone, string, string][] = [
  ["attention", "user", "Yours to review"],
  ["progress", "spinner", "c2 is working"],
  ["danger", "x-circle", "Checks failing on api#41"],
  ["info", "clock", "Waiting on the platform team"],
  ["done", "check-circle", "Merged"],
  ["quiet", "pause", "Parked"],
];

export const Tones: StoryObj = {
  render: () =>
    column(
      <>
        {tones.map(([tone, icon, title]) => (
          <MissionLead key={tone} tone={tone} icon={icon as never} live={tone === "progress"} title={title} step={step}>
            <Inline text="What's happening, in a sentence or two, as ws says it." />
          </MissionLead>
        ))}
      </>,
    ),
};

export const WhatMovesIt: StoryObj = {
  name: "What moves it",
  render: () =>
    column(
      <>
        <MissionLead
          tone="attention"
          icon="user"
          title="Yours to review"
          step={step}
          actions={
            <Button variant="primary">
              Review it <Keys k="v" />
            </Button>
          }
        >
          One thing to do: beside it.
        </MissionLead>
        <MissionLead
          tone="attention"
          icon="question"
          title="c1 asks which date format"
          step={step}
          wide
          actions={
            <MissionLeadAnswer>
              <MissionLeadAnswerForm onSubmit={none}>
                <Input placeholder="Your answer" aria-label="Your answer" />
                <Button variant="primary" type="submit">
                  Answer
                </Button>
              </MissionLeadAnswerForm>
              <MissionLeadButtons>
                <Button>ISO 8601</Button>
                <Button>The locale's</Button>
                <Button variant="ghost">Not yet</Button>
              </MissionLeadButtons>
            </MissionLeadAnswer>
          }
        >
          Choices and a field: under it, its width.
        </MissionLead>
        <MissionLead
          tone="attention"
          icon="alert"
          title="c2 wants to run a command"
          wide
          actions={
            <MissionLeadPrompt>
              <div style={{ padding: 12, border: "1px solid var(--line)", borderRadius: 8 }}>What it's stopped on (crew's CrewPrompt)</div>
              <MoreLink to="/">Open c2's conversation</MoreLink>
            </MissionLeadPrompt>
          }
        />
        <MissionLead
          tone="progress"
          icon="spinner"
          live
          title="c2 is working"
          step={step}
          latest={
            <MissionLeadLatest>
              <MissionLeadSaid who="From c1">
                <MissionSummarySaid text="Done: the export has a **header row**, and dates are ISO 8601." />
              </MissionLeadSaid>
              <MissionLeadSaid who="From c3">
                <MissionSummarySaid text="Waiting on c2's branch before I start the docs." />
              </MissionLeadSaid>
            </MissionLeadLatest>
          }
        >
          With what crew last said, at its foot.
        </MissionLead>
        <MissionLead tone="attention" icon="pencil" title="To brief">
          <MissionLeadRequest>Requested: Archive invoices older than seven years</MissionLeadRequest>
        </MissionLead>
        <MissionLeadNotice icon="message" title="A notice under the lead" actions={<Button>Open it</Button>}>
          <Inline text="Anything after the first card is one of these." />
        </MissionLeadNotice>
      </>,
    ),
};

const picks: CrewPicks = { default: { harness: "claude", model: "opus" }, allowed: [{ harness: "claude", model: "sonnet" }, { harness: "codex" }] } as CrewPicks;

export const Launch: StoryObj = {
  render: () =>
    column(
      <div style={{ display: "flex", gap: 16, alignItems: "center", flexWrap: "wrap" }}>
        <MissionLeadLaunch primary again={false} busy={false} launch={none} />
        <MissionLeadLaunch primary again={false} busy={false} launch={none} picks={picks} />
        <MissionLeadLaunch primary={false} again busy={false} launch={none} picks={picks} />
        <MissionLeadLaunch primary again={false} busy launch={none} picks={picks} />
      </div>,
    ),
};
