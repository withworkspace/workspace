import type { ReactNode } from "react";
import { Row, RowLine, RowMain, RowMeta, RowSub, Rows, RowTitle } from "../ui/Rows";
import { Status } from "../ui/Status";
import type { BelowView } from "./Below";
import type { BelowWhere } from "./chromeState";
import { Text } from "../ui/Text";

// Things to show below, for the docks' stories: Sorrel's runs, written
// by hand, since the panes that will draw real ones aren't parts yet.
// What a view shows here is words standing in for it.

type SampleRun = { name: string; says: string; in: string; when: string; here?: boolean; tone: "done" | "danger" | "progress" };

const results: SampleRun[] = [
  { name: "check-3-1", says: "passed in 0.1s", in: "api / tax-rounding", when: "2m ago", here: true, tone: "done" },
  { name: "check-1-1", says: "failed (exit 1) in 0.7s", in: "api / tax-rounding", when: "2m ago", here: true, tone: "danger" },
  { name: "test", says: "passed in 41s", in: "web / dark-mode", when: "9m ago", tone: "done" },
  { name: "lint", says: "passed in 6s", in: "web / dark-mode", when: "9m ago", tone: "done" },
  { name: "test", says: "passed in 12s", in: "api / reminders", when: "31m ago", tone: "done" },
];

const running: SampleRun[] = [{ name: "dev", says: "up for 14m, at dev.sorrel.localhost", in: "web / dark-mode", when: "14m", tone: "progress" }];

const icon = { done: "check-circle", danger: "x-circle", progress: "circle-dot" } as const;

// A view's rows: here's (the page's capsule), or everywhere's.
function runs(all: SampleRun[], where: BelowWhere): ReactNode {
  const shown = where === "here" ? all.filter((r) => r.here) : all;
  if (shown.length === 0) return <Text as="p" dim style={{ margin: 0, padding: "14px 16px" }}>Nothing here. There's more everywhere.</Text>;
  return (
    <Rows>
      {shown.map((r, i) => (
        <Row key={i} still>
          <Status s={{ icon: icon[r.tone], tone: r.tone, words: r.says }} iconOnly size={14} />
          <RowMain>
            <RowLine>
              <RowTitle>{r.name}</RowTitle>
              <Text dim>{r.says}</Text>
            </RowLine>
            <RowSub>In {r.in}</RowSub>
          </RowMain>
          <RowMeta>{r.when}</RowMeta>
        </Row>
      ))}
    </Rows>
  );
}

const count = (all: SampleRun[], where: BelowWhere) => (where === "here" ? all.filter((r) => r.here) : all).length;

// What's below, as the app will have it once runs are views: what's
// running, what it came to, and a run's terminal, which you opened and
// can close.
export function sampleViews(where: BelowWhere, onCloseTerminal?: () => void): BelowView[] {
  return [
    { id: "running", name: "Running", icon: "play", count: count(running, where), pane: (w) => runs(running, w) },
    { id: "results", name: "Results", icon: "list-checks", count: count(results, where), pane: (w) => runs(results, w) },
    ...(onCloseTerminal
      ? [
          {
            id: "run:dev",
            name: "dev · web/dark-mode",
            icon: "terminal" as const,
            onClose: onCloseTerminal,
            pane: () => (
              <pre style={{ margin: 0, padding: "12px 16px", font: "12px/1.6 var(--mono)", whiteSpace: "pre-wrap" }}>
                {"$ pnpm dev\n\n  VITE v7.1.4  ready in 412 ms\n\n  ➜  Local:   http://dev.sorrel.localhost/\n  ➜  press h + enter to show help\n\n9:41:07 AM [vite] hmr update /src/invoices/InvoiceList.tsx\n9:41:52 AM [vite] hmr update /src/theme/tokens.css"}
              </pre>
            ),
          },
        ]
      : []),
  ];
}
