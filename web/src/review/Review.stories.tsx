import type { Meta, StoryObj } from "@storybook/react-vite";
import { useMemo, useState } from "react";
import type { PassFinding, Review, ReviewNote } from "../api";
import { diffOf, goBefore, goNow } from "../stories/code";
import { Button } from "../ui/Button";
import type { DiffCell } from "../ui/diffLayout";
import { Icon } from "../ui/Icon";
import { Keys } from "../ui/Keys";
import { escapeHTML, highlightLines, languageOf, useHighlighter } from "./highlight";
import { fileRows } from "./model";
import { ReviewComment } from "./ReviewComment";
import { ReviewCommon } from "./ReviewCommon";
import { ReviewCommits } from "./ReviewCommits";
import { ReviewEarlier } from "./ReviewEarlier";
import { ReviewFile } from "./ReviewFile";
import { ReviewFileJump } from "./ReviewFileJump";
import { ReviewFinding } from "./ReviewFinding";
import { ReviewFindingActions } from "./ReviewFindingActions";
import { ReviewGuideGroup } from "./ReviewGuideGroup";
import { ReviewHead } from "./ReviewHead";
import { ReviewLayout } from "./ReviewLayout";
import { ReviewLines } from "./ReviewLines";
import { ReviewNoteEditor } from "./ReviewNoteEditor";
import { ReviewOrphanPath } from "./ReviewOrphanPath";
import { ReviewOrphans } from "./ReviewOrphans";
import { ReviewProgress } from "./ReviewProgress";
import { ReviewScope } from "./ReviewScope";
import { ReviewSelectBar } from "./ReviewSelectBar";
import { ReviewStatus } from "./ReviewStatus";
import { ReviewSubmit } from "./ReviewSubmit";
import { ReviewTabs } from "./ReviewTabs";
import { ReviewVerdicts } from "./ReviewVerdicts";
import { ReviewViewBody } from "./ReviewViewBody";
import { ReviewViewRow } from "./ReviewViewRow";
import { ReviewViews } from "./ReviewViews";
import { Text } from "../ui/Text";
import { proseClass } from "../ui/Prose";

// A review's page from its parts: the list down the side (its views, the
// Changes view and how much of it's viewed), the head and what the diff is
// of, a file with its lines and one folded each way, a group of the
// guide's, comments in each state, a pass's finding, the editor, lines
// picked, the dialog that submits it and the file finder.

const meta: Meta = { title: "Review/Review" };
export default meta;

const none = () => {};
const path = "internal/nav/nav.go";
const changed = diffOf(goBefore, goNow, path);

function useHTML() {
  const ready = useHighlighter();
  return useMemo(() => {
    const lang = ready ? languageOf(path) : undefined;
    const old = lang ? highlightLines(goBefore, lang) : goBefore.map(escapeHTML);
    const now = lang ? highlightLines(goNow, lang) : goNow.map(escapeHTML);
    return (c: DiffCell, side: "old" | "new") => (side === "old" ? old : now)[c.num - 1] ?? escapeHTML(c.text);
  }, [ready]);
}

const note = (o: Partial<ReviewNote>): ReviewNote => ({
  id: "c1",
  path,
  side: "new",
  line: 12,
  endLine: 12,
  commit: "88cf294",
  excerpt: ["func Names(src string) []Name {"],
  kind: "suggestion",
  body: "Name it for what it finds: these are the names declared, not every name.",
  status: "open",
  thread: [],
  ...o,
});

const finding: PassFinding = {
  key: "f1",
  kind: "bug",
  text: "The loop stops at the first `}` it sees, so a struct with a method set after it loses the rest.",
  path,
  side: "new",
  line: 20,
  endLine: 24,
  state: "open",
  blocks: true,
} as PassFinding;

const earlier: Review[] = [
  {
    id: "r1",
    verdict: "changes",
    submitted: "2026-11-24T10:00:00Z",
    summary: "Close, but the names need another look.",
    comments: [note({ id: "c0", kind: "blocking", body: "This drops methods." })],
  } as Review,
];

