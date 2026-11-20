import type { Meta, StoryObj } from "@storybook/react-vite";
import { Link } from "react-router";
import { Chip, chipClass } from "./Chip";
import { Icon } from "./Icon";
import type { Tone } from "./tone";

// Chips: grey, then in each tone, with an icon, and a link that's a chip.

const meta: Meta = { title: "Library/Chip" };
export default meta;

const tones: Tone[] = ["attention", "progress", "done", "waiting", "danger", "info", "quiet"];

export const Tones: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 12 }}>
      <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
        <Chip>no tone</Chip>
        {tones.map((t) => (
          <Chip key={t} tone={t}>
            {t}
          </Chip>
        ))}
      </div>
      <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
        {tones.map((t) => (
          <Chip key={t} tone={t}>
            <Icon name="check" size={12} /> {t}
          </Chip>
        ))}
      </div>
      <div style={{ display: "flex", gap: 8 }}>
        <Link to="/" className={chipClass("quiet")}>
          a link
        </Link>
        <Link to="/" className={chipClass("attention")}>
          one that needs you
        </Link>
      </div>
    </div>
  ),
};
