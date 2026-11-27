import type { Meta, StoryObj } from "@storybook/react-vite";
import { AskField } from "../ask/AskField";
import { Turn, TurnSaid } from "../ui/Turn";
import { ConversationGiven } from "./ConversationGiven";
import { ConversationStage } from "./ConversationStage";
import { RichText } from "../ui/RichText";

// A page's conversation, from its parts: the page with the ask field over
// its foot (clicks beside it reach the page), and what a page gave the
// agent with what you said, folded under it (open it to see it).

const meta: Meta = { title: "Conversation/Conversation" };
export default meta;

export const OnAPage: StoryObj = {
  render: () => (
    <div style={{ display: "flex", height: 360, background: "var(--bg)" }}>
      <ConversationStage foot={<AskField place="over" placeholder="Ask about THING-2…" suggestions={["What's blocking it?", "Summarise its comments"]} onAsk={() => {}} />}>
        <div style={{ flex: 1, padding: 24, overflow: "auto" }}>
          <h2 style={{ marginTop: 0 }}>THING-2: Rate-limit the public API</h2>
          <p>A plugin's page, drawn as the plugin draws it. The field over its foot asks about the thing it's of.</p>
        </div>
      </ConversationStage>
    </div>
  ),
};

export const Given: StoryObj = {
  render: () => (
    <div style={{ display: "grid", width: 420, padding: 24, background: "var(--bg)" }}>
      <Turn by="you" dense landed>
        <TurnSaid>
          <RichText text="Why is this one stuck?" />
        </TurnSaid>
        <ConversationGiven
          from="Jira"
          parts={[
            ["Selected (2 cards)", "THING-2: Rate-limit the public API\nTHING-4: Say so in the docs"],
            ["With this message", "The board's In Review column, filtered to mine."],
            ["The page", ""],
          ]}
        />
      </Turn>
    </div>
  ),
};
