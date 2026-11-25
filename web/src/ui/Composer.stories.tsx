import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { Button } from "./Button";
import { Composer } from "./Composer";
import { Icon } from "./Icon";
import { Keys } from "./Keys";
import { Text } from "./Text";

// Composers: with a bar under the words (what sending does, and what's
// pressed), with nothing to say in it, with something else to do, a
// small one with its buttons beside the words, and one with no bar,
// whose keys are in its corner. Each has the ring while its words have
// the keys, and grows as they're written.

const meta: Meta = { title: "Components/Composer" };
export default meta;

function One(props: Omit<React.ComponentProps<typeof Composer>, "value" | "onChange" | "onSend" | "children"> & { start?: string; children?: (text: string) => React.ReactNode }) {
  const { start = "", children, ...rest } = props;
  const [value, setValue] = useState(start);
  return (
    <Composer value={value} onChange={setValue} onSend={() => setValue("")} {...rest}>
      {children?.(value)}
    </Composer>
  );
}

const row = { display: "grid", gridTemplateColumns: "120px minmax(0, 520px)", gap: 12, alignItems: "start" } as const;

export const Bar: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 18 }}>
      <div style={row}>
        <Text dim>With a hint</Text>
        <One
          rows={2}
          aria-label="Start a conversation"
          placeholder="What are you thinking about? An idea, a problem, a question about the code…"
          hint={
            <>
              <Icon name="eye" size={12} />
              The agent reads the repos, the missions and GitHub, and changes nothing.
            </>
          }
        >
          {(text) => (
            <Button variant="primary" disabled={!text.trim()}>
              Start
            </Button>
          )}
        </One>
      </div>
      <div style={row}>
        <Text dim>Nothing to say</Text>
        <One aria-label="Reply" placeholder="Reply…">
          {(text) => (
            <Button variant="primary" disabled={!text.trim()}>
              Send
            </Button>
          )}
        </One>
      </div>
      <div style={row}>
        <Text dim>Something else to do</Text>
        <One
          aria-label="Message api-1"
          start="Leave the export as it is: the rounding's the only thing to change."
          hint={
            <>
              <Icon name="message" size={12} /> Sent to it as a message, for when this turn ends.
            </>
          }
          also={
            <Button variant="ghost">
              <Icon name="stop" size={12} /> Interrupt
            </Button>
          }
        >
          {(text) => (
            <Button variant="primary" disabled={!text.trim()}>
              Send
            </Button>
          )}
        </One>
      </div>
      <div style={row}>
        <Text dim>Being sent</Text>
        <One aria-label="Reply" start="And the totals row?" busy>
          {() => (
            <Button variant="primary" disabled>
              Sending…
            </Button>
          )}
        </One>
      </div>
    </div>
  ),
};

export const Beside: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 18 }}>
      <div style={row}>
        <Text dim>One thing to press</Text>
        <One layout="beside" send="enter" aria-label="Ask for these words to be changed" placeholder="What should change here? Tighten it, make it a table, add what's missing…">
          {(text) => (
            <Button variant="primary" size="small" disabled={!text.trim()}>
              Ask <Keys k="⏎" />
            </Button>
          )}
        </One>
      </div>
      <div style={row}>
        <Text dim>Two</Text>
        <One layout="beside" send="enter" aria-label="Change this cell, or ask about it" start="Group them by service, and drop the last column.">
          {(text) => (
            <>
              <Button size="small" disabled={!text.trim()}>
                Just ask
              </Button>
              <Button variant="primary" size="small" disabled={!text.trim()}>
                Change <Keys k="⏎" />
              </Button>
            </>
          )}
        </One>
      </div>
    </div>
  ),
};

export const Corner: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 18 }}>
      <div style={row}>
        <Text dim>Keys in the corner</Text>
        <One layout="corner" aria-label="Ask about this page" placeholder="Ask about this page…" />
      </div>
      <div style={row}>
        <Text dim>With something held</Text>
        <One
          layout="corner"
          aria-label="Ask about this page"
          start="Is this still how it works?"
          over={
            <span className="ask-ctx holding">
              <Icon name="layers" size={11} />
              <span className="q">“Rounding moved to the end, so a penny doesn't go missing.”</span>
            </span>
          }
        />
      </div>
    </div>
  ),
};
