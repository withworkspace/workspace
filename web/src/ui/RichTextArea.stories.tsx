import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { type Features, RichTextArea } from "./RichTextArea";
import { Text } from "./Text";

// Boxes written in markdown, each offering something else: words alone,
// marks, lists, a quote, code, all of it. The box has no look of its own,
// so each is in a frame here, and under it is the markdown it hands back
// and what it sent. Type **bold**, - a list, > a quote, ``` code.

const meta: Meta = { title: "Components/RichTextArea" };
export default meta;

const frame = { border: "1px solid var(--line-strong)", borderRadius: "var(--radius)", background: "var(--raised)", padding: "8px 10px" } as const;
const row = { display: "grid", gridTemplateColumns: "120px minmax(0, 460px) minmax(0, 1fr)", gap: 12, alignItems: "start" } as const;
const out = { margin: 0, font: "var(--text-sm) / 1.5 var(--mono)", color: "var(--dim)", whiteSpace: "pre-wrap" } as const;

function One({ label, start = "", send, ...features }: Features & { label: string; start?: string; send?: "enter" | "mod+enter" }) {
  const [value, setValue] = useState(start);
  const [sent, setSent] = useState<string>();
  return (
    <div style={row}>
      <Text dim>{label}</Text>
      <div style={frame}>
        <RichTextArea
          {...features}
          value={value}
          onChange={setValue}
          send={send}
          onSend={() => {
            setSent(value);
            setValue("");
          }}
          rows={2}
          placeholder="Write something…"
          aria-label={label}
        />
      </div>
      <pre style={out}>
        {value || "—"}
        {sent !== undefined && `\n\nsent:\n${sent}`}
      </pre>
    </div>
  );
}

const all = { marks: true, lists: true, quote: true, code: true };

export const Offers: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 18 }}>
      <One label="Words" start={"Only words, in paragraphs."} />
      <One label="Marks" marks start={"**Bold**, *italic*, `code` and a link: https://example.com"} />
      <One label="Lists" lists marks start={"- one\n- two\n  1. nested"} />
      <One label="Quote" quote marks start={"> What was said\n\nAnd the answer."} />
      <One label="Code" code marks start={"Run this:\n\n```sh\njust try up marketing\n```"} />
      <One label="All of it" {...all} />
    </div>
  ),
};

// The key that sends: with ⏎, it goes on in a list or a block of code
// first, and an empty item ends the list; with ⌘⏎, ⏎ is a new line.
export const SendKeys: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 18 }}>
      <One label="⏎ sends" send="enter" {...all} start={"- an item"} />
      <One label="⌘⏎ sends" send="mod+enter" {...all} start={"- an item"} />
    </div>
  ),
};
