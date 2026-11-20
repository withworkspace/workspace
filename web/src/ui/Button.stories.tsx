import type { Meta, StoryObj } from "@storybook/react-vite";
import { Link } from "react-router";
import { Button, type ButtonLook, buttonClass } from "./Button";
import { Icon } from "./Icon";
import { Keys } from "./kit";

// Buttons: each variant, plain and small, then disabled, with an icon,
// with a key, square (an icon alone), and a link drawn as a button.

const meta: Meta = { title: "Library/Button" };
export default meta;

const variants: { name: string; look: ButtonLook }[] = [
  { name: "Plain", look: {} },
  { name: "Primary", look: { variant: "primary" } },
  { name: "Danger", look: { variant: "danger" } },
  { name: "Ghost", look: { variant: "ghost" } },
  { name: "Link", look: { variant: "link" } },
];

const cell = { display: "flex", gap: 8, alignItems: "center", flexWrap: "wrap" } as const;

export const Variants: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 16 }}>
      {variants.map(({ name, look }) => (
        <div key={name} style={cell}>
          <span className="dim" style={{ width: 80 }}>
            {name}
          </span>
          <Button {...look}>Save</Button>
          <Button {...look} size="small">
            Save
          </Button>
          <Button {...look} disabled>
            Disabled
          </Button>
          <Button {...look}>
            <Icon name="play" size={13} /> With an icon
          </Button>
          <Button {...look}>
            Approve <Keys k="a" />
          </Button>
          <Button {...look} square title="Close">
            <Icon name="x" size={13} />
          </Button>
          <Button {...look} square size="small" title="Close">
            <Icon name="x" size={11} />
          </Button>
        </div>
      ))}
    </div>
  ),
};

export const Links: StoryObj = {
  render: () => (
    <div className="page" style={cell}>
      <Link to="/" className={buttonClass()}>
        A link
      </Link>
      <Link to="/" className={buttonClass({ variant: "primary" })}>
        Primary
      </Link>
      <Link to="/" className={buttonClass({ variant: "ghost" })}>
        Ghost
      </Link>
      <Link to="/" className={buttonClass({ size: "small" })}>
        Small
      </Link>
    </div>
  ),
};
