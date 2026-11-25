import type { Meta, StoryObj } from "@storybook/react-vite";
import { Doing } from "./Doing";
import { Turn, TurnAvatar, TurnHead, TurnSaid } from "./Turn";
import { markdownClass } from "../markdown/LazyMarkdown";
import { cx } from "./cx";
import { textClass } from "./Text";

// Turns: an exchange as a conversation's page draws it (who said each,
// yours a bubble to the right under your name, the agent's its words to
// the left, ws's quiet, and the turn going now), and the same exchange
// dense, as it's drawn beside something else: no heads.

const meta: Meta = { title: "Components/Turn" };
export default meta;

const answer = (
  <div className={cx("markdown", markdownClass)}>
    <p>In one place, by one rule.</p>
    <p>
      <code>invoice.go</code> has <code>roundDiv</code>, which rounds half away from zero. Nothing else in the api divides an amount.
    </p>
  </div>
);

const at = "2026-11-23T10:04:00Z";

export const Page: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 18, maxWidth: 620 }}>
      <Turn by="you" landed head={<TurnHead avatar={<TurnAvatar by="you" />} who="You" at={at} />}>
        <TurnSaid written>{"Where do we round money,\nand is it the same everywhere?"}</TurnSaid>
      </Turn>
      <Turn by="agent" landed head={<TurnHead avatar={<TurnAvatar by="agent" />} who="Agent" at={at} />}>
        <TurnSaid>{answer}</TurnSaid>
      </Turn>
      <Turn by="ws" landed head={<TurnHead avatar={<TurnAvatar by="ws" />} who="ws" at={at} />}>
        <TurnSaid>The agent stopped before it answered: its run was ended.</TurnSaid>
      </Turn>
      <Turn by="agent" running head={<TurnHead avatar={<TurnAvatar by="agent" />} who="Agent" />}>
        <TurnSaid className={textClass("dim")}>
          <div className={cx("markdown", markdownClass)}>
            <p>Looking at where the dashboard divides.</p>
          </div>
        </TurnSaid>
        <Doing calls={[]} />
      </Turn>
    </div>
  ),
};

export const Dense: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 14, maxWidth: 360 }}>
      <Turn by="you" dense landed>
        <TurnSaid written>Where do we round money, and is it the same everywhere?</TurnSaid>
      </Turn>
      <Turn by="agent" dense landed>
        <TurnSaid>{answer}</TurnSaid>
      </Turn>
      <Turn by="ws" dense landed>
        <TurnSaid>The agent stopped before it answered.</TurnSaid>
      </Turn>
      <Turn by="agent" dense running>
        <Doing calls={[]} />
      </Turn>
    </div>
  ),
};
