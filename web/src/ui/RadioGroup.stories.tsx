import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { Radio, RadioGroup } from "./RadioGroup";
import { Text } from "./Text";

// A radio group: a word or two each, words that run to two lines (the
// dot sits by the first), and disabled. Press one, or move with the
// arrows.

const meta: Meta = { title: "Components/RadioGroup" };
export default meta;

function Whose({ disabled }: { disabled?: boolean }) {
  const [whose, setWhose] = useState("me");
  return (
    <RadioGroup value={whose} onChange={setWhose} disabled={disabled} aria-label="Whose">
      <Radio value="me">
        <span>
          Just me <Text dim>— in ws.local.toml</Text>
        </span>
      </Radio>
      <Radio value="team">
        <span>
          The team <Text dim>— in ws.toml, committed and pushed</Text>
        </span>
      </Radio>
    </RadioGroup>
  );
}

function Places() {
  const [place, setPlace] = useState("0");
  return (
    <RadioGroup value={place} onChange={setPlace} aria-label="Where it'll live">
      {["records", "api"].map((repo, i) => (
        <Radio key={repo} value={String(i)}>
          <span style={{ display: "grid", gap: 2 }}>
            <span style={{ fontWeight: 500 }}>{repo}</span>
            <code>{repo}:sketches/</code>
          </span>
        </Radio>
      ))}
      <Radio value="else">Somewhere else…</Radio>
    </RadioGroup>
  );
}

export const Groups: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 24, justifyItems: "start" }}>
      <Whose />
      <Places />
      <Whose disabled />
    </div>
  ),
};
