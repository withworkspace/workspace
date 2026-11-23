import type { Meta, StoryObj } from "@storybook/react-vite";
import { Button } from "./Button";
import { Icon } from "./Icon";
import { Keys } from "./kit";
import { Tooltip } from "./Tooltip";
import { Toasts } from "./kit";

// Tooltips: rest the pointer on a button, or Tab to it. A Button's title
// is one; a long one wraps. The last two can't be used now: the disabled
// one is out of reach and has the browser's title; the unavailable one
// says why in its tooltip, and again in a toast when it's pressed.

const meta: Meta = { title: "Components/Tooltip" };
export default meta;

const row = { display: "flex", gap: 10, alignItems: "center", flexWrap: "wrap", paddingTop: 60 } as const;

export const OnButtons: StoryObj = {
  render: () => (
    <div className="page" style={row}>
      <Button title="Approve the document as it is (⇧A)">
        Approve <Keys k="⇧A" />
      </Button>
      <Button variant="ghost" square title="Close (esc)">
        <Icon name="x" size={13} />
      </Button>
      <Button variant="ghost" size="small" title="Every layer's changes together, to see where the stack ends up. It's for reading: comments are made on a layer. (w)">
        <Icon name="layers" size={12} /> Whole stack
      </Button>
      <Tooltip text="Under it, where there's no room over it" placement="bottom">
        <Button>Placed under</Button>
      </Tooltip>
      <Button disabled title="The browser's own, while it's disabled">
        Disabled
      </Button>
      <Button variant="primary" unavailable="Pick a verdict first">
        Submit
      </Button>
      <Toasts />
    </div>
  ),
};
