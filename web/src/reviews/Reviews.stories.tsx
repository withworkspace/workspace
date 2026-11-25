import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import type { PassMark } from "../api";
import { Chip } from "../ui/Chip";
import { Icon } from "../ui/Icon";
import type { StatusInfo } from "../ui/Status";
import { ReviewsAsOf } from "./ReviewsAsOf";
import { ReviewsFilter } from "./ReviewsFilter";
import { ReviewsLayers } from "./ReviewsLayers";
import { ReviewsRow } from "./ReviewsRow";
import { ReviewsSection } from "./ReviewsSection";
import { RowPasses } from "./RowPasses";

// The Reviews page from its parts: when GitHub was asked and the filter,
// a section of teammates' pull requests (one with passes, a stack with its
// layers open and one folded), a folded section, and yours below them.

const meta: Meta = { title: "Reviews/Reviews" };
export default meta;

const none = () => {};
const at = "2026-11-25T09:00:00Z";
const needs: StatusInfo = { icon: "circle-dot", tone: "attention", words: "Needs your review" };
const waiting: StatusInfo = { icon: "clock", tone: "waiting", words: "Waiting on them" };
const approved: StatusInfo = { icon: "check-circle", tone: "done", words: "Approved" };
const marks: PassMark[] = [
  { name: "bugs", title: "Bugs", standing: { status: "nogo", open: 2, blocking: 1 } },
  { name: "tests", title: "Tests", standing: { status: "go", open: 0, blocking: 0 }, moved: true },
];

function Page() {
  const [q, setQ] = useState("");
  const [picked, setPicked] = useState("a");
  const [folded, setFolded] = useState(false);
  const [open, setOpen] = useState(false);
  return (
    <div className="page">
      <div style={{ display: "flex", gap: 12, alignItems: "center", marginBottom: 16 }}>
        <ReviewsAsOf at={at} asking={false} />
        <ReviewsAsOf at={at} asking />
        <ReviewsFilter value={q} onChange={setQ} />
      </div>
      <ReviewsSection title="Needs your review" count={3} open>
        <ReviewsRow
          id="pr-row-a"
          selected={picked === "a"}
          status={needs}
          title="Add idempotency keys"
          chips={
            <>
              <Chip tone="quiet">
                <Icon name="flag" size={11} /> Make retries safe
              </Chip>
              <RowPasses marks={marks} />
            </>
          }
          sub="maya · api · #41 · needs your review"
          additions={68}
          deletions={0}
          files={2}
          checks="success"
          updated={at}
          to="/reviews/api/41"
          onHover={() => setPicked("a")}
          onOpen={none}
        />
        <ReviewsRow
          id="pr-row-b"
          selected={picked === "b"}
          status={needs}
          title="Rate-limit the public API"
          chips={
            <Chip tone="quiet">
              <Icon name="layers" size={11} /> stack of 3
            </Chip>
          }
          sub="mira · api · needs your review · 1 merged below"
          additions={212}
          deletions={38}
          checks="pending"
          updated={at}
          to="/reviews/api/44"
          onHover={() => setPicked("b")}
          onOpen={none}
          fold={{ folded, onFold: () => setFolded((f) => !f) }}
          layers={
            !folded && (
              <ReviewsLayers
                layers={[
                  { number: 44, status: approved, title: "Count requests a key, in Redis", words: "approved", additions: 80, deletions: 4, checks: "success", to: "/reviews/api/44" },
                  { number: 45, status: needs, title: "Answer 429 over the limit, with when to try again", words: "needs your review", passes: <RowPasses marks={marks.slice(0, 1)} />, additions: 120, deletions: 30, checks: "failure", to: "/reviews/api/45" },
                  { number: 46, status: waiting, title: "Say so in the docs", words: "waiting on them", additions: 12, deletions: 4, href: "https://github.com" },
                ]}
              />
            )
          }
        />
        <ReviewsRow id="pr-row-c" selected={picked === "c"} status={waiting} title="Fix the empty state" sub="jun · web · #25 · back with them" additions={9} deletions={0} files={1} checks="failure" to="/reviews/web/25" onHover={() => setPicked("c")} onOpen={none} />
      </ReviewsSection>
      <ReviewsSection title="Drafts" count={2} folds open={open} onFold={() => setOpen((o) => !o)}>
        <ReviewsRow id="pr-row-d" selected={false} status={{ icon: "circle-dashed", tone: "quiet", words: "Draft" }} title="Try a new chart" sub="tomas · web · #30 · draft" additions={300} deletions={20} files={12} onHover={none} onOpen={none} to="/reviews/web/30" />
      </ReviewsSection>
      <ReviewsSection title="Ready to merge" count={1} open yoursFirst>
        <ReviewsRow id="pr-row-e" selected={false} status={{ icon: "check-circle", tone: "attention", words: "Ready" }} title="Name the names a file declares" sub="You · api · #43 · ready to merge" additions={47} deletions={0} files={1} checks="success" updated={at} href="https://github.com" onHover={none} onOpen={none} />
      </ReviewsSection>
    </div>
  );
}

export const ThePage: StoryObj = { render: () => <Page /> };
