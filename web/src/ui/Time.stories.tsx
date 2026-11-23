import type { Meta, StoryObj } from "@storybook/react-vite";
import { Time } from "./Time";

// A time, each way it's said, for times from a minute ago to last year:
// as it reads best from now, how long ago, a list's compact age, and the
// clock. Each has the exact time on hover.

const meta: Meta = { title: "Components/Time" };
export default meta;

const ago = (ms: number) => new Date(Date.now() - ms).toISOString();
const MIN = 60_000;
const HOUR = 60 * MIN;
const DAY = 24 * HOUR;

const times: [string, string][] = [
  ["a moment ago", ago(20_000)],
  ["5 minutes ago", ago(5 * MIN)],
  ["3 hours ago", ago(3 * HOUR)],
  ["yesterday", ago(DAY)],
  ["4 days ago", ago(4 * DAY)],
  ["3 weeks ago", ago(21 * DAY)],
  ["over a year ago", ago(400 * DAY)],
];

const forms = ["stamp", "relative", "age", "clock"] as const;

export const Forms: StoryObj = {
  render: () => (
    <div className="page">
      <table style={{ borderSpacing: "24px 6px", textAlign: "left" }}>
        <thead>
          <tr className="dim">
            <th />
            {forms.map((f) => (
              <th key={f} style={{ fontWeight: 500 }}>
                {f}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {times.map(([name, iso]) => (
            <tr key={name}>
              <td className="dim">{name}</td>
              {forms.map((f) => (
                <td key={f}>
                  <Time iso={iso} as={f} />
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
      <p>
        In a line of words, <Time iso={ago(12 * MIN)} as="relative" className="dim" />, in the class the page gives it.
      </p>
    </div>
  ),
};
