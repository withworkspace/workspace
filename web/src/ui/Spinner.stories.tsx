import type { Meta, StoryObj } from "@storybook/react-vite";
import { Button } from "./Button";
import { Spinner } from "./Spinner";

// A spinner: at an icon's sizes, beside the words that say what's going
// on, and in a button that's at it.

const meta: Meta = { title: "Components/Spinner" };
export default meta;

export const Sizes: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 16 }}>
      <div style={{ display: "flex", gap: 16, alignItems: "center" }}>
        {[11, 12, 14, 16, 20].map((s) => (
          <Spinner key={s} size={s} />
        ))}
      </div>
      <div className="dim">
        <Spinner size={12} /> Asking GitHub…
      </div>
      <div style={{ display: "flex", gap: 8 }}>
        <Button size="small" disabled>
          <Spinner size={12} /> Upgrading…
        </Button>
        <Button variant="ghost" size="small" disabled>
          <Spinner size={11} /> Removing…
        </Button>
        <Button variant="primary" disabled>
          <Spinner /> Brief it
        </Button>
      </div>
      <div>
        <Spinner size={11} title="Writing the guide…" /> with a title, it's named for a screen reader
      </div>
    </div>
  ),
};