function Side() {
  return (
    <>
      <ReviewViews>
        <ReviewViewRow icon="info" label="Overview" n={1} here={false} onPick={none} />
        <ReviewViewRow icon="book" label="Guide" n={2} here={false} stands={<Icon name="check" size={11} />} onPick={none} />
        <ReviewViewRow icon="message" label="Discussion" n={3} here={false} count={4} news="2 new since you looked" onPick={none} />
        <ReviewViewRow icon="list-checks" label="Passes" n={4} here={false} onPick={none} />
      </ReviewViews>
      <ReviewViews>
        <ReviewViewRow
          icon="code"
          label="Changes"
          n={5}
          here
          count="1/3"
          onPick={none}
          tools={
            <Button variant="ghost" size="small" square title="Jump to a file" aria-label="Jump to a file">
              <Icon name="search" size={13} />
            </Button>
          }
        />
      </ReviewViews>
      <ReviewProgress viewed={1} of={3} />
      <ReviewCommon path="internal/nav/" />
    </>
  );
}

function Page() {
  const html = useHTML();
  const rows = useMemo(() => fileRows(0, changed, { viewed: false, expanded: {} }, false).slice(1), []);
  const [view, setView] = useState("changes");
  return (
    <div style={{ height: 900 }}>
      <ReviewLayout side={<Side />} onResize={none} onResetWidth={none}>
        <ReviewHead
          back={{ to: "/missions/names", label: "Find the names a file declares" }}
          name={<Text mono>api/names</Text>}
          scope={<ReviewScope since={{ name: "r1", title: "What's changed since r1" }} commits={3} additions={42} deletions={7} />}
          actions={
            <Button variant="primary">
              Submit review · 2 <Keys k="s" />
            </Button>
          }
        />
        <ReviewTabs
          views={[
            { key: "overview", label: "Overview", icon: "info", n: 1 },
            { key: "discussion", label: "Discussion", icon: "message", n: 2, news: "2 new" },
            { key: "changes", label: "Changes", icon: "code", n: 3 },
          ]}
          here={view}
          onPick={setView}
        />
        <ReviewStatus>
          Re-reviewing what's changed since r1 (changes requested). Its comments sit on the lines they were made on; 1 still to resolve: <Keys k="⏎" /> replies, <Keys k="x" /> resolves.
        </ReviewStatus>
        <ReviewViewBody label="Changes" hidden={false}>
          <ReviewCommits commits={[{ sha: "88cf2940000", subject: "nav: names a file declares", author: "Maya" }, { sha: "0a963600000", subject: "nav: read a file once", author: "Maya" }]} />
          <ReviewEarlier reviews={earlier} />
          <ReviewGuideGroup label="What it finds" files={2} why="Start here: `Names` is what the rest is built on." />
          <ReviewFile index={0} status="modified" path={path} additions={42} deletions={7} viewed={false} folded={false} cursor guide="Finds the names a file declares, for the explorer's outline." onHead={none} onFold={none} onViewed={none}>
            <ReviewLines label={`Changes to ${path}`} rows={rows} split={false} html={html} onRow={none} onGap={none} onNote={none} draw={() => null} />
          </ReviewFile>
          <ReviewFile index={1} status="added" path="internal/nav/names_test.go" additions={30} deletions={0} viewed folded={false} cursor={false} onHead={none} onFold={none} onViewed={none} />
          <ReviewGuideGroup label="Lockfiles" files={1} skim />
          <ReviewFile index={2} status="renamed" oldPath="go.sum.old" path="go.sum" additions={4} deletions={4} viewed={false} folded cursor={false} skim="Generated" onHead={none} onFold={none} onViewed={none} />
          <ReviewFile index={3} status="deleted" path="internal/nav/old.go" additions={0} deletions={12} viewed={false} folded cursor={false} unchanged onHead={none} onFold={none} onViewed={none} />
          <ReviewOrphans label="Comments on files not changed since r1">
            <div>
              <ReviewOrphanPath path="internal/nav/walk.go" />
              <ReviewComment id="row-o1" note={note({ id: "c2", path: "internal/nav/walk.go", kind: "question", body: "Does `walk` still need the depth?", excerpt: ["func walk(n Node, depth int) {"] })} draft={false} apart cursor={false} replying={false} onClick={none} onToggle={none} onReply={none} onReplyCancel={none} />
            </div>
            <div>
              <ReviewOrphanPath />
              <ReviewComment id="row-o2" note={note({ id: "c3", path: "", kind: "", body: "On the whole: good." })} draft apart cursor={false} replying={false} onClick={none} onToggle={none} onReply={none} onReplyCancel={none} />
            </div>
          </ReviewOrphans>
        </ReviewViewBody>
      </ReviewLayout>
    </div>
  );
}

