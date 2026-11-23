import type { ReactNode } from "react";
import { FileIcon } from "../../ui/FileIcon";
import { Icon } from "../../ui/Icon";
import { Status } from "../../ui/status";
import type { Tone } from "../../ui/tone";
import type { CodeHit, PullFound } from "../find";

// What the stories of Find's parts draw: rows as a workspace might have
// them, made up, and a line or two of a file. Nothing here is read by
// the app.

// A row as FindRow takes it, with the letters "inv" matched in its title.
export type SampleRow = { id: string; mark: ReactNode; title: string; at?: number[]; path?: boolean; hint?: string; notes?: string[]; tone?: Tone; kind: string; pull?: PullFound };

const hit = (title: string, word: string): number[] => {
  const from = title.toLowerCase().indexOf(word);
  return from < 0 ? [] : [...word].map((_, i) => from + i);
};

const review = { tone: "attention", icon: "eye", words: "Review" } as const;
const running = { tone: "progress", icon: "spinner", words: "Running", live: true } as const;
const parked = { tone: "quiet", icon: "pause", words: "Parked" } as const;

export const missions: SampleRow[] = [
  { id: "m1", mark: <Status s={review} iconOnly />, title: "Export invoices as CSV", hint: "export-invoices-as-csv", kind: "Mission" },
  { id: "m2", mark: <Status s={running} iconOnly />, title: "Paginate the invoices endpoint", hint: "paginate-the-invoices-endpoint", kind: "Mission" },
  { id: "m3", mark: <Status s={parked} iconOnly />, title: "Invoices in more than one currency", hint: "invoices-in-more-than-one-currency", kind: "Mission" },
].map((r) => ({ ...r, at: hit(r.title, "inv") }));

export const pulls: SampleRow[] = [
  {
    id: "p1",
    mark: <Icon name="pr" />,
    title: "Paginate the invoices endpoint",
    hint: "api#45 · you",
    notes: ["its checks are running"],
    kind: "Pull request",
    pull: { repo: "api", number: 45, title: "Paginate the invoices endpoint", author: "you", branch: "paginate", words: "Open, waiting on its checks", checks: "pending" },
  },
  {
    id: "p2",
    mark: <Icon name="pr" />,
    title: "Idempotency keys for POST /invoices",
    hint: "api#41 · maya",
    notes: ["needs your review"],
    tone: "attention" as const,
    kind: "Pull request",
    pull: { repo: "api", number: 41, title: "Idempotency keys for POST /invoices", author: "maya", branch: "idempotency-keys", words: "Asks for your review", checks: "success", stack: "2 of 3" },
  },
  {
    id: "p3",
    mark: <Icon name="pr" />,
    title: "Keyboard shortcuts for the invoice list",
    hint: "web#27 · ines",
    notes: ["needs a review from your team", "checks fail"],
    tone: "attention" as const,
    kind: "Pull request",
    pull: { repo: "web", number: 27, title: "Keyboard shortcuts for the invoice list", author: "ines", branch: "list-keys", words: "Asks your team for a review", checks: "failure", draft: true },
  },
].map((r) => ({ ...r, at: hit(r.title, "inv") }));

export const files: SampleRow[] = [
  { id: "f1", title: "api/internal/invoice/invoice.go", notes: ["changed in paginate"] },
  { id: "f2", title: "web/src/invoices/InvoiceList.tsx" },
  { id: "f3", title: "web/src/a/folder/that/goes/on/for/a/long/way/before/it/gets/to/invoices/money.ts:12" },
].map((r) => ({ ...r, kind: "File", mark: <FileIcon path={r.title.replace(/:\d+$/, "")} />, path: true, at: hit(r.title, "inv") }));

export const places: SampleRow[] = [
  { id: "d1", mark: <Icon name="play" />, title: "Resume", hint: "Invoices in more than one currency is parked", kind: "Do" },
  { id: "d2", mark: <Icon name="chevron-right" />, title: "Reviews", hint: "Pull requests that ask for you", kind: "Place" },
];

const line = (path: string, n: number, text: string, word: string, worktree = ".ground"): CodeHit => {
  const ranges: [number, number][] = [];
  for (let from = text.indexOf(word); from >= 0; from = text.indexOf(word, from + word.length)) ranges.push([from, from + word.length]);
  return { repo: "api", worktree, path, line: n, column: (ranges[0]?.[0] ?? 0) + 1, text, ranges };
};

export const hits: CodeHit[] = [
  line("internal/httpapi/page.go", 7, '\t"example.com/api/internal/invoice"', "invoice", "paginate"),
  line("internal/httpapi/page.go", 10, "// pageSize is how many invoices GET /invoices gives unless ?limit= asks", "invoice", "paginate"),
  line("internal/invoice/invoice.go", 41, "func (s *Store) Invoices(ctx context.Context) ([]invoice.Invoice, error) {", "nvoice"),
  line("a/folder/that/goes/on/for/a/long/way/before/it/gets/to/the/file/total.go", 128, "\t\tsum = sum.Add(l.Net) // each line of the invoice, in minor units, summed before VAT is worked out on the total", "invoice"),
  line("justfile", 3, "seed: # some invoices to look at", "invoice"),
];
