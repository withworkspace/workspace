import type { Meta, StoryObj } from "@storybook/react-vite";
import { Printed } from "./Printed";

// What a command printed: a few lines, and more than fit, which scroll.

const meta: Meta = { title: "Components/Printed" };
export default meta;

export const Outputs: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 16, maxWidth: 560 }}>
      <Printed>{"$ go test ./...\nok  \tdemo\t0.212s"}</Printed>
      <Printed>{Array.from({ length: 40 }, (_, i) => `=== RUN   TestGreeting/${i}\n--- PASS: TestGreeting/${i} (0.00s)`).join("\n")}</Printed>
    </div>
  ),
};
