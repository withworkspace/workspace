import type { Meta, StoryObj } from "@storybook/react-vite";
import { RefsScope } from "../refs/known";
import type { Known } from "../refs/refs";
import { ReviewSaid } from "../review/ReviewSaid";
import { storyWorkspace } from "../stories/missions";
import { useWorkspaceStore } from "../workspace/store";
import { RichText } from "./RichText";
import { Text } from "./Text";
import { Turn, TurnSaid } from "./Turn";

// What someone said, as it reads: lines as they were written, markdown as
// a RichTextArea writes it, and the names it says drawn as what they are.
// Each as a turn of yours (a bubble) and as a comment in a review.

const meta: Meta = { title: "Components/RichText" };
export default meta;

// The names are the stories' workspace's missions.
storyWorkspace();
const ws = useWorkspaceStore.getState().workspace!;
const mission = ws.missions[0]?.slug ?? "a-mission";
const known: Known = { repos: new Map(), missions: ws.missions.map((m) => m.slug), pages: [], talks: [] };

const said = {
  lines: "Two things:\nthe totals row is off by a penny,\nand the export rounds again.",
  markdown: "Could we **also** keep `amount_minor`?\n\n- for stored rows\n- and in the export\n\n> rounds half to even\n\n```sh\njust try up marketing\n```",
  heading: "# Why\n\nA heading in a comment is barely larger than its words.",
  refs: `Like ${mission} did, before it rounded.`,
};

const row = { display: "grid", gridTemplateColumns: "120px minmax(0, 460px) minmax(0, 460px)", gap: 24, alignItems: "start" } as const;

export const Said: StoryObj = {
  render: () => (
    <RefsScope known={known}>
      <div className="page" style={{ display: "grid", gap: 24 }}>
        {Object.entries(said).map(([name, text]) => (
          <div key={name} style={row}>
            <Text dim>{name}</Text>
            <Turn by="you">
              <TurnSaid>
                <RichText text={text} />
              </TurnSaid>
            </Turn>
            <ReviewSaid who="You">
              <RichText text={text} />
            </ReviewSaid>
          </div>
        ))}
      </div>
    </RefsScope>
  ),
};