export const OnePage: StoryObj = { render: () => <Page /> };

function Read() {
  return (
    <div style={{ height: 500 }}>
      <ReviewLayout side={<Side />} onResize={none} onResetWidth={none} foot={<div style={{ padding: 12, pointerEvents: "auto" }}>A foot over the pane</div>}>
        <ReviewHead back={{ to: "/reviews", label: "Reviews" }} name="Add idempotency keys" actions={<Button variant="primary">Submit review</Button>} />
        <ReviewViewBody label="Overview" hidden={false} read>
          <h2>What it's for</h2>
          <p className={proseClass()}>Requests that repeat with the same key get the first answer again.</p>
          <h2>Its mission</h2>
          <p>Make retries safe.</p>
        </ReviewViewBody>
      </ReviewLayout>
    </div>
  );
}

export const AViewRead: StoryObj = { render: () => <Read /> };

function Cards() {
  const [dismissing, setDismissing] = useState(false);
  return (
    <div style={{ display: "grid", gap: 8, maxWidth: 760, padding: 24, font: "var(--text-sm) / 20px var(--mono)" }}>
      <ReviewComment id="a" note={note({})} draft cursor replying={false} onClick={none} onToggle={none} onReply={none} onReplyCancel={none} />
      <ReviewComment
        id="b"
        note={note({ id: "c4", kind: "blocking", body: "This drops a method set after the struct.", suggestion: "func Names(src string) ([]Name, error) {", thread: [{ by: "crew", at: "2026-11-25T09:00:00Z", body: "Fixed in 88cf294.", commit: "88cf2940" }] })}
        draft={false}
        cursor
        replying={false}
        followup={{ kind: "addressed", text: "The loop now reads every `}`." }}
        onClick={none}
        onToggle={none}
        onReply={none}
        onReplyCancel={none}
      />
      <ReviewComment id="c" note={note({ id: "c5", kind: "nit", by: "tomas", url: "https://github.com", status: "open" })} draft={false} elsewhere cursor={false} replying followup={{ kind: "not-addressed", text: "Still reads one at a time.", blocks: true }} onClick={none} onToggle={none} onReply={none} onReplyCancel={none} />
      <ReviewComment id="d" note={note({ id: "c6", status: "resolved", by: "agent" })} draft={false} cursor={false} replying={false} onClick={none} onToggle={none} onReply={none} onReplyCancel={none} />
      <ReviewFinding
        id="e"
        title="Bugs"
        finding={finding}
        cursor
        onClick={none}
        actions={<ReviewFindingActions dismissing={dismissing} cursor canAsk onTake={none} onDismissing={setDismissing} onDismiss={() => setDismissing(false)} onAsk={none} />}
      />
      <ReviewNoteEditor target={{ side: "old", line: 4, endLine: 9 }} lines={["func walk(n Node) {"]} onSave={none} onCancel={none} />
      <div style={{ position: "relative", height: 60 }}>
        <div style={{ position: "absolute", top: 0, left: 0, right: 0, height: 0 }}>
          <ReviewSelectBar label="Lines 12–14" onComment={none} onAsk={none} />
        </div>
      </div>
      <ReviewVerdicts label="Verdict" value="approve" keyed onPick={none} after={<Text dim>Left alone</Text>} />
      <ReviewVerdicts label="Verdict" value="comment" keyed={false} unavailable={(v) => (v === "comment" ? undefined : "It's your own")} onPick={none} />
    </div>
  );
}

export const CommentsAndFindings: StoryObj = { render: () => <Cards /> };

export const Submitting: StoryObj = { render: () => <ReviewSubmit draft={{ id: "draft", comments: [note({}), note({ id: "c7", kind: "nit" })] } as Review} onClose={none} onDiscard={none} onSubmit={async () => {}} /> };

export const JumpingToAFile: StoryObj = { render: () => <ReviewFileJump paths={[path, "internal/nav/names_test.go", "go.sum", "web/src/review/Review.tsx"]} onPick={none} onClose={none} /> };
