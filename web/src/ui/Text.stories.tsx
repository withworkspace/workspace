import type { Meta, StoryObj } from "@storybook/react-vite";
import { Link } from "react-router";
import { Icon } from "./Icon";
import { Input } from "./Input";
import { Spacer } from "./Spacer";
import { SrOnly } from "./SrOnly";
import { Text, textClass } from "./Text";
import { Time } from "./Time";

// Words with a look of their own among the words round them, each look
// alone and together; then the looks given to parts that aren't Text (an
// Icon, a Time, an Input, a router Link); then a row with a Spacer in it,
// and a mark with words for a screen reader alone.

const meta: Meta = { title: "Components/Text" };
export default meta;

export const Looks: StoryObj = {
  render: () => (
    <div className="page narrow" style={{ display: "grid", gap: 12 }}>
      <p>
        Filed as <Text mono>records:docs/invoices.md</Text> <Text dim>· landed an hour ago</Text>
      </p>
      <p>
        <Text mono dim>internal/nav/nav.go:12</Text> is where it's defined.
      </p>
      <p>
        Changed <Text plus>+128</Text> <Text minus>−37</Text> in three files.
      </p>
      <Text as="p" dim>
        A paragraph that's quieter than the words round it: what a setting does, said under its name.
      </Text>
      <p>
        Read <Link to="/settings/process" className={textClass("link")}>how ws works</Link>, or <Link to="/missions/archive-invoices" className={textClass("link", "mono")}>archive-invoices</Link>.
      </p>
    </div>
  ),
};

export const OnParts: StoryObj = {
  render: () => (
    <div className="page narrow" style={{ display: "grid", gap: 12 }}>
      <p>
        <Icon name="branch" size={12} className={textClass("dim")} /> archive-invoices <Time iso="2026-11-25T09:00:00Z" as="relative" className={textClass("dim")} />
      </p>
      <Input className={textClass("mono")} value="records:notes/" onChange={() => {}} aria-label="A folder, as repo:folder/" />
    </div>
  ),
};

export const SpacerAndSrOnly: StoryObj = {
  render: () => (
    <div className="page narrow" style={{ display: "grid", gap: 12 }}>
      <div style={{ display: "flex", gap: 8, padding: 8, border: "1px solid var(--line)" }}>
        <strong>A row's head</strong>
        <Text dim>its count</Text>
        <Spacer />
        <Text dim>at its end</Text>
      </div>
      <p>
        <Icon name="check" size={12} />
        <SrOnly> (done)</SrOnly> Brief confirmed
      </p>
    </div>
  ),
};
