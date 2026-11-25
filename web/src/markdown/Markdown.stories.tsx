import type { Meta, StoryObj } from "@storybook/react-vite";
import { Markdown } from "./LazyMarkdown";
import type { Place } from "./links";

// A document as the reader draws it: every kind of block there is, alone
// and as a document to read (.prose), its headings' links and what can't
// be found.

const meta: Meta = { title: "Markdown/Markdown" };
export default meta;

const text = `---
title: Limit each key
owner: api
---

# Limit each key

A paragraph with **strong words**, *a slant*, ~~struck~~, \`inline code\`, [a link](https://example.com) and a footnote.[^1]

## What changes

- A bullet
- Another, with a [link to a heading](#what-changes)
  - and one under it

1. First
2. Second

- [ ] Still to do
- [x] Done

### In a table

| Key | Limit |
| --- | ---: |
| api | 100 |
| web | 20 |

#### Smaller still

> A quote from someone,
> over two lines.

> [!NOTE]
> Something worth knowing.

> [!TIP]
> A better way.

> [!IMPORTANT]
> Not to be missed.

> [!WARNING]
> Careful here.

> [!CAUTION]
> This can't be taken back.

\`\`\`go
func limit(key string) int {
	return 100
}
\`\`\`

\`\`\`
no language
\`\`\`

Maths in a line, $x^2 + y^2$, and on lines of its own:

$$
\\sum_{i=0}^{n} i = \\frac{n(n+1)}{2}
$$

\`\`\`mermaid
graph LR
  A[Asked] --> B[Limited]
\`\`\`

\`\`\`mermaid
graph LR
  A[ -->
\`\`\`

A link [out of the repo](../../../../elsewhere.md) and an image that isn't there: ![a picture](../../../../gone.png)

---

[^1]: What the footnote says.
`;

const at: Place = { api: "/api/files/demo", path: "docs/limits.md", to: (p) => `/files/demo?path=${encodeURIComponent(p)}` };

export const EveryBlock: StoryObj = {
  render: () => (
    <div style={{ padding: 32 }}>
      <Markdown text={text} at={at} headingLink={() => {}} />
    </div>
  ),
};

export const ToRead: StoryObj = {
  render: () => (
    <div style={{ padding: 32 }} className="prose">
      <Markdown text={text} at={at} lines />
    </div>
  ),
};
