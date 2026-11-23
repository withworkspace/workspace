import type { Meta, StoryObj } from "@storybook/react-vite";
import { Button } from "./Button";
import { Callout, type CalloutTone } from "./Callout";

// Callouts: in each tone with its own icon, then what one can hold: a
// title alone, words alone, an icon of its own or none, what's pressed,
// and a big one.

const meta: Meta = { title: "Components/Callout" };
export default meta;

const tones: CalloutTone[] = ["attention", "info", "danger", "done", "progress", "quiet"];

export const Tones: StoryObj = {
  render: () => (
    <div className="page narrow">
      {tones.map((t) => (
        <Callout key={t} tone={t} title={`In ${t}`}>
          What it says, in a sentence or two under its title.
        </Callout>
      ))}
    </div>
  ),
};

export const Holds: StoryObj = {
  render: () => (
    <div className="page narrow">
      <Callout tone="attention" title="A title alone" />
      <Callout tone="done" icon="check-circle">
        Words alone: you approved it, an hour ago.
      </Callout>
      <Callout tone="info" icon="rotate" title="An icon of its own">
        ws 0.15.0 is installed.
      </Callout>
      <Callout tone="quiet" icon={null} title="No icon">
        Its words start at its edge.
      </Callout>
      <Callout tone="attention" icon="arrow-up" title="ws 0.16.0 is out" actions={<Button variant="primary">Upgrade to 0.16.0</Button>}>
        It restarts once nothing's at work.
      </Callout>
      <Callout
        tone="danger"
        title="Couldn't load this"
        actions={
          <>
            <Button>Try again</Button>
            <Button variant="ghost">Dismiss</Button>
          </>
        }
      >
        The daemon didn't answer.
      </Callout>
      <Callout tone="quiet" icon="clock" big title="Waiting on a review" actions={<Button>Look now</Button>}>
        A big one: its title a size up, as a mission's second headline is.
      </Callout>
    </div>
  ),
};
