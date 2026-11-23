import type { Meta, StoryObj } from "@storybook/react-vite";
import { Button } from "./Button";
import { ToastPill } from "./ToastPill";
import { Toasts } from "./Toasts";
import { toast } from "./toast";

// Toasts: one in each tone, one with something to press, a long one;
// then said for real, at the foot of the window, three at most.

const meta: Meta = { title: "Components/Toasts" };
export default meta;

export const Every: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 10, justifyItems: "start" }}>
      <ToastPill>Copied the link</ToastPill>
      <ToastPill tone="done">Approved b3</ToastPill>
      <ToastPill tone="danger">Couldn't copy: the browser said no</ToastPill>
      <ToastPill tone="done" action={{ label: "Undo", run: () => {} }}>
        Parked the mission
      </ToastPill>
      <ToastPill action={{ label: "Open", run: () => {} }}>A long one: the pull request for archive-invoices-older-than-seven-years was opened against main, and its checks are running</ToastPill>
    </div>
  ),
};

let n = 0;

export const Said: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "flex", gap: 8 }}>
      <Button onClick={() => toast(`Said ${++n}`)}>Say something</Button>
      <Button onClick={() => toast("Done", { tone: "done", action: { label: "Undo", run: () => toast("Undone") } })}>With Undo</Button>
      <Button onClick={() => toast("It failed", { tone: "danger" })}>Something failed</Button>
      <Toasts />
    </div>
  ),
};
