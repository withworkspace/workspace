import type { ArtifactContent, Checking, Conclusion, DocDraft, EndedCapsule } from "../api";
import taken from "./fixtures/taken.json";

// What the stories of a mission's endings, its checks running and a
// document being edited are drawn from, where no seed leaves a mission:
// Z checked part by part (met, not met, nobody checked), capsules that
// ended alone, ws checking a step, and a document with an edit kept.

const at = new Date(taken.at).getTime();
const ago = (s: number) => new Date(at - s * 1000).toISOString();
const ahead = (s: number) => new Date(at + s * 1000).toISOString();

export const zChecked: Conclusion = {
  ending: "answered",
  text: "The slow page was the invoices list: it sorted in memory.",
  z: [
    { z: "We know why the invoices list is slow", met: true, how: "a profile of `GET /invoices` shows the sort, 1.8s of 2.1s" },
    { z: "There's a mission for the fix", met: false, how: "nobody raised one yet" },
    { z: "The finding is written up in `records`", met: false },
  ],
  to: ["paginate-the-invoices-endpoint", "show-vat-by-rate-on-the-pdf"],
};

export const endedCapsules: EndedCapsule[] = [
  { repo: "api", name: "spike-sort", ending: "dropped", why: "the spike answered it", at: ago(3600) },
  { repo: "web", name: "old-list", ending: "dropped", at: ago(7200) },
  { repo: "api", name: "paginate", ending: "handed-on", to: "paginate-the-invoices-endpoint", at: ago(600) },
];

export const checkingCommand: Checking = { step: 2, n: 1, of: 3, command: "go test ./internal/invoices/... -run TestList -count=1", line: "ok  \tsorrel/internal/invoices\t0.412s", judged: [3], since: ago(80), until: ahead(820) };
export const checkingQuiet: Checking = { step: 2, n: 2, of: 3, command: "just lint", since: ago(5), until: ahead(3595) };
export const checkingJudge: Checking = { step: 4, of: 5, judged: [4, 5], judge: "the judge (claude:sonnet)", since: ago(42), until: ahead(258) };

export const editedDoc = {
  ref: "records:proposals/overdue-reminders.md",
  kind: "doc",
  label: "Overdue reminders",
  state: "draft",
  text: "# Overdue reminders\n\nSend a reminder when an invoice is 7, 14 and 30 days overdue.\n\n## Why\n\nMost late payers pay on the first nudge.\n",
  markdown: true,
  version: "v2",
  annotatable: false,
  editable: true,
  saves: { how: "land", branch: "main", verify: "just check", subject: "Edit proposals/overdue-reminders.md" },
} as unknown as ArtifactContent;

export const editedDraft: DocDraft = { ref: editedDoc.ref, base: "v1", text: editedDoc.text + "\nA reminder says how to pay.\n", at: ago(300) } as DocDraft;

// A picture and a video, as crew hand them over to show their work.
export const picture = "data:image/svg+xml," + encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" width="640" height="360"><rect width="640" height="360" fill="#e8e8ec"/><rect x="40" y="40" width="560" height="60" rx="8" fill="#fff"/></svg>');
