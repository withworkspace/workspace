import type { Meta, StoryObj } from "@storybook/react-vite";
import { Inline } from "./Inline";
import { plain } from "./text";

// Inline markdown: what an agent wrote, as a line of the words around it.
// Code, bold, emphasis and a link's words are drawn; a list, a heading, a
// quote and a fence fold into the line; and it's cut to a line or two
// where there's only room for that.

const meta: Meta = { title: "Components/Inline" };
export default meta;

const line = "Rounding is in `invoice.go`: **half away from zero**, as the *ledger* does. See [the auth config](https://example.com/auth).";

const blocks = ["## What changed", "", "- `roundDiv` takes the divisor's sign", "- the dashboard calls it", "", "> Nothing else divides an amount.", "", "```go", "func roundDiv(a, b int64) int64", "```"].join("\n");

const long =
  "The dashboard divided an amount by a count with `/`, which truncates toward zero, while `invoice.go` rounds half away from zero with `roundDiv`: the two disagreed by a penny on about one invoice in forty, always in the customer's favour, and only where the count was odd.";

export const Written: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 14, maxWidth: 520 }}>
      <div>
        <Inline text={line} />
      </div>
      <div>
        <div className="dim">Blocks fold into the line</div>
        <Inline text={blocks} />
      </div>
      <div>
        <div className="dim">The same, with no markup, as a title has it (plain)</div>
        {plain(blocks)}
      </div>
    </div>
  ),
};

export const Clamped: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 14, maxWidth: 360 }}>
      {[1, 2, 3].map((n) => (
        <div key={n}>
          <div className="dim">
            {n} {n === 1 ? "line" : "lines"}
          </div>
          <Inline text={long} lines={n} />
        </div>
      ))}
      <div>
        <div className="dim">Whole</div>
        <Inline text={long} />
      </div>
    </div>
  ),
};
