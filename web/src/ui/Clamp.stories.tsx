import type { Meta, StoryObj } from "@storybook/react-vite";
import { Clamp } from "./Clamp";

// Clamps: something long cut to a height, with the way to the rest;
// something short, which has none; and one cut to a count of lines.

const meta: Meta = { title: "Components/Clamp" };
export default meta;

const long = [
  "The export reads invoices by id, which only goes up, so they come out in the order they were raised.",
  "Rounding moved to the end: a line rounded alone loses a penny in three, and INV-0042 came out at £1,094.39.",
  "The totals row is written last, after the file's flushed, which is what the test for an empty month was missing.",
  "Nothing else changed: the columns are as they were, and so is the header.",
  "What's left is the date format, which the brief doesn't say: it's ISO now, as the API's is.",
];

const col = { display: "grid", gap: 6, maxWidth: 420 } as const;

export const Clamps: StoryObj = {
  render: () => (
    <div className="page" style={col}>
      <span className="dim">Long</span>
      <Clamp of={long}>
        {long.map((p, i) => (
          <p key={i} style={{ margin: "0 0 8px" }}>
            {p}
          </p>
        ))}
      </Clamp>
      <span className="dim">Short</span>
      <Clamp of={long[0]}>
        <p style={{ margin: 0 }}>{long[0]}</p>
      </Clamp>
      <span className="dim">Three lines</span>
      <Clamp lines={3} more="More" less="Less" of={long}>
        {long.join(" ")}
      </Clamp>
    </div>
  ),
};
