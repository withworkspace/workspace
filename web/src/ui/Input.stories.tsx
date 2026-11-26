import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { Input } from "./Input";
import { Text, textClass } from "./Text";

// Text boxes: empty with a placeholder, with words, in mono, disabled,
// invalid, and one a page draws without its box (in a bar of its own).

const meta: Meta = { title: "Components/Input" };
export default meta;

function One(props: React.ComponentProps<typeof Input> & { start?: string }) {
  const { start = "", ...rest } = props;
  const [value, setValue] = useState(start);
  return <Input value={value} onChange={(e) => setValue(e.target.value)} {...rest} />;
}

const row = { display: "grid", gridTemplateColumns: "100px 280px", gap: 12, alignItems: "center" } as const;

export const Boxes: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 14 }}>
      <label style={row}>
        <Text dim>Empty</Text>
        <One placeholder="What it's named from" aria-label="Title" />
      </label>
      <label style={row}>
        <Text dim>With words</Text>
        <One start="Export invoices as CSV" aria-label="Title" />
      </label>
      <label style={row}>
        <Text dim>Mono</Text>
        <One className={textClass("mono")} start="records:proposals/" aria-label="Where" />
      </label>
      <label style={row}>
        <Text dim>Disabled</Text>
        <One start="Export invoices as CSV" disabled aria-label="Title" />
      </label>
      <label style={row}>
        <Text dim>Invalid</Text>
        <One start="not a number" aria-invalid aria-label="How many" />
      </label>
      <label style={row}>
        <Text dim>A page's own</Text>
        <span style={{ display: "flex", borderBottom: "1px solid var(--line)" }}>
          <One placeholder="Find a setting" aria-label="Find" style={{ flex: 1, border: 0, background: "none", boxShadow: "none", padding: "6px 0" }} />
        </span>
      </label>
    </div>
  ),
};
