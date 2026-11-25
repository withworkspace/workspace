import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { AskField } from "./AskField";
import { Text } from "../ui/Text";

// The ask field: the one place you ask an agent about what you're looking
// at. Go to it and what's offered stands over it (↑ and ↓ go through
// them); ⏎ asks. Something in hand is a chip before the words, dropped by
// its x or by backspace in the empty field. What asking does is its
// page's: here it only says what was asked.

const meta: Meta = { title: "Ask/AskField" };
export default meta;

const ground: React.CSSProperties = { display: "flex", flexDirection: "column", justifyContent: "flex-end", height: 260, padding: "0 24px", background: "var(--bg)" };

function Field({ held = false, busy = false, slow = false }: { held?: boolean; busy?: boolean; slow?: boolean }) {
  const [holding, setHolding] = useState(held);
  const [asked, setAsked] = useState<string>();
  return (
    <div style={ground}>
      <Text as="p" dim style={{ margin: "0 0 auto", paddingTop: 16 }}>
        {asked ? `Asked: ${asked}` : "Nothing asked yet."}
      </Text>
      <AskField
        placeholder={holding ? "Ask about this passage…" : "Ask about Usage-based billing…"}
        busy={busy}
        suggestions={holding ? ["Explain this passage", "What does it change?"] : ["Summarise it", "What's still open?", "Who would this affect?"]}
        holding={holding ? { icon: "quote", text: "“nothing is sent until someone has read it”", kind: "passage", drop: { label: "Ask about the whole document", onDrop: () => setHolding(false) } } : undefined}
        onAsk={(q) => (slow ? new Promise<void>((done) => setTimeout(() => (setAsked(q), done()), 900)) : setAsked(q))}
      />
    </div>
  );
}

export const AtAPagesFoot: StoryObj = { render: () => <Field /> };

// With a passage in hand: a new thing to ask about.
export const WithSomethingInHand: StoryObj = { render: () => <Field held /> };

// Asking takes a moment: it says so, and keeps what's typed until it's
// asked.
export const WhileItsAsked: StoryObj = { render: () => <Field slow /> };

// Its agent is answering: nothing more is asked here till it's done.
export const Busy: StoryObj = { render: () => <Field busy /> };

// Where it sits, other than at the foot of what's read: inline in a
// column (the brief's planner, what's offered over a fade), framed by a
// foot that's the page's own (a review's), and over a page's foot,
// lifted (a plugin page's conversation). Go to each to see what's offered.
export const Places: StoryObj = {
  render: () => (
    <div style={{ display: "grid", gap: 96, padding: "96px 24px 24px", background: "var(--bg)" }}>
      <div style={{ position: "relative", maxWidth: 520 }}>
        <AskField place="inline" placeholder="Ask the planner…" suggestions={["What's still open?", "Make step 2 smaller, with its checks said first, and the rest after it"]} onAsk={() => {}} />
      </div>
      <div style={{ position: "relative" }}>
        <AskField place="framed" placeholder="Ask about #41…" suggestions={["What does it change?", "Is anything risky?"]} holding={{ icon: "code", text: "api/limit.go:12–30", kind: "lines", drop: { label: "Ask about the whole pull request", onDrop: () => {} } }} onAsk={() => {}} />
      </div>
      <div style={{ position: "relative" }}>
        <AskField place="over" placeholder="Ask about THING-2…" holding={{ icon: "layers", text: "3 cards", title: "THING-2, THING-4, THING-7" }} busy onAsk={() => {}} />
      </div>
    </div>
  ),
};
