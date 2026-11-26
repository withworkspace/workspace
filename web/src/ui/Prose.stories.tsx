import type { Meta, StoryObj } from "@storybook/react-vite";
import { Markdown } from "../markdown/LazyMarkdown";
import { Prose } from "./Prose";

// Prose: a document's markdown, larger and at a reading measure, its link
// in the accent; beside it the same markdown not in Prose.

const meta: Meta = { title: "Components/Prose" };
export default meta;

const text = `## Where invoices go

Archived invoices move to **cold storage** after ninety days. The [retention policy](https://example.com) says how long each kind is kept.

- Paid invoices: seven years
- Drafts: thirty days`;

export const Reading: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 24 }}>
      <Prose>
        <Markdown text={text} />
      </Prose>
      <Markdown text={text} />
    </div>
  ),
};
