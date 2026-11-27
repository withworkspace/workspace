import type { Meta, StoryObj } from "@storybook/react-vite";
import { LineChart } from "./LineChart";

// A chart of numbers over time: one series (coverage over a month), a few
// together, one point, and none yet.

const meta: Meta<typeof LineChart> = { title: "Components/LineChart", component: LineChart };
export default meta;

const days = (n: number, f: (i: number) => number): [string, number][] =>
  Array.from({ length: n }, (_, i) => [new Date(Date.UTC(2026, 9, 28 + i, 3)).toISOString(), f(i)]);

export const Coverage: StoryObj<typeof LineChart> = {
  args: { title: "api coverage, 30 days", unit: "%", series: [{ name: "api", points: days(30, (i) => 68 + i * 0.12 + Math.sin(i) * 0.6) }] },
};

export const ByRepo: StoryObj<typeof LineChart> = {
  args: {
    title: "Coverage by repo",
    unit: "%",
    series: [
      { name: "api", points: days(30, (i) => 71 + Math.sin(i / 3)) },
      { name: "web", points: days(30, (i) => 54 + i * 0.3) },
      { name: "worker", points: days(20, (i) => 82 - i * 0.2) },
    ],
  },
};

export const OnePoint: StoryObj<typeof LineChart> = { args: { title: "api coverage", unit: "%", series: [{ name: "api", points: days(1, () => 71.4) }] } };

export const NoneYet: StoryObj<typeof LineChart> = { args: { title: "api coverage", series: [{ name: "api", points: [] }] } };
