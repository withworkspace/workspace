import type { Meta, StoryObj } from "@storybook/react-vite";
import { Change } from "./Change";
import { Text } from "./Text";

// Changes: a suggestion on code (signed, titled, a long line scrolling),
// one that takes lines away, words that wrap (an edit to a document),
// passages of a brief with what was there struck through, and the lines
// alone, for a card that's a box already.

const meta: Meta = { title: "Components/Change" };
export default meta;

const col = { display: "grid", gap: 6, maxWidth: 560 } as const;

export const Code: StoryObj = {
  render: () => (
    <div className="page" style={col}>
      <Text dim>A suggestion</Text>
      <Change
        title="Suggested change"
        signs
        was={["\tfor _, l := range lines {", "\t\tsum += l.Amount.Round()", "\t}"]}
        now={["\tfor _, l := range lines {", "\t\tsum += l.Amount", "\t}", "\treturn sum.Round() // once, at the end: a line rounded alone loses a penny in three"]}
      />
      <Text dim>Lines taken away</Text>
      <Change title="Suggested change" signs was={["\tlog.Printf(\"total %v\", sum)"]} gone="(delete these lines)" />
    </div>
  ),
};

export const Words: StoryObj = {
  render: () => (
    <div className="page" style={col}>
      <Text dim>An edit to a document</Text>
      <Change wrap was={["Invoices are exported in the order they were raised, which is the order customers expect to see them in when they open the file."]} now={["Invoices are exported in the order they were raised."]} />
      <Text dim>Passages of a brief</Text>
      <Change
        wrap
        struck
        hunks={[
          { was: ["Export invoices as CSV."], now: ["Export invoices as CSV, one row an invoice.", ""] },
          { was: ["- totals in pence"], now: ["- totals in pounds, to the penny"] },
        ]}
      />
      <Text dim>The lines alone</Text>
      <Change bare wrap was={["a penny short"]} gone="(delete it)" />
    </div>
  ),
};
