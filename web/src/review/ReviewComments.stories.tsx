import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import type { CommentEntry, CommentFilter } from "./comments";
import { ReviewCommentList } from "./ReviewCommentList";

// A review's comments as the Comments tab beside it lists them: each
// file's together, a draft of yours, open ones, one answered, one
// settled; which to show along the top. Press one to mark it gone to.

const meta: Meta = { title: "Review/Comments" };
export default meta;

const list: CommentEntry[] = [
  { key: "0:c:1", id: "c1", path: "internal/invoice/invoice.go", lines: "L4", removed: false, kind: "note", state: "draft", body: "sort only orders the PDF's rows. Say so where TaxByRate uses it, or it reads as if the total depends on it.", replies: 0 },
  { key: "0:c:2", id: "c2", path: "internal/invoice/invoice.go", lines: "L55", removed: false, kind: "question", state: "open", body: "Half away from zero, as before? Worth saying which in ADR 0001, now this is the one place tax is rounded.", replies: 0 },
  { key: "0:c:3", id: "c3", path: "internal/invoice/invoice.go", lines: "L42–43", removed: true, kind: "suggestion", state: "addressed", body: "\"Click here\" says nothing about what it does. Name the thing.", last: { by: "jun", body: "Done: it says what will be here now, and offers the first invoice." }, replies: 2 },
  { key: "1:c:4", id: "c4", path: "internal/invoice/invoice_test.go", lines: "L30", removed: false, kind: "nit", state: "resolved", body: "Is 99p × 3 at 20% the smallest case that shows the drift?", last: { by: "c1", body: "Yes: two lines round the same way and cancel out; three don't." }, replies: 1 },
  { key: "2:c:5", id: "c5", path: "", lines: "", removed: false, kind: "", state: "draft", body: "On the whole: good. One question on rounding.", replies: 0 },
];

function List({ start = "all" as CommentFilter, of = list }) {
  const [filter, setFilter] = useState<CommentFilter>(start);
  const [at, setAt] = useState<string>("0:c:2");
  return (
    <div style={{ width: 360, border: "1px solid var(--line)", borderRadius: 12, background: "var(--backdrop)" }}>
      <ReviewCommentList list={of} filter={filter} onFilter={setFilter} at={at} onPick={setAt} />
    </div>
  );
}

export const Every: StoryObj = { render: () => <List /> };
export const Drafts: StoryObj = { render: () => <List start="draft" /> };
export const None: StoryObj = { render: () => <List of={[]} /> };
